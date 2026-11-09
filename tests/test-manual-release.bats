#!/usr/bin/env bats
# tests/test-manual-release.bats
# scripts/test/test-manual-release.sh, the CI-only manual-release check: it runs only in GitHub Actions, only in a
# run from a branch (a manual release's context), and only with INPUT_TAG naming the tag; otherwise it builds no
# repository and runs no target. Its full run needs a runner, a container runtime and the network: ci.yml's
# Manual release tag job.

setup() {
  ROOT="$(cd "${BATS_TEST_DIRNAME}/.." && pwd -P)"
  SCRIPT="${ROOT}/scripts/test/test-manual-release.sh"
  CALLS="${BATS_TEST_TMPDIR}/calls"
  mkdir -p "${BATS_TEST_TMPDIR}/bin"
  : > "${CALLS}"
  for tool in make git; do
    printf '#!/usr/bin/env bash\nprintf "%s|%%s\\n" "$*" >> "%s"\n' "${tool}" "${CALLS}" > "${BATS_TEST_TMPDIR}/bin/${tool}"
    chmod +x "${BATS_TEST_TMPDIR}/bin/${tool}"
  done
  export PATH="${BATS_TEST_TMPDIR}/bin:${PATH}"
  unset GITHUB_ACTIONS GITHUB_REF_TYPE INPUT_TAG
  cd "${ROOT}"
}

@test "test-manual-release: outside GitHub Actions it skips and runs nothing" {
  run bash "${SCRIPT}"
  [ "${status}" -eq 0 ]
  [[ "${output}" == *skipped* ]] || false
  [ ! -s "${CALLS}" ]
}

@test "test-manual-release: in a tag-push run it fails and runs nothing" {
  GITHUB_ACTIONS=true GITHUB_REF_TYPE=tag INPUT_TAG=v0.8.2 run bash "${SCRIPT}"
  [ "${status}" -ne 0 ]
  [[ "${output}" == *GITHUB_REF_TYPE* ]] || false
  [ ! -s "${CALLS}" ]
}

@test "test-manual-release: without INPUT_TAG it fails and runs nothing" {
  GITHUB_ACTIONS=true GITHUB_REF_TYPE=branch run bash "${SCRIPT}"
  [ "${status}" -ne 0 ]
  [[ "${output}" == *INPUT_TAG* ]] || false
  [ ! -s "${CALLS}" ]
}

@test "test-manual-release: an argument is rejected" {
  run bash "${SCRIPT}" extra
  [ "${status}" -ne 0 ]
}
