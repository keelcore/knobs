#!/usr/bin/env bats
# tests/canonical/smoke.bats
# Canonical harness sanity. Verifies BATS itself runs and basic shell
# expectations hold. No project dependencies; runs in any environment
# with bats installed.
#
# Created by bootstrap-standards.sh template; safe to extend with
# additional repo-agnostic sanity checks.

@test "bats produces output and reports success" {
  run echo "ok"
  [ "${status}" -eq 0 ]
  [ "${output}" = "ok" ]
}

@test "the repo root is a git work tree" {
  run git rev-parse --is-inside-work-tree
  [ "${status}" -eq 0 ]
  [ "${output}" = "true" ]
}

@test "the .standards submodule is present" {
  [ -d "${BATS_TEST_DIRNAME}/../../.standards" ]
  [ -f "${BATS_TEST_DIRNAME}/../../.standards/governance/ci.md" ]
}

@test "canonical scripts/lib/paths.sh exists and parses as bash" {
  local paths_sh="${BATS_TEST_DIRNAME}/../../scripts/lib/paths.sh"
  [ -f "${paths_sh}" ]
  bash -n "${paths_sh}"
}
