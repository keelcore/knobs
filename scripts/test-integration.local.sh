#!/usr/bin/env bash
# test-integration.local.sh
# knobs integration tests, delegated to by the canonical scripts/test-integration.sh
# (`make test-integration`): builds every implementation's conformance harness, runs the
# conformance suite once per implementation (KNOBS_IMPL), then every other BATS suite under tests/.
# Project-owned, never canonical.

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
  bash scripts/build/build-conform.sh
  local impl
  for impl in $(built_impls); do
    log "🧪 Conformance: ${impl}"
    KNOBS_IMPL="${impl}" bats tests/conformance.bats
  done
  run_other_suites
  log '✅ BATS suites passed'
}

function log() {
  local msg
  msg="${1:-}"
  printf '%s\n' "${msg}" | tee -a '/tmp/knobs_test_integration.log' >&5
}

function validate_args() {
  if [ "${#}" -gt 1 ] || [ -n "${1:-}" ]; then
    log '❌ Error: Unexpected argument'
    exit 1
  fi
}

# built_impls: the implementations whose harness build-conform.sh produced.
function built_impls() {
  local f
  for f in target/conform/knobs-conform-*; do
    [ ! -x "${f}" ] || printf '%s\n' "${f##*knobs-conform-}"
  done
}

# run_other_suites: every BATS file and directory under tests/ except the conformance suite.
function run_other_suites() {
  log '🧪 Other BATS suites'
  local suites=()
  local suite
  while IFS= read -r suite; do
    suites+=("${suite}")
  done < <(find tests -mindepth 1 -maxdepth 1 \( -name '*.bats' -o -type d \) ! -name conformance.bats \
    ! -name fixtures | LC_ALL=C sort)
  bats --recursive "${suites[@]}"
}

main "${@:-}"
