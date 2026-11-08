#!/usr/bin/env bash
# create-release.local.sh
# knobs bump hook for the canonical scripts/release/create-release.sh.
# Project-owned, never canonical: verify-canonical and refresh-governance skip
# *.local.sh, so consumers never receive this file.
#
# Usage: create-release.local.sh <previous-tag>
#
# Applies the bump policy stated in CHANGELOG.md to the "## [Unreleased]" entries added since
# <previous-tag> (entries the previous tag's CHANGELOG.md already had are not new): any
# "### Removed" entry -> major; else any "### Added" or "### Changed" entry -> minor; else patch.
# Fails when there are none (nothing to release).
#
# stdout is the hook contract: line 1 the bump level, line 2 the tag subject.
# Findings and errors go to stderr (the driver shows them and records them in
# the tag body).

# bash configuration:
# 1) Exit script if you try to use an uninitialized variable.
set -o nounset

# 2) Exit script if a statement returns a non-true return value.
set -o errexit

# 3) Use the error status of the first failure, rather than that of the last item in a pipeline.
set -o pipefail

declare -r CHANGELOG='CHANGELOG.md'
declare -r LOG_FILE='/tmp/create_release_local.log'
declare -ra HEADINGS=(Removed Added Changed Deprecated Fixed Security)

function main() {
  # stdout carries the hook contract; all messaging goes to stderr.
  exec 5>&2
  validate_args "${@:-}"
  local section
  section="$(unreleased_section | new_since "${1}")"
  require_entries "${section}"
  report_counts "${1}" "${section}"
  printf '%s\n' "$(bump_for "${section}")"
  printf '%s\n' "$(subject_for "${section}")"
}

function log() {
  local -r msg="${1:-}"
  printf '%s\n' "${msg}" | tee -a "${LOG_FILE}" >&5
}

function validate_args() {
  if [ "${#}" -ne 1 ] || [ -z "${1:-}" ]; then
    log 'Usage: create-release.local.sh <previous-tag>'
    exit 1
  fi
}

# Body of "## [Unreleased]", up to the next "## " heading.
function unreleased_section() {
  if [ ! -f "${CHANGELOG}" ]; then
    log "${CHANGELOG} not found"
    exit 1
  fi
  awk '/^## \[Unreleased\]/ { p = 1; next } /^## / { if (p) exit } p' "${CHANGELOG}"
}

# new_since TAG: the [Unreleased] section on stdin, keeping only the entries TAG's CHANGELOG.md does not
# have (an entry is its "- " line and the indented lines after it), under their "### " headings.
function new_since() {
  local -r prev="$(mktemp)"
  git show "${1}:${CHANGELOG}" > "${prev}" 2> /dev/null || true
  awk '
    FILENAME == ARGV[1] { seen[$0] = 1; next }
    /^### / { head = $0; next }
    /^- / { keep = !($0 in seen); if (keep && !(head in shown)) { print ""; print head; print ""; shown[head] = 1 } }
    /^- / || /^  / { if (keep) print }
  ' "${prev}" -
  rm -f "${prev}"
}

function require_entries() {
  local -r section="${1}"
  if ! printf '%s\n' "${section}" | grep -q '^- '; then
    log "${CHANGELOG} [Unreleased] has no entries since the previous tag; nothing to release"
    exit 1
  fi
}

# Number of top-level entries under "### <heading>" in the section.
function count_entries() {
  local -r section="${1}" heading="${2}"
  printf '%s\n' "${section}" | awk -v h="### ${heading}" '
    $0 == h { p = 1; next }
    /^### / { p = 0 }
    p && /^- / { n++ }
    END { print n + 0 }'
}

function bump_for() {
  local -r section="${1}"
  if [ "$(count_entries "${section}" Removed)" -gt 0 ]; then
    printf 'major'
  elif [ "$(count_entries "${section}" Added)" -gt 0 ] || [ "$(count_entries "${section}" Changed)" -gt 0 ]; then
    printf 'minor'
  else
    printf 'patch'
  fi
}

# "CHANGELOG: 1 removed, 16 added" — non-zero headings only, in HEADINGS order.
function subject_for() {
  local -r section="${1}"
  local heading n parts=''
  for heading in "${HEADINGS[@]}"; do
    n="$(count_entries "${section}" "${heading}")"
    if [ "${n}" -gt 0 ]; then
      parts="${parts:+${parts}, }${n} $(printf '%s' "${heading}" | awk '{ print tolower($0) }')"
    fi
  done
  printf 'CHANGELOG: %s' "${parts:-no categorized entries}"
}

function report_counts() {
  local -r tag="${1}" section="${2}"
  local heading
  log "${CHANGELOG} [Unreleased] since ${tag}:"
  for heading in "${HEADINGS[@]}"; do
    log "  ${heading}: $(count_entries "${section}" "${heading}")"
  done
}

main "${@:-}"
