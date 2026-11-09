#!/usr/bin/env bash
# pull-release-module.sh
# Fetches the GitHub Release's SHA256SUMS and module zip for the release tag into dist/, from the
# repository go.mod names (github.com/OWNER/REPO), and fails unless the zip matches its SHA256SUMS
# line. release.yml attests the zip's build provenance from dist/.
# Tag: release_tag (scripts/lib/release-tag.sh): INPUT_TAG, else GITHUB_REF_NAME on a tag push.
# Environment: GITHUB_TOKEN (default empty).

# bash configuration:
# 1) Exit script if you try to use an uninitialized variable.
set -o nounset

# 2) Exit script if a statement returns a non-true return value.
set -o errexit

# 3) Use the error status of the first failure, rather than that of the last item in a pipeline.
set -o pipefail

# shellcheck source=../lib/release-tag.sh
source "$(dirname "${BASH_SOURCE[0]}")/../lib/release-tag.sh"

declare -rx GITHUB_TOKEN="${GITHUB_TOKEN:-}"
declare -r DIST='dist'
declare -ra CURL_OPTS=(--silent --show-error --fail --location --connect-timeout 30 --retry 3)

function main() {
  exec 5>&1
  validate_args "${@:-}"
  local tag base zip
  tag="$(release_tag)"
  base="https://$(module_path)/releases/download/${tag}"
  mkdir -p "${DIST}"
  fetch "${base}/SHA256SUMS" "${DIST}/SHA256SUMS"
  zip="$(module_zip "${tag}")"
  fetch "${base}/${zip}" "${DIST}/${zip}"
  verify "${zip}"
}

function log() {
  local msg
  msg="${1:-}"
  printf '%s\n' "${msg}" | tee -a '/tmp/knobs_pull_release_module.log' >&5
}

function validate_args() {
  if [ "${#}" -gt 1 ] || [ -n "${1:-}" ]; then
    log '❌ Error: Unexpected argument'
    exit 1
  fi
}

function module_path() {
  awk '$1 == "module" { print $2; exit }' go.mod
}

# fetch URL OUT: with GITHUB_TOKEN set the request is authenticated, which GitHub can require of release downloads
# that it refuses anonymously. The token reaches curl on stdin (-H @-), never on its command line; curl does not
# forward it when a redirect leaves the host.
function fetch() {
  local -r url="${1}" out="${2}"
  log "⬇️  ${url}"
  if [ -z "${GITHUB_TOKEN}" ]; then
    curl "${CURL_OPTS[@]}" -o "${out}" "${url}"
    return 0
  fi
  printf 'Authorization: Bearer %s\n' "${GITHUB_TOKEN}" | curl "${CURL_OPTS[@]}" -H @- -o "${out}" "${url}"
}

# module_zip TAG: the zip SHA256SUMS lists for TAG.
function module_zip() {
  local zip
  zip="$(awk -v suffix="-${1}.zip" '{ n = $2; sub(/^\*/, "", n) } substr(n, length(n) - length(suffix) + 1) == suffix { print n; exit }' "${DIST}/SHA256SUMS")"
  if [ -z "${zip}" ]; then
    log "❌ SHA256SUMS lists no module zip for ${1}"
    exit 1
  fi
  printf '%s' "${zip}"
}

function verify() {
  local -r zip="${1}"
  (cd "${DIST}" && grep -E "[ *]${zip}\$" SHA256SUMS | sha256sum --check --quiet)
  log "✅ ${DIST}/${zip} matches SHA256SUMS"
}

main "${@:-}"
