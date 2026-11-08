#!/usr/bin/env bash
# scripts/test/test-native-integrity.local.sh
# What knobs runs natively, from the native build the dev image cross-built and the job downloaded
# (target/native/<os>-<arch>/): on macOS the conformance suite and the harness integrity checks against the
# native harness; on Windows (knobs-conform.exe) the integrity checks, as a smoke test. The tools are found under
# target/native/ by relative path, not on PATH: on Windows the PATH that scripts/test/test-native-integrity.sh
# prepends holds drive-letter paths (D:/...), whose colon splits the entry.

# bash configuration:
# 1) Exit script if you try to use an uninitialized variable.
set -o nounset

# 2) Exit script if a statement returns a non-true return value.
set -o errexit

# 3) Use the error status of the first failure, rather than that of the last item in a pipeline.
set -o pipefail

declare -r NATIVE='target/native'

function main() {
  exec 5>&1
  validate_args "${@:-}"
  local dir harness
  dir="$(native_dir)"
  harness="$(native_harness "${dir}")"
  log "🧪 Native harness: ${harness}"
  if [[ "${harness}" == *.exe ]]; then
    KNOBS_CONFORM="${harness}" "${dir}/bats/bin/bats" tests/integrity.bats
    return 0
  fi
  KNOBS_IMPL=go KNOBS_CONFORM="${harness}" "${dir}/bats/bin/bats" tests/conformance.bats
  KNOBS_CONFORM="${harness}" "${dir}/bats/bin/bats" tests/integrity.bats
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

# native_dir: the one native build under target/native/ (the job downloads the build for its own platform).
function native_dir() {
  local dir found=''
  for dir in "${NATIVE}"/*/; do
    [ -f "${dir}bats/bin/bats" ] || continue
    [ -z "${found}" ] || { log "❌ more than one native build under ${NATIVE}: ${found} ${dir}"; exit 1; }
    found="${dir%/}"
  done
  [ -n "${found}" ] || { log "❌ no native build under ${NATIVE}: run make build-native first"; exit 1; }
  printf '%s' "${found}"
}

# native_harness DIR: the absolute path of DIR's knobs-conform.exe or knobs-conform (the suites run it from scratch
# directories; an absolute command path is fine on Windows, where only PATH entries split at the drive colon).
function native_harness() {
  local name
  for name in knobs-conform.exe knobs-conform; do
    if [ -f "${1}/bin/${name}" ]; then
      printf '%s/%s' "$(cd "${1}/bin" && pwd)" "${name}"
      return 0
    fi
  done
  log "❌ ${1}/bin holds no knobs-conform"
  exit 1
}

main "${@:-}"
