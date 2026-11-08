#!/usr/bin/env bash
# tests/fixtures/stub-curl.sh
# curl stand-in for BATS: records each URL in STUB_CALLS and each -H header (also -H @-, read from stdin) in
# STUB_HEADERS (when set), and writes the file of STUB_SERVED named by the URL's last path segment to the -o path,
# so a test serves release assets with no network.

# bash configuration:
# 1) Exit script if you try to use an uninitialized variable.
set -o nounset

# 2) Exit script if a statement returns a non-true return value.
set -o errexit

# 3) Use the error status of the first failure, rather than that of the last item in a pipeline.
set -o pipefail

function main() {
  local out='' url=''
  while [ "${#}" -gt 0 ]; do
    case "${1}" in
      -o) out="${2}"; shift ;;
      -H) record_header "${2}"; shift ;;
      http*) url="${1}" ;;
    esac
    shift
  done
  printf '%s\n' "${url}" >> "${STUB_CALLS}"
  cp "${STUB_SERVED}/${url##*/}" "${out}"
}

# record_header VALUE: records VALUE, or each line of stdin for -H @- (headers curl reads from stdin).
function record_header() {
  [ -n "${STUB_HEADERS:-}" ] || return 0
  if [ "${1}" = '@-' ]; then
    cat >> "${STUB_HEADERS}"
    return 0
  fi
  printf '%s\n' "${1}" >> "${STUB_HEADERS}"
}

main "${@:-}"
