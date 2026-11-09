#!/usr/bin/env bats
# tests/publish-go-module.bats
# scripts/release/publish-go-module.sh: the module path comes from go.mod; the Go checksum database and
# proxy are asked for that module at the tag; without a tag it fails. curl and go are stubs that
# record their calls, so nothing reaches the network.

setup() {
  ROOT="$(cd "${BATS_TEST_DIRNAME}/.." && pwd -P)"
  SCRIPT="${ROOT}/scripts/release/publish-go-module.sh"
  WORK="${BATS_TEST_TMPDIR}/work"
  CALLS="${BATS_TEST_TMPDIR}/calls"
  mkdir -p "${WORK}/bin"
  : > "${CALLS}"
  printf 'module example.com/some/lib\n\ngo 1.27\n' > "${WORK}/go.mod"
  for tool in curl go; do
    printf '#!/usr/bin/env bash\nprintf "%s|%%s\\n" "$*" >> "%s"\n' "${tool}" "${CALLS}" > "${WORK}/bin/${tool}"
    chmod +x "${WORK}/bin/${tool}"
  done
  export PATH="${WORK}/bin:${PATH}"
  unset GITHUB_REF_NAME GITHUB_REF_TYPE INPUT_TAG
  cd "${WORK}"
}

@test "publish-go-module: the module path comes from go.mod and the proxy is asked for the tag" {
  GITHUB_REF_TYPE=tag GITHUB_REF_NAME=v1.2.3 run bash "${SCRIPT}"
  [ "${status}" -eq 0 ]
  grep -q 'curl|.*https://sum.golang.org/lookup/example.com/some/lib@v1.2.3' "${CALLS}"
  grep -q 'curl|.*https://proxy.golang.org/example.com/some/lib/@v/v1.2.3.info' "${CALLS}"
  grep -q '^go|build ./...$' "${CALLS}"
}

@test "publish-go-module: INPUT_TAG names the tag on a manual run" {
  INPUT_TAG=v0.1.0 run bash "${SCRIPT}"
  [ "${status}" -eq 0 ]
  grep -q 'proxy.golang.org/example.com/some/lib/@v/v0.1.0.info' "${CALLS}"
}

@test "publish-go-module: without a tag it fails and asks nothing" {
  run bash "${SCRIPT}"
  [ "${status}" -ne 0 ]
  [ ! -s "${CALLS}" ]
}

@test "publish-go-module: without go.mod it fails" {
  rm go.mod
  GITHUB_REF_TYPE=tag GITHUB_REF_NAME=v1.2.3 run bash "${SCRIPT}"
  [ "${status}" -ne 0 ]
  ! grep -q '^curl|' "${CALLS}" || false
}

@test "publish-go-module: INPUT_TAG wins over GITHUB_REF_NAME (a manual run's ref is its branch)" {
  GITHUB_REF_NAME=main INPUT_TAG=v0.1.0 run bash "${SCRIPT}"
  [ "${status}" -eq 0 ]
  grep -q 'proxy.golang.org/example.com/some/lib/@v/v0.1.0.info' "${CALLS}"
}

@test "publish-go-module: a branch run's GITHUB_REF_NAME is never the tag" {
  GITHUB_REF_TYPE=branch GITHUB_REF_NAME=main run bash "${SCRIPT}"
  [ "${status}" -ne 0 ]
  [ ! -s "${CALLS}" ]
}
