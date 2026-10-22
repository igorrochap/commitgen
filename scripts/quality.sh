#!/usr/bin/env bash

# Keep running after a failed gate so the summary can report every gate.
set -u

readonly COVERAGE_FLOOR=70
readonly STYLE_LINTERS=errcheck,staticcheck,revive,gocritic,errorlint,unused,ineffassign
readonly COMPLEXITY_LINTERS=gocyclo,gocognit

gates=(format vet style complexity coverage tests architecture)
selected_gates=()
results=()
notes=()
reports=()
failed_gates=()

require_command() {
  local tool=$1
  local install_command=$2

  if command -v "$tool" >/dev/null 2>&1; then
    return 0
  fi

  printf '%s is required; install with: %s\n' "$tool" "$install_command"
  return 1
}

first_go_diagnostic() {
  local output=$1
  local line

  while IFS= read -r line; do
    if [[ "$line" == *.go:* ]]; then
      printf '%s\n' "$line"
      return 0
    fi
  done <<<"$output"

  return 1
}

first_format_file() {
  local output=$1
  local line

  while IFS= read -r line; do
    case "$line" in
      'diff --git a/'*)
        line=${line#diff --git a/}
        printf '%s\n' "${line%% b/*}"
        return 0
        ;;
      '--- '*)
        line=${line#--- }
        line=${line# a/}
        line=${line#./}
        line=${line%.orig}
        if [[ "$line" != "/dev/null" ]]; then
          printf '%s\n' "$line"
          return 0
        fi
        ;;
    esac
  done <<<"$output"

  return 1
}

first_test_failure() {
  local output=$1
  local line

  while IFS= read -r line; do
    case "$line" in
      '--- FAIL: '*)
        line=${line#--- FAIL: }
        printf '%s\n' "${line%% (*}"
        return 0
        ;;
    esac
  done <<<"$output"

  while IFS= read -r line; do
    case "$line" in
      FAIL$'\t'*|FAIL\ *)
        line=${line#FAIL}
        line=${line#"${line%%[![:space:]]*}"}
        if [[ -n "$line" ]]; then
          printf '%s\n' "${line%%[[:space:]]*}"
          return 0
        fi
        ;;
    esac
  done <<<"$output"

  return 1
}

format() {
  local output
  local exit_code
  local file

  if ! require_command golangci-lint 'go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.13.1'; then
    return 1
  fi

  if output=$(golangci-lint --color=never fmt --diff 2>&1); then
    exit_code=0
  else
    exit_code=$?
  fi

  if (( exit_code != 0 )); then
    if file=$(first_format_file "$output"); then
      printf 'formatting required: %s\n' "$file"
    elif [[ -n "$output" ]]; then
      printf 'golangci-lint fmt failed: %s\n' "$(first_line "$output")"
    else
      printf '%s\n' 'golangci-lint fmt failed'
    fi
    return 1
  fi

  if [[ -n "$output" ]]; then
    if file=$(first_format_file "$output"); then
      printf 'formatting required: %s\n' "$file"
    else
      printf 'formatting required: %s\n' "$(first_line "$output")"
    fi
    return 1
  fi

  printf '%s\n' 'PASS  gofmt, goimports'
}

vet() {
  local output
  local diagnostic

  if ! require_command go 'install Go 1.25.2+ from https://go.dev/dl/'; then
    return 1
  fi

  if output=$(go vet ./... 2>&1); then
    printf '%s\n' 'PASS  go vet ./...'
    return 0
  fi

  if diagnostic=$(first_go_diagnostic "$output"); then
    printf '%s\n' "$diagnostic"
  elif [[ -n "$output" ]]; then
    printf '%s\n' "$(first_line "$output")"
  else
    printf '%s\n' 'go vet ./... failed'
  fi
  return 1
}

style() {
  diff_gate "$STYLE_LINTERS"
}

complexity() {
  diff_gate "$COMPLEXITY_LINTERS"
}

select_quality_base() {
  local requested_ref
  local candidate
  local commit

  if [[ -n "${QUALITY_BASE_REF:-}" ]]; then
    requested_ref=$QUALITY_BASE_REF
    if ! commit=$(git rev-parse --verify --quiet --end-of-options "${requested_ref}^{commit}" 2>/dev/null); then
      printf "base ref '%s' does not exist\n" "$requested_ref"
      return 1
    fi
    QUALITY_BASE_NAME=$requested_ref
    QUALITY_BASE_COMMIT=$commit
    return 0
  fi

  if [[ -n "${GITHUB_BASE_REF:-}" ]]; then
    requested_ref=$GITHUB_BASE_REF
    for candidate in "origin/$requested_ref" "$requested_ref"; do
      if commit=$(git rev-parse --verify --quiet --end-of-options "${candidate}^{commit}" 2>/dev/null); then
        QUALITY_BASE_NAME=$candidate
        QUALITY_BASE_COMMIT=$commit
        return 0
      fi
    done
    printf "base ref '%s' does not exist (tried 'origin/%s' and '%s')\n" \
      "$requested_ref" "$requested_ref" "$requested_ref"
    return 1
  fi

  for candidate in origin/main main; do
    if commit=$(git rev-parse --verify --quiet --end-of-options "${candidate}^{commit}" 2>/dev/null); then
      QUALITY_BASE_NAME=$candidate
      QUALITY_BASE_COMMIT=$commit
      return 0
    fi
  done

  printf '%s\n' "base refs 'origin/main' and 'main' do not exist"
  return 1
}

append_unique_file() {
  local candidate=$1
  local existing

  for existing in "${changed_files[@]-}"; do
    if [[ "$candidate" == "$existing" ]]; then
      return 0
    fi
  done
  changed_files+=("$candidate")
}

diff_gate() {
  local linters=$1
  local repository_root
  local merge_base
  local temporary_directory
  local tracked_paths
  local untracked_paths
  local file
  local directory
  local exit_code
  local output
  local existing_directory
  local found_directory
  local -a changed_files=()
  local -a package_directories=()

  if ! repository_root=$(git rev-parse --show-toplevel 2>/dev/null); then
    printf '%s\n' 'diff gate must run inside a git repository'
    return 1
  fi
  if ! cd "$repository_root"; then
    printf 'could not change to repository root: %s\n' "$repository_root"
    return 1
  fi

  if ! select_quality_base; then
    return 1
  fi

  if ! merge_base=$(git merge-base HEAD "$QUALITY_BASE_COMMIT" 2>/dev/null); then
    printf "could not find a merge base for HEAD and '%s'; fetch the full history and retry\n" \
      "$QUALITY_BASE_NAME"
    return 1
  fi

  if ! temporary_directory=$(mktemp -d "${TMPDIR:-/tmp}/quality.XXXXXX"); then
    printf '%s\n' 'could not create temporary files for the diff gate'
    return 1
  fi
  tracked_paths=$temporary_directory/tracked-paths
  untracked_paths=$temporary_directory/untracked-paths
  if ! git diff --name-only --diff-filter=ACMR --find-renames -z "$merge_base" -- '*.go' >"$tracked_paths"; then
    printf '%s\n' 'could not list changed Go files'
    rm -rf "$temporary_directory"
    return 1
  fi
  if ! git ls-files --others --exclude-standard -z -- '*.go' >"$untracked_paths"; then
    printf '%s\n' 'could not list untracked Go files'
    rm -rf "$temporary_directory"
    return 1
  fi

  while IFS= read -r -d '' file; do
    if [[ -f "$file" ]]; then
      append_unique_file "$file"
    fi
  done <"$tracked_paths"
  while IFS= read -r -d '' file; do
    if [[ -f "$file" ]]; then
      append_unique_file "$file"
    fi
  done <"$untracked_paths"

  local changed_file_count=${#changed_files[@]}
  if (( changed_file_count == 0 )); then
    printf 'PASS  base %s, 0 changed Go files\n' "$QUALITY_BASE_NAME"
    rm -rf "$temporary_directory"
    return 0
  fi

  for file in "${changed_files[@]}"; do
    directory=${file%/*}
    if [[ "$directory" == "$file" ]]; then
      directory=.
    fi
    found_directory=false
    for existing_directory in "${package_directories[@]-}"; do
      if [[ "$directory" == "$existing_directory" ]]; then
        found_directory=true
        break
      fi
    done
    if [[ "$found_directory" == false ]]; then
      package_directories+=("./$directory")
    fi
  done

  if ! require_command golangci-lint 'go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.13.1'; then
    printf 'FAIL  base %s, %d changed Go files\n' "$QUALITY_BASE_NAME" "$changed_file_count"
    rm -rf "$temporary_directory"
    return 1
  fi

  local patch_file=$temporary_directory/changes.patch
  if ! git diff --no-ext-diff --binary --find-renames --diff-filter=ACMR "$merge_base" -- '*.go' >"$patch_file"; then
    printf '%s\n' 'could not build the Go change patch'
    rm -rf "$temporary_directory"
    return 1
  fi
  for file in "${changed_files[@]}"; do
    if git ls-files --error-unmatch -- "$file" >/dev/null 2>&1; then
      continue
    fi

    git diff --no-index --binary -- /dev/null "$file" >>"$patch_file"
    exit_code=$?
    if (( exit_code > 1 )); then
      printf 'could not add untracked file to the change patch: %s\n' "$file"
      rm -rf "$temporary_directory"
      return 1
    fi
  done

  output=$(golangci-lint run \
    --color=never \
    --enable-only "$linters" \
    --new-from-patch "$patch_file" \
    --whole-files \
    --tests=true \
    --show-stats=false \
    --max-issues-per-linter=0 \
    --max-same-issues=0 \
    "${package_directories[@]}" 2>&1)
  exit_code=$?
  if (( exit_code == 0 )); then
    printf 'PASS  base %s, %d changed Go files\n' "$QUALITY_BASE_NAME" "$changed_file_count"
    rm -rf "$temporary_directory"
    return 0
  fi

  printf 'FAIL  base %s, %d changed Go files\n' "$QUALITY_BASE_NAME" "$changed_file_count"
  if [[ -n "$output" ]]; then
    printf '%s\n' "$output"
  else
    printf '%s\n' 'golangci-lint failed without diagnostics'
  fi
  rm -rf "$temporary_directory"
  return "$exit_code"
}

coverage() {
  local coverage_profile=.cache/coverage.out
  local output
  local measured_coverage
  local go_root
  local PATH="$PATH"

  if ! require_command go 'install Go 1.25.2+ from https://go.dev/dl/'; then
    return 1
  fi

  if go_root=$(go env GOROOT 2>/dev/null) && [[ -x "$go_root/bin/go" ]]; then
    PATH="$go_root/bin:$PATH"
  fi

  if ! mkdir -p .cache; then
    printf '%s\n' 'could not create .cache for the coverage profile'
    return 1
  fi

  if ! go test -count=1 -coverpkg=./... -coverprofile="$coverage_profile" ./... >/dev/null 2>&1; then
    printf '%s\n' 'test suite failed'
    return 1
  fi

  if ! output=$(go tool cover -func="$coverage_profile" 2>&1); then
    printf 'could not read coverage profile: %s\n' "$(first_line "$output")"
    return 1
  fi

  if ! measured_coverage=$(awk '$1 == "total:" { gsub(/%/, "", $3); print $3; found = 1 } END { if (!found) exit 1 }' <<<"$output"); then
    printf 'could not read total coverage from %s\n' "$coverage_profile"
    return 1
  fi

  if awk -v measured="$measured_coverage" -v floor="$COVERAGE_FLOOR" 'BEGIN { exit !(measured >= floor) }'; then
    printf 'PASS  %s%% (floor %d%%)\n' "$measured_coverage" "$COVERAGE_FLOOR"
    return 0
  fi

  printf 'coverage %s%% is below floor %d%%\n' "$measured_coverage" "$COVERAGE_FLOOR"
  return 1
}

tests() {
  local output
  local failure
  local diagnostic

  if ! require_command go 'install Go 1.25.2+ from https://go.dev/dl/'; then
    return 1
  fi

  if output=$(go test ./... 2>&1); then
    printf '%s\n' 'PASS  go test ./...'
    return 0
  fi

  if failure=$(first_test_failure "$output"); then
    printf 'go test failed: %s\n' "$failure"
  elif diagnostic=$(first_go_diagnostic "$output"); then
    printf '%s\n' "$diagnostic"
  elif [[ -n "$output" ]]; then
    printf '%s\n' "$(first_line "$output")"
  else
    printf '%s\n' 'go test ./... failed'
  fi
  return 1
}

architecture() {
  local output

  if ! require_command go 'install Go 1.25.2+ from https://go.dev/dl/'; then
    return 1
  fi

  if output=$(go run ./scripts/architecture 2>&1); then
    printf '%s\n' "$output"
    return 0
  fi

  if [[ -n "$output" ]]; then
    printf '%s\n' "$output"
  else
    printf '%s\n' 'architecture checker failed'
  fi
  return 1
}

is_known_gate() {
  local requested=$1
  local gate

  for gate in "${gates[@]}"; do
    if [[ "$gate" == "$requested" ]]; then
      return 0
    fi
  done
  return 1
}

show_known_gates() {
  printf 'Known gates:\n'
  printf '  %s\n' "${gates[@]}"
}

first_line() {
  local value=$1

  printf '%s\n' "${value%%$'\n'*}"
}

remove_status_prefix() {
  local value=$1

  case "$value" in
    SKIP*) value=${value#SKIP} ;;
    PASS*) value=${value#PASS} ;;
    FAIL*) value=${value#FAIL} ;;
  esac
  value=${value# }
  value=${value# }
  printf '%s\n' "$value"
}

is_diagnostic_gate() {
  case "$1" in
    architecture|style|complexity) return 0 ;;
  esac
  return 1
}

run_gate() {
  local gate=$1
  local report
  local exit_code
  local result
  local note

  if case "$gate" in
    format) report=$(format 2>&1) ;;
    vet) report=$(vet 2>&1) ;;
    style) report=$(style 2>&1) ;;
    complexity) report=$(complexity 2>&1) ;;
    coverage) report=$(coverage 2>&1) ;;
    tests) report=$(tests 2>&1) ;;
    architecture) report=$(architecture 2>&1) ;;
  esac
  then
    exit_code=0
  else
    exit_code=1
  fi
  if (( exit_code != 0 )); then
    result=FAIL
    note=$(first_line "$report")
    if is_diagnostic_gate "$gate"; then
      note=$(remove_status_prefix "$note")
    fi
    if [[ -z "$note" ]]; then
      note="command failed"
    fi
  else
    report=$(first_line "$report")
    result=PASS
    note=""
    case "$report" in
      SKIP*)
        result=SKIP
        note=$(remove_status_prefix "$report")
        ;;
      PASS*)
        note=$(remove_status_prefix "$report")
        ;;
    esac
  fi

  results+=("$result")
  notes+=("$note")
  reports+=("$report")
  if [[ "$result" == FAIL ]]; then
    failed_gates+=("$gate")
  fi
  return 0
}

build_summary() {
  local summary="QUALITY GATE"
  local index
  local gate
  local result
  local note
  local line
  local report_line
  local report_line_index

  for (( index = 0; index < ${#selected_gates[@]}; index++ )); do
    gate=${selected_gates[$index]}
    result=${results[$index]}
    note=${notes[$index]}
    line=$(printf '  %-14s%s' "$gate" "$result")
    if [[ -n "$note" ]]; then
      line+="   $note"
    fi
    summary+=$'\n'"$line"

    if is_diagnostic_gate "$gate" && [[ "$result" == FAIL ]]; then
      report_line_index=0
      while IFS= read -r report_line; do
        if (( report_line_index > 0 )); then
          summary+=$'\n'"    $report_line"
        fi
        report_line_index=$((report_line_index + 1))
      done <<<"${reports[$index]}"
    fi
  done

  if (( ${#failed_gates[@]} > 0 )); then
    summary+=$'\n\nFAILED:'
    for gate in "${failed_gates[@]}"; do
      summary+=" $gate"
    done
  fi
  printf '%s\n' "$summary"
}

if (( $# > 1 )); then
  printf 'Usage: %s [gate|--list]\n' "$0"
  show_known_gates
  exit 2
fi

if (( $# == 0 )); then
  selected_gates=("${gates[@]}")
elif [[ "$1" == "--list" ]]; then
  if (( $# != 1 )); then
    printf 'Usage: %s [gate|--list]\n' "$0"
    show_known_gates
    exit 2
  fi
  printf '%s\n' "${gates[@]}"
  exit 0
else
  if ! is_known_gate "$1"; then
    printf 'Unknown gate: %s\n' "$1"
    show_known_gates
    exit 2
  fi
  selected_gates=("$1")
fi

for gate in "${selected_gates[@]}"; do
  run_gate "$gate"
done

summary=$(build_summary)
printf '%s\n' "$summary"

if [[ -n "${GITHUB_STEP_SUMMARY:-}" ]]; then
  if ! printf '%s\n' "$summary" >>"$GITHUB_STEP_SUMMARY"; then
    printf 'Could not write GitHub step summary: %s\n' "$GITHUB_STEP_SUMMARY" >&2
  fi
fi

if (( ${#failed_gates[@]} > 0 )); then
  exit 1
fi

exit 0
