#!/usr/bin/env bash
# test-manual-release.sh
# CI-only: a manual (workflow_dispatch) release takes its tag from INPUT_TAG alone, never from the branch it runs
# on. In a GitHub Actions run from a branch, it builds a new repository from this commit's tracked files, with
# .standards at its checked-out commit, and there, against the published tag INPUT_TAG names:
#   - make wait-for-publish ARGS=go and make pull-release-module succeed and fetch that tag's module zip;
#   - make pull-release-module with INPUT_TAG unset fails: the branch is never the tag.
# Skipped outside GitHub Actions. ci.yml's Manual release tag job runs it (make test-manual-release).
# Environment: GITHUB_ACTIONS (default empty), GITHUB_REF_TYPE (default empty), INPUT_TAG (default empty).

# bash configuration:
# 1) Exit script if you try to use an uninitialized variable.
set -o nounset

# 2) Exit script if a statement returns a non-true return value.
set -o errexit

# 3) Use the error status of the first failure, rather than that of the last item in a pipeline.
set -o pipefail

declare -rx GITHUB_ACTIONS="${GITHUB_ACTIONS:-}"
declare -rx GITHUB_REF_TYPE="${GITHUB_REF_TYPE:-}"
declare -rx INPUT_TAG="${INPUT_TAG:-}"
declare -ra SCRATCH_GIT=(git -c user.name=test-manual-release -c user.email=test-manual-release@invalid
  -c commit.gpgsign=false -c advice.addEmbeddedRepo=false)

function main() {
  exec 5>&1
  validate_args "${@:-}"
  on_actions || return 0
  check_context
  local repo
  repo="$(scratch_repo)"
  check_tag "${repo}"
  check_no_tag "${repo}"
  log "✅ A manual release from a branch takes its tag from INPUT_TAG (${INPUT_TAG}) alone"
}

function log() {
  local msg
  msg="${1:-}"
  printf '%s\n' "${msg}" | tee -a '/tmp/knobs_test_manual_release.log' >&5
}

function validate_args() {
  if [ "${#}" -gt 1 ] || [ -n "${1:-}" ]; then
    log '❌ Error: Unexpected argument'
    exit 1
  fi
}

# on_actions: true in GitHub Actions; elsewhere it reports the skip and is false.
function on_actions() {
  [ "${GITHUB_ACTIONS}" != 'true' ] || return 0
  log '⏭️  test-manual-release: skipped outside GitHub Actions'
  return 1
}

# check_context: a run from a branch (a manual release's context), with INPUT_TAG naming a tag.
function check_context() {
  if [ "${GITHUB_REF_TYPE}" != 'branch' ]; then
    log "❌ GITHUB_REF_TYPE is '${GITHUB_REF_TYPE}': run from a branch (workflow_dispatch)"
    exit 1
  fi
  if [ -z "${INPUT_TAG}" ]; then
    log '❌ INPUT_TAG is empty: name a published release tag'
    exit 1
  fi
}

# scratch_repo: a new repository in a temporary directory holding this commit's tracked files, with .standards
# as a repository of its own at its checked-out commit; prints its path.
function scratch_repo() {
  local dir
  dir="$(mktemp -d)"
  log "📁 New repository from $(git rev-parse --short HEAD) in ${dir}"
  git archive HEAD | tar -x -C "${dir}"
  mkdir -p "${dir}/.standards"
  git -C .standards archive HEAD | tar -x -C "${dir}/.standards"
  commit_all "${dir}/.standards"
  commit_all "${dir}"
  printf '%s' "${dir}"
}

# commit_all DIR: DIR becomes a repository with one commit of everything in it.
function commit_all() {
  "${SCRATCH_GIT[@]}" -C "${1}" init -q
  "${SCRATCH_GIT[@]}" -C "${1}" add -A
  "${SCRATCH_GIT[@]}" -C "${1}" commit -qm 'test-manual-release: scratch'
}

# check_tag DIR: the release's read-only steps, run in DIR, resolve INPUT_TAG and fetch its module zip.
function check_tag() {
  log "🧪 make wait-for-publish ARGS=go and make pull-release-module at ${INPUT_TAG}"
  (cd "${1}" && make wait-for-publish ARGS=go && make pull-release-module)
  if ! ls "${1}/dist/"*"-${INPUT_TAG}.zip" > /dev/null 2>&1; then
    log "❌ dist/ holds no module zip for ${INPUT_TAG}"
    exit 1
  fi
  log "✅ ${INPUT_TAG}: published, and its module zip matches SHA256SUMS"
}

# check_no_tag DIR: without INPUT_TAG, pull-release-module in DIR fails rather than take the branch for the tag.
function check_no_tag() {
  log '🧪 make pull-release-module with INPUT_TAG unset'
  if (cd "${1}" && env -u INPUT_TAG make pull-release-module); then
    log '❌ pull-release-module succeeded with INPUT_TAG unset: a branch was taken for the tag'
    exit 1
  fi
  log '✅ Without INPUT_TAG it fails: the branch is never the tag'
}

main "${@:-}"
