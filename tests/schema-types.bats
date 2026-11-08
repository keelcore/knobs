#!/usr/bin/env bats
# tests/schema-types.bats
# The type-match audit (scripts/test/check-schema-types.sh): a binding's type table matches the
# schemas' table, key for key; a missing key, an extra key, a different type or a different
# secret marking fails, naming the key.

setup() {
  ROOT="$(cd "${BATS_TEST_DIRNAME}/.." && pwd -P)"
  CHECK="${ROOT}/scripts/test/check-schema-types.sh"
  SCHEMAS=("${ROOT}/spec/schema/http.json" "${ROOT}/spec/schema/db.json")
  TABLE="${BATS_TEST_TMPDIR}/table.json"
  jq -n '[
    {key: "db.password", type: "string", secret: true}, {key: "db.pool", type: "integer", secret: false},
    {key: "db.url", type: "string", secret: false}, {key: "http.debug", type: "boolean", secret: false},
    {key: "http.host", type: "string", secret: false}, {key: "http.mode", type: "string", secret: false},
    {key: "http.port", type: "integer", secret: false}, {key: "http.ratio", type: "number", secret: false},
    {key: "http.read_timeout", type: "duration", secret: false}]' > "${TABLE}"
}

edit_table() {
  jq "${1}" "${TABLE}" > "${TABLE}.new"
  mv "${TABLE}.new" "${TABLE}"
}

@test "schema-types: a matching table passes" {
  run bash "${CHECK}" "${TABLE}" "${SCHEMAS[@]}"
  [ "${status}" -eq 0 ]
}

@test "schema-types: a key the binding lacks fails, naming it" {
  edit_table 'map(select(.key != "http.port"))'
  run bash "${CHECK}" "${TABLE}" "${SCHEMAS[@]}"
  [ "${status}" -ne 0 ]
  [[ "${output}" == *"http.port"* ]] || false
}

@test "schema-types: a key the schemas lack fails, naming it" {
  edit_table '. + [{key: "http.extra", type: "string", secret: false}]'
  run bash "${CHECK}" "${TABLE}" "${SCHEMAS[@]}"
  [ "${status}" -ne 0 ]
  [[ "${output}" == *"http.extra"* ]] || false
}

@test "schema-types: a different type fails" {
  edit_table 'map(if .key == "http.ratio" then .type = "integer" else . end)'
  run bash "${CHECK}" "${TABLE}" "${SCHEMAS[@]}"
  [ "${status}" -ne 0 ]
  [[ "${output}" == *"http.ratio"* ]] || false
}

@test "schema-types: a different secret marking fails" {
  edit_table 'map(if .key == "db.password" then .secret = false else . end)'
  run bash "${CHECK}" "${TABLE}" "${SCHEMAS[@]}"
  [ "${status}" -ne 0 ]
  [[ "${output}" == *"db.password"* ]] || false
}

@test "schema-types: the Go bindings match the test schemas" {
  local -r bin="${ROOT}/target/conform/knobs-conform-go"
  if [ ! -x "${bin}" ]; then
    skip "Built artifact not found at ${bin}; build first (make build-conform)"
  fi
  "${bin}" types > "${TABLE}"
  run bash "${CHECK}" "${TABLE}" "${SCHEMAS[@]}"
  [ "${status}" -eq 0 ]
}
