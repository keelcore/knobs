#!/usr/bin/env bats
# tests/pull-release-module.bats
# scripts/release/pull-release-module.sh: fetches the release's SHA256SUMS and module zip into dist/,
# from the repository go.mod names, and fails unless the zip matches its SHA256SUMS line. curl is a stub
# that serves files from a scratch "release" directory, so nothing reaches the network.

setup() {
  ROOT="$(cd "${BATS_TEST_DIRNAME}/.." && pwd -P)"
  SCRIPT="${ROOT}/scripts/release/pull-release-module.sh"
  WORK="${BATS_TEST_TMPDIR}/work"
  SERVED="${BATS_TEST_TMPDIR}/served"
  CALLS="${BATS_TEST_TMPDIR}/calls"
  mkdir -p "${WORK}/bin" "${SERVED}"
  : > "${CALLS}"
  printf 'module github.com/example/lib\n\ngo 1.27\n' > "${WORK}/go.mod"
  printf 'module zip\n' > "${SERVED}/lib-v1.2.3.zip"
  (cd "${SERVED}" && sha256sum lib-v1.2.3.zip > SHA256SUMS && printf 'abc  other.txt\n' >> SHA256SUMS)
  cp "${ROOT}/tests/fixtures/stub-curl.sh" "${WORK}/bin/curl"
  HEADERS="${BATS_TEST_TMPDIR}/headers"
  : > "${HEADERS}"
  export STUB_CALLS="${CALLS}" STUB_SERVED="${SERVED}" STUB_HEADERS="${HEADERS}"
  export PATH="${WORK}/bin:${PATH}"
  unset GITHUB_REF_NAME INPUT_TAG GITHUB_TOKEN
  cd "${WORK}"
}

@test "pull-release-module: the zip and SHA256SUMS land in dist/ from the go.mod repository's release" {
  GITHUB_REF_NAME=v1.2.3 run bash "${SCRIPT}"
  [ "${status}" -eq 0 ]
  cmp "${SERVED}/lib-v1.2.3.zip" dist/lib-v1.2.3.zip
  grep -qx 'https://github.com/example/lib/releases/download/v1.2.3/SHA256SUMS' "${CALLS}"
  grep -qx 'https://github.com/example/lib/releases/download/v1.2.3/lib-v1.2.3.zip' "${CALLS}"
}

@test "pull-release-module: a zip that differs from its SHA256SUMS line fails" {
  printf 'tampered\n' > "${SERVED}/lib-v1.2.3.zip"
  GITHUB_REF_NAME=v1.2.3 run bash "${SCRIPT}"
  [ "${status}" -ne 0 ]
}

@test "pull-release-module: a release whose SHA256SUMS names no module zip fails" {
  printf 'abc  other.txt\n' > "${SERVED}/SHA256SUMS"
  INPUT_TAG=v1.2.3 run bash "${SCRIPT}"
  [ "${status}" -ne 0 ]
}

@test "pull-release-module: without a tag it fails and fetches nothing" {
  run bash "${SCRIPT}"
  [ "${status}" -ne 0 ]
  [ ! -s "${CALLS}" ]
}

@test "pull-release-module: INPUT_TAG wins over GITHUB_REF_NAME (a manual run's ref is its branch)" {
  GITHUB_REF_NAME=main INPUT_TAG=v1.2.3 run bash "${SCRIPT}"
  [ "${status}" -eq 0 ]
  grep -qx 'https://github.com/example/lib/releases/download/v1.2.3/lib-v1.2.3.zip' "${CALLS}"
}

@test "pull-release-module: with GITHUB_TOKEN every download is authenticated" {
  GITHUB_TOKEN=t0ken GITHUB_REF_NAME=v1.2.3 run bash "${SCRIPT}"
  [ "${status}" -eq 0 ]
  [ "$(grep -cx 'Authorization: Bearer t0ken' "${HEADERS}")" -eq 2 ]
  [[ "${output}" != *t0ken* ]] || false
}

@test "pull-release-module: without GITHUB_TOKEN no download carries credentials" {
  GITHUB_REF_NAME=v1.2.3 run bash "${SCRIPT}"
  [ "${status}" -eq 0 ]
  ! grep -q '^Authorization' "${HEADERS}" || false
}
