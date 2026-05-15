#!/usr/bin/env bash
set -euo pipefail

verify_production_git_state() {
  local action="${1:-production}"
  local branch="${2:-master}"
  local remote="${3:-origin}"

  command -v git >/dev/null 2>&1 || { echo "error: git is required" >&2; return 1; }
  command -v gh >/dev/null 2>&1 || { echo "error: gh is required to verify open PR state" >&2; return 1; }

  if [[ "${branch}" != "master" ]]; then
    echo "error: ${action} is allowed only from branch 'master' (configured branch: '${branch}')" >&2
    return 1
  fi

  local current_branch
  current_branch="$(git branch --show-current)"
  if [[ "${current_branch}" != "${branch}" ]]; then
    echo "error: ${action} is allowed only from branch '${branch}' (current: '${current_branch:-detached HEAD}')" >&2
    return 1
  fi

  if [[ -n "$(git status --porcelain)" ]]; then
    echo "error: ${action} requires a clean worktree on ${branch}; commit or remove local changes first" >&2
    return 1
  fi

  timeout -k 30s -s SIGKILL 30s git fetch --prune "${remote}" "+refs/heads/${branch}:refs/remotes/${remote}/${branch}"

  local head_sha
  local remote_sha
  head_sha="$(git rev-parse HEAD)"
  remote_sha="$(git rev-parse "refs/remotes/${remote}/${branch}")"
  if [[ "${head_sha}" != "${remote_sha}" ]]; then
    echo "error: local ${branch} (${head_sha}) does not match ${remote}/${branch} (${remote_sha})" >&2
    return 1
  fi

  local open_pr_count
  open_pr_count="$(gh pr list --state open --limit 1000 --json number --jq 'length')"
  if [[ "${open_pr_count}" != "0" ]]; then
    echo "error: ${action} requires zero open PRs; found ${open_pr_count}" >&2
    gh pr list --state open --limit 1000 --json number,title,headRefName,baseRefName --jq '.[] | "  #\(.number) \(.headRefName) -> \(.baseRefName): \(.title)"' >&2
    return 1
  fi

  echo "==> [${action}] Verified production git state: branch=${branch} remote=${remote}/${branch} commit=${head_sha} open_prs=0"
}

if [[ "${BASH_SOURCE[0]}" == "$0" ]]; then
  verify_production_git_state "${1:-production}" "${2:-master}" "${3:-origin}"
fi
