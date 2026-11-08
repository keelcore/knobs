# Contributing to knobs

Thank you for your interest in contributing. This document covers the path from idea to merged pull request.

## Before You Start

- Check [open issues](https://github.com/keelcore/knobs/issues) and
  [open pull requests](https://github.com/keelcore/knobs/pulls).
- For a behavior change, open an issue first: behavior is defined by the conformance cases in `spec/`, and a change
  there reaches all five implementations.
- Small fixes (typos, test gaps, documentation) can go straight to a pull request; [LAZY.md](LAZY.md) shows the fork
  and pull request flow.

## Development Setup

The host needs `make`, `bash`, `git` and a container runtime (Apple `container` on macOS, docker on Linux). Every
build, lint and test tool runs in the dev image.

```bash
git clone https://github.com/keelcore/knobs.git
cd knobs
git submodule update --init --recursive   # .standards/
make install-hooks                        # pre-commit hook
make test
```

## Making Changes

All changes follow the [keelcore/standards](https://github.com/keelcore/standards) governance in `.standards/governance/`:
[coding](../.standards/governance/coding.md), [CI](../.standards/governance/ci.md),
[testing](../.standards/governance/testing.md) and [bash](../.standards/governance/bash.md).

- **Test first.** Add or change the conformance case in `spec/cases/` and the unit tests, show them failing, then
  implement until they pass.
- **Every implementation.** A conformance case passes in every language before the change merges.
- **Coverage.** Per-file coverage never decreases in any language; new files reach 95%.
- **No suppressions.** Lint findings are fixed at the source or the rule is turned off in the linter's configuration,
  with the reason; suppression comments in source are rejected.
- **Changelog.** Every change adds its entry under `## [Unreleased]` in `CHANGELOG.md`.

## Submitting a Pull Request

- Sign off every commit (`git commit -s`): the DCO check requires the `Signed-off-by` trailer.
- Sign commits; the `main` ruleset requires signed commits.
- Keep the pull request to one concern, describe what it changes and why, and link its issue.
- CI must be green: lint, unit tests and conformance for every language, coverage, audit, secret scan.

## Security Vulnerabilities

Report them privately; see [SECURITY.md](../SECURITY.md).

## Code of Conduct

Participation follows the [Code of Conduct](CODE_OF_CONDUCT.md).
