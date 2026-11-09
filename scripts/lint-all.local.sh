#!/usr/bin/env bash
# lint-all.local.sh
# knobs' hook of the canonical scripts/lint-all.sh (scripts/lib/local-hook.sh), so the pre-commit hook and
# `make lint-all` run knobs' own linters: the unit-test layout (scripts/lint/lint-test-layout.sh, knobs#13).
# `make lint` runs it as the lint-test-layout target.
#
# Run locally:  bash scripts/lint-all.local.sh

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
  bash "$(dirname "${BASH_SOURCE[0]}")/lint/lint-test-layout.sh"
}

function log() {
  local -r msg="${1:-}"
  printf '%s\n' "${msg}" | tee -a '/tmp/knobs_lint_all_local.log' >&5
}

function validate_args() {
  if [ "${#}" -gt 1 ] || [ -n "${1:-}" ]; then
    log '❌ Usage: lint-all.local.sh'
    exit 1
  fi
}

main "${@:-}"
