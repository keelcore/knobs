#!/usr/bin/env bash
# scripts/test/build-native.local.sh OS ARCH OUT EXT
# knobs' part of a native build: the conformance harness knobs-conform, cross-built for the platform into
# OUT/bin/, for the native integrity job (scripts/test/test-native-integrity.local.sh). It also records the
# module root package's directory in OUT/tests.tsv as ".": the canonical build-native.sh strips the
# working-directory prefix from subpackage directories only, so the root package keeps the dev image's
# absolute mount path, which the native host does not have.

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
  local -r os="${1}" arch="${2}" out="${3}" ext="${4:-}"
  log "🔨 knobs-conform for ${os}/${arch}"
  mkdir -p "${out}/bin"
  GOOS="${os}" GOARCH="${arch}" CGO_ENABLED=0 go build -trimpath -o "${out}/bin/knobs-conform${ext}" ./cmd/knobs-conform
  log "✅ ${out}/bin/knobs-conform${ext}"
  relative_root "${out}/tests.tsv"
}

function log() {
  local msg
  msg="${1:-}"
  printf '%s\n' "${msg}" | tee -a '/tmp/knobs_build_native_local.log' >&5
}

function validate_args() {
  if [ "${#}" -lt 3 ] || [ -z "${1:-}" ] || [ -z "${2:-}" ] || [ -z "${3:-}" ]; then
    log '❌ Usage: build-native.local.sh OS ARCH OUT [EXT]'
    exit 1
  fi
}

# relative_root TSV: the row whose directory is the working directory itself names it ".".
function relative_root() {
  local -r tsv="${1}"
  [ -f "${tsv}" ] || return 0
  awk -F '\t' -v root="${PWD}" 'BEGIN { OFS = "\t" } $1 == root { $1 = "." } { print }' "${tsv}" > "${tsv}.new"
  mv "${tsv}.new" "${tsv}"
  log "✅ root package directory recorded as . in ${tsv}"
}

main "${@:-}"
