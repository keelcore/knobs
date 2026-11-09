#!/usr/bin/env bash
# scripts/test/test-native-integrity.local.sh
# What knobs runs natively, from the native build the dev image cross-built and the job downloaded: on macOS the
# conformance suite and the harness integrity checks against the native harness; on Windows (knobs-conform.exe)
# the integrity checks, as a smoke test. scripts/test/test-native-integrity.sh runs this hook with the build's
# bats/bin and bin first on PATH, in the host's own form, and NATIVE_DIR set to the build: bats and the harness
# come from PATH.
# Environment: NATIVE_DIR (default empty).

# bash configuration:
# 1) Exit script if you try to use an uninitialized variable.
set -o nounset

# 2) Exit script if a statement returns a non-true return value.
set -o errexit

# 3) Use the error status of the first failure, rather than that of the last item in a pipeline.
set -o pipefail

declare -rx NATIVE_DIR="${NATIVE_DIR:-}"

function main() {
  exec 5>&1
  validate_args "${@:-}"
  local harness
  harness="$(native_harness)"
  log "🧪 Native harness: ${harness} (build ${NATIVE_DIR})"
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
  if [ -z "${NATIVE_DIR}" ]; then
    log '❌ NATIVE_DIR is unset: run make test-native-integrity DIR=target/native/<os>-<arch>'
    exit 1
  fi
}

# native_harness: the harness's absolute path, from PATH; knobs-conform.exe first, the Windows build's name.
function native_harness() {
  command -v knobs-conform.exe && return 0
  command -v knobs-conform && return 0
  log "❌ knobs-conform is in no PATH directory: ${PATH}"
  exit 1
}

main "${@:-}"
