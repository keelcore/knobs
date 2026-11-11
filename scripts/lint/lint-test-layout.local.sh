#!/usr/bin/env bash
# lint-test-layout.local.sh
# Check the unit-test layout: all unit tests for {foo}.go are in {foo}_test.go, in the same directory
# (knobs#13). Fails, naming each file, on:
#   - a test file with no {foo}.go beside it, unless config/test-layout.local.txt lists it as a helpers-only
#     file (one path per line, # comments);
#   - a listed helpers-only file that holds a Test, Fuzz, Benchmark or Example function;
#   - a source file with no {foo}_test.go beside it, except cmd/*/main.go (exercised through the built binary)
#     and doc.go (no code).
# Exits 1 if violations are found (CI mode).
#
# Run locally:  bash scripts/lint/lint-test-layout.local.sh (make lint-test-layout-local, part of make lint)

# bash configuration:
# 1) Exit script if you try to use an uninitialized variable.
set -o nounset

# 2) Exit script if a statement returns a non-true return value.
set -o errexit

# 3) Use the error status of the first failure, rather than that of the last item in a pipeline.
set -o pipefail

# shellcheck source=../lib/paths.sh
source "$(dirname "${BASH_SOURCE[0]}")/../lib/paths.sh"

declare -r HELPERS_LIST='config/test-layout.local.txt'

function main() {
  exec 5>&1
  validate_args "${@:-}"
  log '🔍 Unit-test layout: tests for {foo}.go in {foo}_test.go...'
  local -a helpers=()
  local line
  if [ -f "${HELPERS_LIST}" ]; then
    while IFS= read -r line; do
      helpers+=("${line}")
    done < <(grep -v -e '^#' -e '^[[:space:]]*$' "${HELPERS_LIST}")
  fi
  local failed=0 f
  while IFS= read -r f; do
    [ -f "${f}" ] || continue
    check_file "${f}" "${helpers[@]+"${helpers[@]}"}" || failed=1
  done < <(source_files '*.go')
  if [ "${failed}" -ne 0 ]; then
    log '❌ Unit-test layout check failed'
    exit 1
  fi
  log '✅ Unit-test layout: every test file has its source, every source its test file'
}

function log() {
  local -r msg="${1:-}"
  printf '%s\n' "${msg}" | tee -a '/tmp/knobs_lint_test_layout.log' >&5
}

function validate_args() {
  if [ "${#}" -gt 1 ] || [ -n "${1:-}" ]; then
    log '❌ Usage: lint-test-layout.local.sh'
    exit 1
  fi
}

# check_file FILE [HELPER...]: FILE against the rule; returns 1 on a violation, having logged it.
function check_file() {
  local -r f="${1}"
  shift
  if [[ "${f}" == *_test.go ]]; then
    check_test_file "${f}" "$@"
    return
  fi
  if [[ "${f}" == cmd/*/main.go || "${f}" == doc.go || "${f}" == */doc.go ]]; then
    return 0
  fi
  if [ ! -f "${f%.go}_test.go" ]; then
    log "❌ ${f}: no ${f%.go}_test.go"
    return 1
  fi
}

function check_test_file() {
  local -r f="${1}"
  shift
  local h
  for h in "$@"; do
    [ "${h}" = "${f}" ] || continue
    if grep -qE '^func (Test|Fuzz|Benchmark|Example)' "${f}"; then
      log "❌ ${f}: listed as helpers-only in ${HELPERS_LIST}, but holds tests"
      return 1
    fi
    return 0
  done
  if [ ! -f "${f%_test.go}.go" ]; then
    log "❌ ${f}: no ${f%_test.go}.go; its tests belong in the test file of the source they test"
    return 1
  fi
}

main "${@:-}"
