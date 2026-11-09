#!/usr/bin/env bats
# tests/apply-rulesets.bats
# scripts/admin/apply-rulesets.sh: its environment inputs. GH_TOKEN authenticates the API calls and is required
# by apply; GITHUB_REPOSITORY (default keelcore/knobs) names the repository; RULESETS_LIVE_DIR serves the live
# rulesets from files. curl is a stub that records its arguments and serves a ruleset list and one ruleset, so
# nothing reaches the network.

setup() {
  ROOT="$(cd "${BATS_TEST_DIRNAME}/.." && pwd -P)"
  SCRIPT="${ROOT}/scripts/admin/apply-rulesets.sh"
  WORK="${BATS_TEST_TMPDIR}/work"
  SERVED="${BATS_TEST_TMPDIR}/served"
  CALLS="${BATS_TEST_TMPDIR}/calls"
  mkdir -p "${WORK}/bin" "${WORK}/.github/rulesets" "${WORK}/.github/workflows" "${SERVED}"
  : > "${CALLS}"
  printf 'jobs:\n  lint:\n    name: Lint\n' > "${WORK}/.github/workflows/ci.yml"
  printf '%s\n' '{"name": "main", "target": "branch", "enforcement": "active", "conditions": {},' \
    '"bypass_actors": [], "rules": [{"type": "required_status_checks",' \
    '"parameters": {"required_status_checks": [{"context": "Lint"}]}}]}' > "${WORK}/.github/rulesets/main.json"
  cp "${WORK}/.github/rulesets/main.json" "${SERVED}/main.json"
  printf '[{"name": "main", "id": 7}]\n' > "${SERVED}/list.json"
  {
    printf '#!/usr/bin/env bash\n'
    printf 'printf "%%s\\n" "$*" >> "%s"\n' "${CALLS}"
    printf 'case "$*" in\n'
    printf '  */rulesets) cat "%s/list.json" ;;\n' "${SERVED}"
    printf '  *) cat "%s/main.json" ;;\n' "${SERVED}"
    printf 'esac\n'
  } > "${WORK}/bin/curl"
  chmod +x "${WORK}/bin/curl"
  export PATH="${WORK}/bin:${PATH}"
  unset GH_TOKEN GITHUB_REPOSITORY RULESETS_LIVE_DIR
  cd "${WORK}"
}

@test "apply-rulesets: without GITHUB_REPOSITORY the live rulesets are keelcore/knobs'" {
  run bash "${SCRIPT}" check
  [ "${status}" -eq 0 ]
  grep -q 'https://api.github.com/repos/keelcore/knobs/rulesets$' "${CALLS}"
  grep -q 'https://api.github.com/repos/keelcore/knobs/rulesets/7$' "${CALLS}"
}

@test "apply-rulesets: GITHUB_REPOSITORY names the repository" {
  GITHUB_REPOSITORY=example/repo run bash "${SCRIPT}" check
  [ "${status}" -eq 0 ]
  grep -q 'https://api.github.com/repos/example/repo/rulesets$' "${CALLS}"
  ! grep -q 'keelcore/knobs' "${CALLS}" || false
}

@test "apply-rulesets: without GH_TOKEN no API call carries credentials" {
  run bash "${SCRIPT}" check
  [ "${status}" -eq 0 ]
  ! grep -q 'Authorization' "${CALLS}" || false
}

@test "apply-rulesets: GH_TOKEN authenticates every API call and is not printed" {
  GH_TOKEN=t0ken run bash "${SCRIPT}" check
  [ "${status}" -eq 0 ]
  [ "$(grep -c 'Authorization: Bearer t0ken' "${CALLS}")" -eq 2 ]
  [[ "${output}" != *t0ken* ]] || false
}

@test "apply-rulesets: apply without GH_TOKEN fails, naming it, and calls nothing" {
  run bash "${SCRIPT}" apply
  [ "${status}" -ne 0 ]
  [[ "${output}" == *GH_TOKEN* ]] || false
  [ ! -s "${CALLS}" ]
}

@test "apply-rulesets: without RULESETS_LIVE_DIR the live rulesets come from the API" {
  run bash "${SCRIPT}" check
  [ "${status}" -eq 0 ]
  [ -s "${CALLS}" ]
}

@test "apply-rulesets: RULESETS_LIVE_DIR serves the live rulesets from files, with no API call" {
  RULESETS_LIVE_DIR="${SERVED}" run bash "${SCRIPT}" check
  [ "${status}" -eq 0 ]
  [ ! -s "${CALLS}" ]
}

@test "apply-rulesets: a live ruleset that differs from the committed one fails check" {
  jq '.enforcement = "disabled"' "${SERVED}/main.json" > "${BATS_TEST_TMPDIR}/differs.json"
  mv "${BATS_TEST_TMPDIR}/differs.json" "${SERVED}/main.json"
  RULESETS_LIVE_DIR="${SERVED}" run bash "${SCRIPT}" check
  [ "${status}" -ne 0 ]
}
