#!/usr/bin/env bash
# check-schema-types.sh
# The type-match audit: a binding's type table (`knobs-conform types`, JSON rows of key, type,
# secret) matches the table derived from the module schemas, key for key. A missing key, an extra
# key, a different type or a different secret marking fails, naming the key.
# Usage: check-schema-types.sh TABLE.json SCHEMA.json...
#        check-schema-types.sh            (the Go harness's table against spec/schema/*.json)

# bash configuration:
# 1) Exit script if you try to use an uninitialized variable.
set -o nounset

# 2) Exit script if a statement returns a non-true return value.
set -o errexit

# 3) Use the error status of the first failure, rather than that of the last item in a pipeline.
set -o pipefail

declare -r DEFAULT_HARNESS='target/conform/knobs-conform-go'

function main() {
  exec 5>&1
  validate_args "${@:-}"
  if [ -z "${1:-}" ]; then
    check_go_bindings
    return 0
  fi
  local -r table="${1}"
  shift
  compare "${table}" "$@"
}

function log() {
  local msg
  msg="${1:-}"
  printf '%s\n' "${msg}" | tee -a '/tmp/knobs_check_schema_types.log' >&5
}

function validate_args() {
  if [ "${#}" -eq 1 ] && [ -n "${1:-}" ]; then
    log '❌ Usage: check-schema-types.sh [TABLE.json SCHEMA.json...]'
    exit 1
  fi
}

# check_go_bindings: the Go harness's table against every test schema with a Go binding.
function check_go_bindings() {
  local table
  table="$(mktemp)"
  "${DEFAULT_HARNESS}" types > "${table}"
  compare "${table}" spec/schema/http.json spec/schema/db.json
  rm -f "${table}"
}

# compare TABLE SCHEMA...: the two tables, row for row.
function compare() {
  local -r table="${1}"
  shift
  log "🔍 Type table ${table} against ${*}"
  local diff
  diff="$(diff <(schema_table "$@") <(binding_table "${table}") || true)"
  if [ -n "${diff}" ]; then
    log '❌ The binding does not match the schemas (< schema, > binding):'
    log "${diff}"
    exit 1
  fi
  log '✅ Binding types match the schemas'
}

# schema_table SCHEMA...: one line per property, module.name<TAB>type<TAB>secret, sorted.
function schema_table() {
  jq -r '."x-knobs-module" as $m | .properties | to_entries[]
    | [ "\($m).\(.key)", (if .value."x-knobs-type" == "duration" then "duration" else .value.type end),
        (.value."x-knobs-secret" // false | tostring) ] | @tsv' "$@" | LC_ALL=C sort
}

# binding_table TABLE: the binding's rows in the same form.
function binding_table() {
  jq -r '.[] | [ .key, .type, (.secret | tostring) ] | @tsv' "${1}" | LC_ALL=C sort
}

main "${@:-}"
