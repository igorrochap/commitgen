#!/usr/bin/env bash

# Keep running after a failed gate so the summary can report every gate.
set -u

gates=(format vet style complexity coverage tests architecture)
selected_gates=()
results=()
notes=()
failed_gates=()

format() {
  # Put the format command for this project here.
  printf '%s\n' "SKIP  not configured"
}

vet() {
  # Put the vet command for this project here.
  printf '%s\n' "SKIP  not configured"
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
  # Put the tests command for this project here.
  printf '%s\n' "SKIP  not configured"
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
