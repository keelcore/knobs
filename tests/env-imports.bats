#!/usr/bin/env bats
# tests/env-imports.bats
# knobs' own bash files import every environment variable they read per governance/bash.md R37, as checked by
# lint-bash (scripts/lib/env-imports.sh), and the check still catches an inline read in them.

setup() {
  ROOT="$(cd "${BATS_TEST_DIRNAME}/.." && pwd -P)"
  OWN=(scripts/admin/apply-rulesets.sh scripts/release/publish-go-module.sh scripts/release/pull-release-module.sh
    tests/fixtures/stub-curl.sh)
}

# check FILE...: the findings for FILE..., from the working directory. The library runs in its own bash, as
# lint-bash runs it: bats calls tests inside functions, where its `declare -r` globals would be local.
check() {
  bash -c 'source "${0}" && check_env_imports "$@"' "${ROOT}/scripts/lib/env-imports.sh" "$@"
}

@test "env-imports: knobs' own bash files conform" {
  cd "${ROOT}"
  run check "${OWN[@]}"
  [ "${status}" -eq 0 ]
  [ -z "${output}" ]
}

@test "env-imports: the old inline tag read in publish-go-module.sh is reported" {
  local -r copy="${BATS_TEST_TMPDIR}/copy"
  mkdir -p "${copy}/scripts/release" "${copy}/scripts/lib"
  cp "${ROOT}/scripts/lib/release-tag.sh" "${copy}/scripts/lib/"
  sed 's/^  tag="$(release_tag)"$/  tag="${INPUT_TAG:-${GITHUB_REF_NAME:-}}"/' \
    "${ROOT}/scripts/release/publish-go-module.sh" > "${copy}/scripts/release/publish-go-module.sh"
  ! cmp -s "${ROOT}/scripts/release/publish-go-module.sh" "${copy}/scripts/release/publish-go-module.sh" || false
  cd "${copy}"
  run check scripts/release/publish-go-module.sh
  [[ "${output}" == *'scripts/release/publish-go-module.sh:'*': INPUT_TAG: '* ]] || false
}

@test "env-imports: an inline read inside a function of pull-release-module.sh is reported" {
  local -r copy="${BATS_TEST_TMPDIR}/copy"
  mkdir -p "${copy}/scripts/release" "${copy}/scripts/lib"
  cp "${ROOT}/scripts/lib/release-tag.sh" "${copy}/scripts/lib/"
  sed 's/^function module_path() {$/&\n  [ -z "${PROBE:-}" ] || log probe/' \
    "${ROOT}/scripts/release/pull-release-module.sh" > "${copy}/scripts/release/pull-release-module.sh"
  cd "${copy}"
  run check scripts/release/pull-release-module.sh
  [[ "${output}" == *'scripts/release/pull-release-module.sh:'*': PROBE: read without an import'* ]] || false
}
