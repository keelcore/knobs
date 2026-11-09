#!/usr/bin/env bash
# test-native-scratch.sh DIR
# CI-only, on a native macOS or Windows runner: DIR is the native build build-native-scratch.sh made in a new
# repository, downloaded here. In a new repository made from this commit (scripts/lib/scratch-repo.sh), with DIR
# at the same path, make test-native and make test-native-integrity pass, the module root's tests ran (its log
# holds --- PASS lines), and the integrity hook took the harness from PATH, inside that repository.
# Skipped outside GitHub Actions. ci.yml's native jobs on a workflow_dispatch run it (make test-native-scratch).
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

function main() {
  exec 5>&1
  validate_args "${@:-}"
  on_actions || return 0
  check_build "${1}"
  local repo
  repo="$(scratch_repo)"
  log "📁 New repository from $(git rev-parse --short HEAD) in ${repo}"
  place_build "${repo}" "${1}"
  check_unit "${repo}" "${1}"
  check_integrity "${repo}" "${1}"
}

function log() {
  local msg
  msg="${1:-}"
  printf '%s\n' "${msg}" | tee -a "${TMPDIR:-/tmp}/knobs_test_native_scratch.log" >&5
}

function validate_args() {
  if [ "${#}" -ne 1 ] || [ -z "${1:-}" ]; then
    log '❌ Usage: test-native-scratch.sh target/native/<os>-<arch>   (made by make build-native-scratch)'
    exit 1
  fi
}

# on_actions: true in GitHub Actions; elsewhere it reports the skip and is false.
function on_actions() {
  [ "${GITHUB_ACTIONS}" != 'true' ] || return 0
  log '⏭️  test-native-scratch: skipped outside GitHub Actions'
  return 1
}

# check_build DIR: DIR holds a native build (its tests.tsv).
function check_build() {
  [ ! -f "${1}/tests.tsv" ] || return 0
  log "❌ ${1} holds no native build (tests.tsv): download the build-native-scratch artifact first"
  exit 1
}

# place_build REPO DIR: DIR, at the same path in REPO.
function place_build() {
  mkdir -p "${1}/$(dirname "${2}")"
  cp -R "${2}" "${1}/${2}"
}

# check_unit REPO DIR: the native unit tests pass in REPO, and the module root's test binary ran its tests.
function check_unit() {
  (cd "${1}" && make test-native DIR="${2}")
  if ! grep -q -- '--- PASS' "${1}/${2}/logs/_root"*.log; then
    log "❌ ${2}/logs/_root*.log holds no --- PASS line: the module root's tests did not run"
    exit 1
  fi
  log "✅ the module root's tests ran ($(grep -c -- '--- PASS' "${1}/${2}/logs/_root"*.log) passed)"
}

# check_integrity REPO DIR: the integrity hook passes in REPO with the harness PATH gives, from REPO's DIR/bin.
function check_integrity() {
  (cd "${1}" && make test-native-integrity DIR="${2}") 2>&1 | tee "${1}/integrity.out" >&5
  if ! grep -qE "Native harness: .*/$(basename "${1}")/${2}/bin/knobs-conform(\\.exe)? " "${1}/integrity.out"; then
    log "❌ the integrity hook did not take ${2}/bin/knobs-conform from PATH in the new repository"
    exit 1
  fi
  log "✅ the integrity hook took the harness from PATH, in the new repository"
}

main "${@:-}"
