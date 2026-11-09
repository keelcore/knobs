#!/usr/bin/env bats
# tests/native-scratch.bats
# The CI-only native checks in a new repository: scripts/lib/scratch-repo.sh builds one from this commit, and
# scripts/test/build-native-scratch.sh and test-native-scratch.sh run only in GitHub Actions, with valid
# arguments, before they build or run anything. Their full runs need a container runtime and macOS and Windows
# runners: ci.yml's jobs on a workflow_dispatch.

setup() {
  ROOT="$(cd "${BATS_TEST_DIRNAME}/.." && pwd -P)"
  CALLS="${BATS_TEST_TMPDIR}/calls"
  mkdir -p "${BATS_TEST_TMPDIR}/bin"
  : > "${CALLS}"
  printf '#!/usr/bin/env bash\nprintf "make|%%s\\n" "$*" >> "%s"\n' "${CALLS}" > "${BATS_TEST_TMPDIR}/bin/make"
  chmod +x "${BATS_TEST_TMPDIR}/bin/make"
  unset GITHUB_ACTIONS
  cd "${ROOT}"
}

# scratch: the path of a new repository from this commit (bats calls tests inside functions, where the library's
# `declare -r` globals would be local, so it runs in its own bash).
scratch() {
  bash -c 'source scripts/lib/scratch-repo.sh && scratch_repo'
}

@test "scratch-repo: the new repository holds this commit's tracked files, in one commit" {
  local repo
  repo="$(scratch)"
  [ -d "${repo}" ]
  cmp <(git show HEAD:go.mod) "${repo}/go.mod"
  [ "$(git -C "${repo}" rev-list --count HEAD)" -eq 1 ]
  [ -z "$(git -C "${repo}" status --porcelain)" ]
  [ ! -e "${repo}/target" ]
}

@test "scratch-repo: .standards is a repository of its own, at the checked-out commit's files" {
  local repo
  repo="$(scratch)"
  cmp <(git -C .standards show HEAD:templates/Makefile.canonical) "${repo}/.standards/templates/Makefile.canonical"
  [ "$(git -C "${repo}/.standards" rev-list --count HEAD)" -eq 1 ]
  [ -z "$(git -C "${repo}/.standards" status --porcelain)" ]
}

@test "build-native-scratch: outside GitHub Actions it skips and runs nothing" {
  PATH="${BATS_TEST_TMPDIR}/bin:${PATH}" run bash scripts/test/build-native-scratch.sh darwin arm64
  [ "${status}" -eq 0 ]
  [[ "${output}" == *skipped* ]] || false
  [ ! -s "${CALLS}" ]
}

@test "build-native-scratch: an unknown platform fails and runs nothing" {
  GITHUB_ACTIONS=true PATH="${BATS_TEST_TMPDIR}/bin:${PATH}" run bash scripts/test/build-native-scratch.sh linux arm64
  [ "${status}" -ne 0 ]
  [ ! -s "${CALLS}" ]
}

@test "test-native-scratch: outside GitHub Actions it skips and runs nothing" {
  PATH="${BATS_TEST_TMPDIR}/bin:${PATH}" run bash scripts/test/test-native-scratch.sh target/native/darwin-arm64
  [ "${status}" -eq 0 ]
  [[ "${output}" == *skipped* ]] || false
  [ ! -s "${CALLS}" ]
}

@test "test-native-scratch: a directory with no native build fails and runs nothing" {
  GITHUB_ACTIONS=true PATH="${BATS_TEST_TMPDIR}/bin:${PATH}" \
    run bash scripts/test/test-native-scratch.sh "${BATS_TEST_TMPDIR}/none"
  [ "${status}" -ne 0 ]
  [ ! -s "${CALLS}" ]
}
