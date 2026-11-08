#!/usr/bin/env bash
# build-conform.sh
# Builds the conformance harness (knobs-conform) of every implementation present in the tree into
# target/conform/knobs-conform-<impl>, the path tests/conformance.bats runs.
# Usage: build-conform.sh [IMPL]   (default: every implementation present)

# bash configuration:
# 1) Exit script if you try to use an uninitialized variable.
set -o nounset

# 2) Exit script if a statement returns a non-true return value.
set -o errexit

# 3) Use the error status of the first failure, rather than that of the last item in a pipeline.
set -o pipefail

declare -r OUT_DIR='target/conform'

function main() {
  exec 5>&1
  validate_args "${@:-}"
  mkdir -p "${OUT_DIR}"
  local impl
  for impl in $(impls "${1:-}"); do
    build_impl "${impl}"
  done
}

function log() {
  local msg
  msg="${1:-}"
  printf '%s\n' "${msg}" | tee -a '/tmp/knobs_build_conform.log' >&5
}

function validate_args() {
  if [ "${#}" -gt 1 ]; then
    log '❌ Usage: build-conform.sh [IMPL]'
    exit 1
  fi
}

# impls [IMPL]: IMPL alone, or every implementation whose sources are in the tree.
function impls() {
  if [ -n "${1:-}" ]; then
    printf '%s\n' "${1}"
    return 0
  fi
  [ ! -f go.mod ] || printf 'go\n'
}

function build_impl() {
  local -r impl="${1}"
  log "🔨 Building the ${impl} conformance harness..."
  case "${impl}" in
    go) go build -trimpath -o "${OUT_DIR}/knobs-conform-go" ./cmd/knobs-conform ;;
    *) log "❌ Unknown implementation: ${impl}"; exit 1 ;;
  esac
  log "✅ ${OUT_DIR}/knobs-conform-${impl}"
}

main "${@:-}"
