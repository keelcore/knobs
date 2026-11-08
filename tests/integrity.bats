#!/usr/bin/env bats
# tests/integrity.bats
# Integrity of the built conformance harness (KNOBS_CONFORM, default target/conform/knobs-conform-go): it
# rejects bad invocations with the error object and exit status 2, never with a crash.
# Per .standards/governance/testing.md: integrity tests run against the BUILT artifact.

setup() {
  REPO_ROOT="$(cd "${BATS_TEST_DIRNAME}" && git rev-parse --show-toplevel)"
  BIN="${KNOBS_CONFORM:-${REPO_ROOT}/target/conform/knobs-conform-go}"
  if [ ! -x "${BIN}" ]; then
    skip "Built artifact not found at ${BIN}; build first (make build-conform)"
  fi
}

@test "binary exists and is executable" {
  [ -x "${BIN}" ]
}

@test "no command fails with invalid_argument" {
  run --separate-stderr "${BIN}" --app demo
  [ "${status}" -eq 2 ]
  [ "$(jq -r .error.code <<< "${output}")" = 'invalid_argument' ]
}

@test "unknown flag produces a clean failure (no panic / stack trace)" {
  run --separate-stderr "${BIN}" --not-a-real-flag-please resolve
  [ "${status}" -eq 2 ]
  [ "$(jq -r .error.code <<< "${output}")" = 'invalid_argument' ]
  [[ "${output}" != *"goroutine"* ]] || false
}
