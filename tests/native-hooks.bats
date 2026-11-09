#!/usr/bin/env bats
# tests/native-hooks.bats
# knobs' hooks of the canonical native targets. build-native.local.sh cross-builds knobs-conform into OUT/bin/ and
# leaves the canonical OUT/tests.tsv as written. test-native-integrity.local.sh runs with the native build's tools
# first on PATH and NATIVE_DIR set (scripts/test/test-native-integrity.sh): bats and the harness come from PATH,
# nothing is looked up under target/native/. go, bats and the harness are stubs that record their calls.

setup() {
  ROOT="$(cd "${BATS_TEST_DIRNAME}/.." && pwd -P)"
  WORK="${BATS_TEST_TMPDIR}/work"
  STUBS="${BATS_TEST_TMPDIR}/stubs"
  HARNESS="${BATS_TEST_TMPDIR}/native/bin"
  CALLS="${BATS_TEST_TMPDIR}/calls"
  mkdir -p "${WORK}" "${STUBS}" "${HARNESS}"
  : > "${CALLS}"
  # go build ... -o FILE ...: records the call and creates FILE.
  printf '%s\n' '#!/usr/bin/env bash' "printf 'go|%s\\n' \"\$*\" >> '${CALLS}'" \
    'while [ "$#" -gt 0 ]; do [ "$1" != -o ] || : > "$2"; shift; done' > "${STUBS}/go"
  printf '%s\n' '#!/usr/bin/env bash' \
    "printf 'bats|KNOBS_IMPL=%s KNOBS_CONFORM=%s %s\\n' \"\${KNOBS_IMPL:-}\" \"\${KNOBS_CONFORM:-}\" \"\$*\" >> '${CALLS}'" \
    > "${STUBS}/bats"
  chmod +x "${STUBS}/go" "${STUBS}/bats"
  unset NATIVE_DIR
  cd "${WORK}"
}

# harness NAME: a stub harness NAME in the native build's bin/, which is on PATH.
harness() {
  printf '#!/usr/bin/env bash\n' > "${HARNESS}/${1}"
  chmod +x "${HARNESS}/${1}"
}

@test "build-native.local.sh: knobs-conform is cross-built into OUT/bin/ for the platform" {
  PATH="${STUBS}:${PATH}" run bash "${ROOT}/scripts/test/build-native.local.sh" windows amd64 out .exe
  [ "${status}" -eq 0 ]
  [ -f out/bin/knobs-conform.exe ]
  grep -q '^go|build .*-o out/bin/knobs-conform.exe ./cmd/knobs-conform$' "${CALLS}"
}

@test "build-native.local.sh: the canonical tests.tsv is left as written" {
  mkdir -p out
  printf '%s\t%s\n' "${PWD}" 'tests/_root' 'internal/conform' 'tests/internal_conform' > out/tests.tsv
  cp out/tests.tsv "${BATS_TEST_TMPDIR}/tests.tsv.before"
  PATH="${STUBS}:${PATH}" run bash "${ROOT}/scripts/test/build-native.local.sh" darwin arm64 out
  [ "${status}" -eq 0 ]
  [ -f out/bin/knobs-conform ]
  cmp "${BATS_TEST_TMPDIR}/tests.tsv.before" out/tests.tsv
}

@test "test-native-integrity.local.sh: on a POSIX host it runs both suites with bats and the harness from PATH" {
  harness knobs-conform
  NATIVE_DIR="${BATS_TEST_TMPDIR}/native" PATH="${STUBS}:${HARNESS}:${PATH}" \
    run bash "${ROOT}/scripts/test/test-native-integrity.local.sh"
  [ "${status}" -eq 0 ]
  grep -qx "bats|KNOBS_IMPL=go KNOBS_CONFORM=${HARNESS}/knobs-conform tests/conformance.bats" "${CALLS}"
  grep -qx "bats|KNOBS_IMPL= KNOBS_CONFORM=${HARNESS}/knobs-conform tests/integrity.bats" "${CALLS}"
  [[ "${output}" == *"Native harness: ${HARNESS}/knobs-conform"* ]] || false
}

@test "test-native-integrity.local.sh: with knobs-conform.exe it runs the integrity suite only" {
  harness knobs-conform.exe
  NATIVE_DIR="${BATS_TEST_TMPDIR}/native" PATH="${STUBS}:${HARNESS}:${PATH}" \
    run bash "${ROOT}/scripts/test/test-native-integrity.local.sh"
  [ "${status}" -eq 0 ]
  grep -qx "bats|KNOBS_IMPL= KNOBS_CONFORM=${HARNESS}/knobs-conform.exe tests/integrity.bats" "${CALLS}"
  [ "$(grep -c '^bats|' "${CALLS}")" -eq 1 ]
}

@test "test-native-integrity.local.sh: without NATIVE_DIR it fails, naming it, and runs nothing" {
  harness knobs-conform
  PATH="${STUBS}:${HARNESS}:${PATH}" run bash "${ROOT}/scripts/test/test-native-integrity.local.sh"
  [ "${status}" -ne 0 ]
  [[ "${output}" == *NATIVE_DIR* ]] || false
  [ ! -s "${CALLS}" ]
}

@test "test-native-integrity.local.sh: with no knobs-conform on PATH it fails, naming it, and runs nothing" {
  NATIVE_DIR="${BATS_TEST_TMPDIR}/native" PATH="${STUBS}:${PATH}" \
    run bash "${ROOT}/scripts/test/test-native-integrity.local.sh"
  [ "${status}" -ne 0 ]
  [[ "${output}" == *knobs-conform* ]] || false
  [ ! -s "${CALLS}" ]
}
