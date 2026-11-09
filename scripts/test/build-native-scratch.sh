#!/usr/bin/env bash
# build-native-scratch.sh OS ARCH
# CI-only: the native build of a new repository made from this commit (scripts/lib/scratch-repo.sh), so no target/
# or other local state comes along. It runs make build-native there and checks that tests.tsv names each package
# by its directory in the repository, never an absolute path, with the module root as . and every package below
# listed. The build is copied to target/native/OS-ARCH here, for the native runner's job (test-native-scratch.sh).
# Skipped outside GitHub Actions. ci.yml's native jobs on a workflow_dispatch run it (make build-native-scratch).
# Environment: GITHUB_ACTIONS (default empty).

# bash configuration:
# 1) Exit script if you try to use an uninitialized variable.
set -o nounset

# 2) Exit script if a statement returns a non-true return value.
set -o errexit

# 3) Use the error status of the first failure, rather than that of the last item in a pipeline.
set -o pipefail

# shellcheck source=../lib/scratch-repo.sh
source "$(dirname "${BASH_SOURCE[0]}")/../lib/scratch-repo.sh"

declare -rx GITHUB_ACTIONS="${GITHUB_ACTIONS:-}"
# The packages with tests, by directory: the module root and each subpackage.
declare -ra PACKAGES=(. internal/bumpcli internal/conform knobstest)

function main() {
  exec 5>&1
  validate_args "${@:-}"
  on_actions || return 0
  local -r build="target/native/${1}-${2}"
  local repo
  repo="$(scratch_repo)"
  log "📁 New repository from $(git rev-parse --short HEAD) in ${repo}"
  (cd "${repo}" && make build-native OS="${1}" ARCH="${2}")
  check_tests_tsv "${repo}/${build}/tests.tsv"
  copy_build "${repo}" "${build}"
}

function log() {
  local msg
  msg="${1:-}"
  printf '%s\n' "${msg}" | tee -a '/tmp/knobs_build_native_scratch.log' >&5
}

function validate_args() {
  case "${1:-}:${2:-}" in
    darwin:arm64 | darwin:amd64 | windows:amd64 | windows:arm64) ;;
    *)
      log '❌ Usage: build-native-scratch.sh darwin|windows arm64|amd64'
      exit 1
      ;;
  esac
}

# on_actions: true in GitHub Actions; elsewhere it reports the skip and is false.
function on_actions() {
  [ "${GITHUB_ACTIONS}" != 'true' ] || return 0
  log '⏭️  build-native-scratch: skipped outside GitHub Actions'
  return 1
}

# check_tests_tsv TSV: no absolute directory (/x, or a drive letter), and every one of PACKAGES has a row.
function check_tests_tsv() {
  local -r tsv="${1}"
  if cut -f1 "${tsv}" | grep -qE '^(/|[A-Za-z]:)'; then
    log "❌ ${tsv} names a package by an absolute path: $(cut -f1 "${tsv}" | tr '\n' ' ')"
    exit 1
  fi
  local pkg
  for pkg in "${PACKAGES[@]}"; do
    cut -f1 "${tsv}" | grep -qxF "${pkg}" || { log "❌ ${tsv} has no row for ${pkg}"; exit 1; }
  done
  log "✅ tests.tsv: every package by its directory in the repository (${PACKAGES[*]})"
}

# copy_build REPO BUILD: REPO's BUILD replaces this repository's, for the upload.
function copy_build() {
  rm -rf "${2}"
  mkdir -p "$(dirname "${2}")"
  cp -R "${1}/${2}" "${2}"
  log "✅ ${2}: the new repository's native build"
}

main "${@:-}"
