#!/usr/bin/env bash

# Keep running after a failed gate so the summary can report every gate.
set -u

gates=(format vet style complexity coverage tests architecture)
selected_gates=()
results=()
notes=()
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
  # Put the style command for this project here.
  printf '%s\n' "SKIP  not configured"
}

complexity() {
  # Put the complexity command for this project here.
  printf '%s\n' "SKIP  not configured"
}

coverage() {
  # Put the coverage command for this project here.
  printf '%s\n' "SKIP  not configured"
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
  # Put the architecture command for this project here.
  printf '%s\n' "SKIP  not configured"
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
  esac
  value=${value# }
  value=${value# }
  printf '%s\n' "$value"
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

  for (( index = 0; index < ${#selected_gates[@]}; index++ )); do
    gate=${selected_gates[$index]}
    result=${results[$index]}
    note=${notes[$index]}
    line=$(printf '  %-14s%s' "$gate" "$result")
    if [[ -n "$note" ]]; then
      line+="   $note"
    fi
    summary+=$'\n'"$line"
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
