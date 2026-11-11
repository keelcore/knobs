# Testing

The standards in [`.standards/governance/testing.md`](../.standards/governance/testing.md) apply in full; this page
lists knobs' suites.

| Suite | Target | What it covers |
| --- | --- | --- |
| Unit | `make test-unit` | every Go package, in process; every conformance case through the harness |
| Coverage | `make test-coverage`, `make check-coverage-regression` | per-file line coverage against `tests/coverage-baseline.lcov`; new files at 95% or more |
| Integration | `make test-integration` | the conformance suite against the built harness, then every other BATS suite |
| Fuzz | `make test-fuzz` | text conversion, file layers, connection values |
| Types | `make check-schema-types` | the Go bindings' types against the test schemas |
| Native | `make build-native`, `make test-native`, `make test-native-integrity` | the unit tests on macOS and Windows; the conformance suite on macOS; a harness smoke test on Windows |

## Unit tests

- All unit tests for `{foo}.go` are in `{foo}_test.go` beside it. `make lint` checks it
  (`scripts/lint/lint-test-layout.local.sh`): a test file with no source, or a source with no test file, fails;
  `cmd/*/main.go`, `doc.go` and the helpers-only files `config/test-layout.local.txt` lists are exempt.
- `doc_test.go` (the contract `doc.go` describes) and `internal/conform/conform_test.go` run every `spec/cases/` case
  in process, so each package is measured against the whole contract.
- `connection_test.go` opens 12800 connections from 64 goroutines against one process configuration (run under
  `-race`), and measures the heap that 100, 1000 and 10000 live connections hold: under 512 bytes each, and no growth
  per connection with the count.
- `knobstest/` tests the committed-schema and single-major checks.

## BATS suites

| File | Covers |
| --- | --- |
| `tests/conformance.bats` | every conformance case against `knobs-conform` |
| `tests/integrity.bats` | the built harness rejects bad invocations cleanly |
| `tests/schema-types.bats` | the type-match audit |
| `tests/publish-go-module.bats` | the Go module publisher, with stubbed network tools |
| `tests/pull-release-module.bats` | the release module fetch and checksum check, with a stubbed `curl` |
| `tests/canonical/smoke.bats` | the harness itself |

Every target runs in the dev image (`scripts/dev/run.sh`). The native jobs run on macOS and Windows runners what
the dev image cross-built (`target/native/<os>-<arch>/`: the Go test binaries, bats, `knobs-conform`); the runners
install nothing.
