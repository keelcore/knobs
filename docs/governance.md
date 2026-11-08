# Governance

knobs follows the engineering standards of [`keelcore/standards`](https://github.com/keelcore/standards), pinned as
the git submodule `.standards/` at a release tag. The standards documents in `.standards/governance/` apply in full;
this file describes how knobs consumes them.

## The submodule

- `.standards` records the exact release of the standards this tree follows. Every clone and every CI run sees the
  same text and the same canonical scripts.
- `make refresh-governance` moves `.standards` to its latest release tag and copies the canonical scripts into
  `scripts/`, byte-identical; `make verify-canonical` checks they still match.
- Canonical scripts are owned by `keelcore/standards`. knobs specializes them through `*.local.sh` hooks at the same
  path, which are owned by knobs and never overwritten.
- Scripts for knobs' other languages (Rust, TypeScript, Python, C/C++) and their publishers are owned by knobs.

## The Makefile

`Makefile` includes `.standards/templates/Makefile.canonical`, which defines every canonical target. knobs adds its
own targets below the include, each running one `scripts/**/*.sh`. Workflows run `make <target>` only, so every CI
step runs the same way on a developer machine.

## Tools

Every tool runs in the dev image (`containers/dev/Containerfile`, built `FROM` the `.standards` dev image): the host
needs `make`, `bash`, `git` and a container runtime. The pre-commit hook writes `containers/dev/image.ref`, naming
the image built from each commit.

## Dependency updates

Pins change only through `make update-pins`: it discovers every pin, takes the newest eligible release of each (no
prereleases, a 14-day cooldown), verifies each stale pin alone in a scratch worktree, then all together, and with
`APPLY=1` writes the passing pins for review and commit. `make create-release` refuses to tag while any pin is stale.

Dependabot is one of its inputs and writes nothing to `main`:

- Version updates open their pull requests against `deps-inbox` (`target-branch` in `.github/dependabot.yml`), a
  branch nothing is merged from. `update-pins` reads every open Dependabot pull request as a candidate and names it
  in its report.
  They wait out the same 14-day cooldown, so Dependabot proposes only releases `update-pins` would take.
- Security updates open no pull requests (turned off in the repository settings). Dependabot alerts stay on;
  `update-pins` reads each open alert's patched version as a candidate, so an alert makes its pin stale and holds the
  release until the pin moves.
- The canonical workflows (`scorecard.yml`, `codeql.yml`) are excluded: their pins arrive with the `.standards`
  release.

This departs from `.standards/governance/devsecops.md`, Repository Controls 5, which asks for Dependabot security
updates to be on: a known vulnerability reaches the release gate through its alert and `update-pins` instead of
through a pull request.

## Branch protection

Rulesets are code: `.github/rulesets/*.json` holds the branch and tag rulesets, `make check-rulesets` compares them
with the live repository, and `make apply-rulesets` writes them (admin token). Every required status check names a
job in `.github/workflows/`.

## AI assistants

`CLAUDE.md`, `.github/copilot-instructions.md` and `.cursor/rules/` point at the same governance documents, and
`.local-claude.md` holds knobs' own guidance.
