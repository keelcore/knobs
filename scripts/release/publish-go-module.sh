#!/usr/bin/env bash
# publish-go-module.sh
# Publishes the Go module of this repository at the release tag: verifies the module builds, then
# asks the Go checksum database and module proxy for the module at the tag, which records it in both
# (pkg.go.dev reads the proxy). The module path is read from go.mod.
# Tag: release_tag (scripts/lib/release-tag.sh): INPUT_TAG, else GITHUB_REF_NAME on a tag push.
# knobs-owned: the canonical publish-go.sh publishes the .standards module only.

# bash configuration:
# 1) Exit script if you try to use an uninitialized variable.
set -o nounset

# 2) Exit script if a statement returns a non-true return value.
set -o errexit

# 3) Use the error status of the first failure, rather than that of the last item in a pipeline.
set -o pipefail

# shellcheck source=../lib/release-tag.sh
source "$(dirname "${BASH_SOURCE[0]}")/../lib/release-tag.sh"

function main() {
  exec 5>&1
  validate_args "${@:-}"
  local tag module
  tag="$(release_tag)"
  module="$(module_path)"
  log "📦 Publishing ${module}@${tag}"
  verify_build
  notify "https://sum.golang.org/lookup/${module}@${tag}"
  notify "https://proxy.golang.org/${module}/@v/${tag}.info"
  log "✅ ${module}@${tag} is in the checksum database and the proxy"
}

function log() {
  local msg
  msg="${1:-}"
  printf '%s\n' "${msg}" | tee -a '/tmp/knobs_publish_go_module.log' >&5
}

function validate_args() {
  if [ "${#}" -gt 1 ] || [ -n "${1:-}" ]; then
    log '❌ Error: Unexpected argument'
    exit 1
  fi
}

function module_path() {
  if [ ! -f go.mod ]; then
    log '❌ go.mod not found; run from the repository root'
    exit 1
  fi
  awk '$1 == "module" { print $2; exit }' go.mod
}

function verify_build() {
  log '🔨 Verifying the module builds'
  go build ./...
}

function notify() {
  local -r url="${1}"
  log "🔔 ${url}"
  curl --silent --connect-timeout 30 --speed-limit 1024 --speed-time 60 --fail --show-error "${url}" > /dev/null
}

main "${@:-}"
