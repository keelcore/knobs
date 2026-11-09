# Conformance

Every implementation of knobs resolves the same inputs to the same result. The contract is
[spec/README.md](../spec/README.md); each behavior it states has a case in `spec/cases/`.

## The suite

`tests/conformance.bats` runs every case against one implementation's harness, named by `KNOBS_IMPL` (`go` today):

```sh
make build-conform          # target/conform/knobs-conform-<impl> for every implementation in the tree
make test-integration       # the conformance suite once per built harness, then the other BATS suites
```

Each case is copied to a scratch directory, its file modes applied and `{schema}` replaced by `spec/schema/`; the
harness runs there with the case's arguments and command, and its stdout is compared with the case's `expect` (or
`expect_error`, with exit status 2) as JSON. The Go unit tests also run every case in process (`doc_test.go`,
`internal/conform`), so the engine is measured against the whole contract.

## Adding behavior

1. Add the case to `spec/cases/<name>/case.json`, with any files it names, and the rule to `spec/README.md`.
2. Run the suite: the case fails in every implementation.
3. Implement it in each language until every implementation passes.

## Type-match audit

Each implementation's bindings of the test schemas print their type table (`knobs-conform types`);
`make check-schema-types` (`scripts/test/check-schema-types.sh`) compares it with the table derived from
`spec/schema/`, key for key: type and secret marking.
