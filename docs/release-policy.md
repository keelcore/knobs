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
