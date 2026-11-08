#!/usr/bin/env bash
# scripts/test/test-native-integrity.local.sh
# What knobs runs natively (scripts/test/test-native-integrity.sh puts the cross-built bats and knobs-conform
# first on PATH): on macOS the conformance suite and the harness integrity checks against the native
# harness; on Windows (knobs-conform.exe) the integrity checks, as a smoke test.

# bash configuration:
# 1) Exit script if you try to use an uninitialized variable.
set -o nounset

# 2) Exit script if a statement returns a non-true return value.
set -o errexit

# 3) Use the error status of the first failure, rather than that of the last item in a pipeline.
set -o pipefail

function main() {
  exec 5>&1
  validate_args "${@:-}"
  local harness
  harness="$(native_harness)"
  log "🧪 Native harness: ${harness}"
  if [[ "${harness}" == *.exe ]]; then
    KNOBS_CONFORM="${harness}" bats tests/integrity.bats
    return 0
  fi
  KNOBS_IMPL=go KNOBS_CONFORM="${harness}" bats tests/conformance.bats
  KNOBS_CONFORM="${harness}" bats tests/integrity.bats
}

function log() {
  local msg
  msg="${1:-}"
  printf '%s\n' "${msg}" | tee -a "${TMPDIR:-/tmp}/knobs_native_integrity.log" >&5
}

function validate_args() {
  if [ "${#}" -gt 1 ] || [ -n "${1:-}" ]; then
    log '❌ Error: Unexpected argument'
    exit 1
  fi
}

# native_harness: the knobs-conform (knobs-conform.exe on Windows) in a PATH directory, which the native build
# placed there; found by file, not by command lookup, so the result does not depend on the shell's executable rules.
function native_harness() {
  local dir
  local -a dirs
  IFS=: read -r -a dirs <<< "${PATH}"
  for dir in "${dirs[@]}"; do
    harness_in "${dir}" && return 0
  done
  log "❌ knobs-conform is in no PATH directory: ${PATH}"
  exit 1
}

# harness_in DIR: prints DIR's knobs-conform.exe or knobs-conform, if it holds one.
function harness_in() {
  local name
  for name in knobs-conform.exe knobs-conform; do
    if [ -f "${1}/${name}" ]; then
      printf '%s' "${1}/${name}"
      return 0
    fi
  done
  return 1
}

main "${@:-}"
