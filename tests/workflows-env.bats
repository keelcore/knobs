#!/usr/bin/env bats
# tests/workflows-env.bats
# The workflows leave GitHub's own variables alone and hand the tool image nothing by hand. GitHub ignores a
# workflow's assignment to a GITHUB_* or RUNNER_* variable it sets itself, so an env: entry naming one only
# misleads; GITHUB_TOKEN, which GitHub does not set in the environment, is the exception. scripts/dev/run.sh
# passes each name a script imports (governance/bash.md R37), so nothing names KEELCORE_DEV_ENV.

setup() {
  ROOT="$(cd "${BATS_TEST_DIRNAME}/.." && pwd -P)"
  cd "${ROOT}"
}

# env_keys FILE...: FILE:LINE: NAME for every key of every env: block (workflow, job or step).
env_keys() {
  awk '
    FNR == 1 { depth = -1 }
    {
      indent = match($0, /[^ ]/) - 1
      if (depth >= 0 && indent <= depth && $0 !~ /^[[:space:]]*(#|$)/) depth = -1
      if (depth >= 0 && indent > depth && match($0, /^[[:space:]]*[A-Za-z_][A-Za-z0-9_]*:/)) {
        key = substr($0, indent + 1, RLENGTH - indent - 1)
        printf "%s:%d: %s\n", FILENAME, FNR, key
      }
      if ($0 ~ /^[[:space:]]*env:[[:space:]]*$/) depth = indent
    }
  ' "$@"
}

@test "workflows-env: env_keys finds the keys of a step's env block" {
  printf 'jobs:\n  a:\n    steps:\n      - run: make x\n        env:\n          FOO: 1\n          GITHUB_REF_NAME: v1\n      - run: make y\n' \
    > "${BATS_TEST_TMPDIR}/w.yml"
  run env_keys "${BATS_TEST_TMPDIR}/w.yml"
  [ "${status}" -eq 0 ]
  [[ "${output}" == *'w.yml:6: FOO'* ]] || false
  [[ "${output}" == *'w.yml:7: GITHUB_REF_NAME'* ]] || false
  [ "$(printf '%s\n' "${output}" | wc -l)" -eq 2 ]
}

@test "workflows-env: no workflow assigns a GITHUB_* or RUNNER_* variable other than GITHUB_TOKEN" {
  run env_keys .github/workflows/*.yml
  [ "${status}" -eq 0 ]
  local -r assigned="$(printf '%s\n' "${output}" | grep -E ': (GITHUB|RUNNER)_[A-Z0-9_]+$' | grep -v ': GITHUB_TOKEN$')"
  printf '%s\n' "${assigned}"
  [ -z "${assigned}" ]
}

@test "workflows-env: nothing knobs owns names KEELCORE_DEV_ENV" {
  run grep -rn KEELCORE_DEV_ENV .github scripts tests/fixtures Makefile
  printf '%s\n' "${output}"
  [ "${status}" -eq 1 ]
}
