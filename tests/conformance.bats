#!/usr/bin/env bats
# tests/conformance.bats
# Runs every case in spec/cases/ against one implementation's conformance harness: KNOBS_IMPL names
# the implementation (go, rust, ts, python, c), KNOBS_CONFORM the harness (default
# target/conform/knobs-conform-<impl>). Each case directory is copied to a scratch directory, its
# modes applied and {schema} replaced by spec/schema/; stdout is compared as JSON with the case's
# expect, or with expect_error and exit status 2 (spec/README.md, Cases).

ROOT="$(cd "${BATS_TEST_DIRNAME}/.." && pwd -P)"
IMPL="${KNOBS_IMPL:-go}"
CONFORM="${KNOBS_CONFORM:-${ROOT}/target/conform/knobs-conform-${IMPL}}"

setup_file() {
  if [ ! -x "${CONFORM}" ]; then
    printf 'conformance harness for %s not built: %s\n' "${IMPL}" "${CONFORM}" >&3
    return 1
  fi
}

# run_case NAME: one conformance case.
run_case() {
  local -r name="${1}"
  local -r dir="${BATS_TEST_TMPDIR}/${name}"
  cp -R "${ROOT}/spec/cases/${name}" "${dir}"
  apply_modes "${dir}"
  local args=()
  while IFS= read -r arg; do
    args+=("${arg}")
  done < <(jq -r --arg schema "${ROOT}/spec/schema" '.args[] | gsub("\\{schema\\}"; $schema)' "${dir}/case.json")
  cd "${dir}"
  run --separate-stderr "${CONFORM}" "${args[@]}" "$(jq -r .command case.json)"
  if jq -e 'has("expect_error")' case.json > /dev/null; then
    expect_json "${status}" 2 '.expect_error'
  else
    expect_json "${status}" 0 '.expect'
  fi
}

# apply_modes DIR: each file named in the case's modes takes its mode.
apply_modes() {
  local -r dir="${1}"
  local file mode
  while IFS=$'\t' read -r file mode; do
    chmod "${mode}" "${dir}/${file}"
  done < <(jq -r '(.modes // {}) | to_entries[] | "\(.key)\t\(.value)"' "${dir}/case.json")
}

# expect_json STATUS WANT FILTER: the run exited WANT and its stdout equals the case's FILTER as JSON.
expect_json() {
  local -r got="${1}" want="${2}" filter="${3}"
  if [ "${got}" -ne "${want}" ]; then
    printf 'exit status %s, want %s\noutput: %s\n' "${got}" "${want}" "${output}"
    return 1
  fi
  if ! jq -e --argjson have "${output}" "${filter} == \$have" case.json > /dev/null; then
    printf 'output differs\nwant: %s\nhave: %s\n' "$(jq -cS "${filter}" case.json)" "$(jq -cS . <<< "${output}")"
    return 1
  fi
}

for case_dir in "${ROOT}"/spec/cases/*/; do
  case_name="$(basename "${case_dir}")"
  bats_test_function --description "${IMPL}: ${case_name}" -- run_case "${case_name}"
done
