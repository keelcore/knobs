# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

Breaking changes to any package's public API: major bump. New backward-compatible functionality: minor bump. Fixes:
patch bump. `scripts/release/create-release.local.sh` reads the bump from the entries below.

Security vulnerabilities are reported via [GitHub Security Advisories](https://github.com/keelcore/knobs/security/advisories).

## [Unreleased]

### Fixed

- CI passes `PR_AUTHOR` to the PR policy check (its Dependabot exemption) and `HEAD_SHA` to the coverage delta gate.
- Dependabot leaves the canonical workflows (`scorecard.yml`, `codeql.yml`) alone: their pins come with the
  `.standards` release. It covers the Go module and its vendor tree.
- Dependabot writes nothing to `main`: version updates open their pull requests against `deps-inbox`, security
  update pull requests are off and the alerts stay on; `make update-pins` takes both as candidates
  (`docs/governance.md`, Dependency updates).
- Dependabot waits 14 days after a release before proposing it, the cooldown `update-pins` applies.

### Added

- The release workflow (`release.yml`, `github-release.yml`): after CI passes on the tagged commit and a reviewer
  approves the `release` environment, the Go module is published to the checksum database and proxy
  (`make publish-go-module`, knobs's own `scripts/release/publish-go-module.sh`), then the GitHub Release carries
  the module zip, its SBOM, checksums, signatures and provenance. The canonical `.standards` package publishers are
  excluded (`.canonical-exclude`).
- `Module` and `Engine.Attach(modules...)`: each Go module gives the engine its own schema, its code values and its
  own files, so `main` only attaches modules. The lowest layer is `code`: every module's code values, then a
  `Code` layer's (harness `--code`); a schema `default` overrides it as the `schema` layer, which replaces the
  `default` layer. A key marked secret set in code fails with `secret_in_code`.
- Repository bootstrapped to keelcore standards (`.standards` v0.10.24): canonical `Makefile`, scripts, pre-commit
  hook, dev image (`containers/dev`), CI, CodeQL, DCO, Scorecard, dev-image and update-pins workflows, rulesets as
  code (`.github/rulesets/`, `make check-rulesets`), `README.md`, `SECURITY.md`, governance and contribution docs.
- `spec/`: the contract every implementation follows (`spec/README.md`): names, the schema keywords knobs enforces
  (a schema using any other fails), the eight layers and their secret rules, connection overrides, error codes, the
  conformance harness and 44 conformance cases with their test schemas.
- Go package `github.com/keelcore/knobs`: `New`, `Attach` (one schema per module, with optional module files),
  `Add` and `AddStandard` (default, global, user, secrets, env, cli, runtime), `Resolve`; typed accessors, `Bind`,
  `Dump`, `Values`, the master schema; the `Secret` type, which never prints; `Config.WithConnection`, a stateless
  per-connection overlay that holds only its own overrides. Seeded from servotrim's `pipeline/config` and
  `clients/go/knob` (Apache-2.0), whose tests are ported in `servotrim_test.go`.
- `knobs-schema-bump OLD NEW` and `SchemaBump`: keel's configuration release rule, unchanged: a removed schema field
  is major, an added field minor, no change patch, printed in the release hook's form.
- `knobs-conform`, the Go conformance harness; `make build-conform`; `tests/conformance.bats` runs every case against
  it, `tests/schema-types.bats` and `make check-schema-types` audit the Go bindings' types against the test schemas.
- Unit tests for parallel connections and for the memory 100, 1000 and 10000 live connections hold (bounded per
  connection, independent of the count); fuzz targets for text conversion, file layers and connection values.
- `config/gosec-sites.local.txt`: the two reviewed G304 sites, each reading a config path the operator names.
