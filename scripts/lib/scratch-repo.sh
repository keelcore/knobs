#!/usr/bin/env bash
# scripts/lib/scratch-repo.sh
# A new repository for CI-only checks: this commit's tracked files, with .standards as a repository of its own
# holding its checked-out commit's files, each committed once, in a new temporary directory. Nothing untracked or
# built (target/, dist/) comes along. knobs-owned, never canonical. Source this file.

declare -ra SCRATCH_GIT=(git -c user.name=knobs-scratch -c user.email=knobs-scratch@invalid -c commit.gpgsign=false
  -c advice.addEmbeddedRepo=false)

# scratch_repo: makes the new repository from the repository root and prints its path.
function scratch_repo() {
  local dir
  dir="$(mktemp -d)"
  git archive HEAD | tar -x -C "${dir}"
  mkdir -p "${dir}/.standards"
  git -C .standards archive HEAD | tar -x -C "${dir}/.standards"
  scratch_commit "${dir}/.standards"
  scratch_commit "${dir}"
  printf '%s' "${dir}"
}

# scratch_commit DIR: DIR becomes a repository with one commit of everything in it.
function scratch_commit() {
  "${SCRATCH_GIT[@]}" -C "${1}" init -q
  "${SCRATCH_GIT[@]}" -C "${1}" add -A
  "${SCRATCH_GIT[@]}" -C "${1}" commit -qm 'scratch: this commit, for a CI-only check'
}
