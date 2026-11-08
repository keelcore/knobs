# Release Policy

The standards release policy, [`.standards/governance/release-policy.md`](../.standards/governance/release-policy.md),
applies in full. This document covers knobs' customization only.

## Bump policy

`scripts/release/create-release.local.sh` reads the `CHANGELOG.md` `## [Unreleased]` entries added since the previous
tag: any `### Removed` entry → major; else any `### Added` or `### Changed` entry → minor; else patch. With no new
entries there is nothing to release, and the hook fails.

knobs' public surface is the API of five packages and the conformance contract in `spec/`. A change that breaks any
package's API, or changes the result of an existing conformance case, is recorded under `### Removed` or
`### Changed` with its effect, so the hook reports it.

## One version for every package

The tag `vX.Y.Z` is the version of every package: the Go module, the crate, the npm and PyPI packages and the C/C++
archives. Each package takes its version from the tag at build time; no version file is committed.

## Running a release

```bash
make update-pins                # report stale pins
make update-pins APPLY=1        # apply the passing ones; review, commit, push
make create-release DRY_RUN=1   # preview the version and tag message
make create-release
```

`make create-release-local PREV_TAG=vX.Y.Z` runs the bump hook alone.

## What a release publishes

knobs is a library: a release is the Go module `github.com/keelcore/knobs` at the tag. The pushed tag starts
`.github/workflows/release.yml`:

1. **CI passed:** waits for every workflow run on the tagged commit, and fails if any is red.
2. **Publish:** after a reviewer approves the `release` environment, `make publish-go-module` records the module at
   the tag in the Go checksum database and module proxy, and `make wait-for-publish ARGS=go` waits until the proxy
   serves it.
3. **GitHub Release** (`github-release.yml`): the module zip as the proxy serves it, checked against the checksum
   database; its SBOM and the SBOMs of the tool images the release was built in; `SHA256SUMS`; a cosign bundle per
   file; the notes (the tag message, the version's CHANGELOG entries, the pins). Its last step verifies every
   checksum and signature.
4. **Provenance:** `make pull-release-module` fetches the module zip from the release and checks it against
   `SHA256SUMS`, and `actions/attest-build-provenance` attests its SLSA build provenance; verify it with
   `gh attestation verify knobs-vX.Y.Z.zip -R keelcore/knobs`.

Consumers take a release with `go get github.com/keelcore/knobs@vX.Y.Z`; release hooks run the schema bump tool with
`go run github.com/keelcore/knobs/cmd/knobs-schema-bump@vX.Y.Z OLD NEW`.
