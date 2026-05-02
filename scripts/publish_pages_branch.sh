#!/usr/bin/env bash
set -euo pipefail

repo_root="$(git rev-parse --show-toplevel)"
cd "${repo_root}"

remote="${PAGES_PUBLISH_REMOTE:-origin}"
branch="${PAGES_PUBLISH_BRANCH:-gh-pages}"
source_branch="${PAGES_PUBLISH_SOURCE_BRANCH:-master}"
allow_dirty="${PAGES_PUBLISH_ALLOW_DIRTY:-0}"
force_publish="${PAGES_PUBLISH_FORCE:-0}"

if [[ -n "${source_branch}" ]]; then
  current_branch="$(git branch --show-current)"
  if [[ "${current_branch}" != "${source_branch}" ]]; then
    current_branch_label="${current_branch:-detached HEAD}"
    echo "error: refusing to publish Pages from ${current_branch_label}; expected ${source_branch}" >&2
    echo "Set PAGES_PUBLISH_SOURCE_BRANCH= to allow the current checkout explicitly." >&2
    exit 1
  fi
fi

if [[ "${allow_dirty}" != "1" ]]; then
  dirty_status="$(git status --porcelain)"
  if [[ -n "${dirty_status}" ]]; then
    echo "error: refusing to publish Pages from a dirty worktree" >&2
    printf '%s\n' "${dirty_status}" >&2
    exit 1
  fi
fi

source_commit="$(git rev-parse --verify HEAD)"
source_short="$(git rev-parse --short HEAD)"
tmp_root="${TMPDIR:-/tmp}"
tmp_root="${tmp_root%/}"
artifact_dir="$(mktemp -d "${tmp_root}/pinguin-pages-artifact.XXXXXX")"
worktree_dir="$(mktemp -d "${tmp_root}/pinguin-pages-worktree.XXXXXX")"
rmdir "${worktree_dir}"

cleanup() {
  git worktree remove --force "${worktree_dir}" >/dev/null 2>&1 || true
  rm -rf "${artifact_dir}" "${worktree_dir}"
}
trap cleanup EXIT

./scripts/build_pages_artifact.sh "${artifact_dir}"

if git ls-remote --exit-code --heads "${remote}" "${branch}" >/dev/null 2>&1; then
  git fetch "${remote}" "${branch}"
  git worktree add --detach "${worktree_dir}" FETCH_HEAD
else
  git worktree add --detach "${worktree_dir}" "${source_commit}"
  git -C "${worktree_dir}" checkout --orphan "pages-publish-${source_short}-$$"
  git -C "${worktree_dir}" rm -r --ignore-unmatch . >/dev/null 2>&1 || true
fi

git -C "${worktree_dir}" rm -r --ignore-unmatch . >/dev/null 2>&1 || true
find "${worktree_dir}" -mindepth 1 -maxdepth 1 ! -name .git -exec rm -rf {} +
cp -R "${artifact_dir}/." "${worktree_dir}/"

git -C "${worktree_dir}" add -A

if git -C "${worktree_dir}" diff --cached --quiet && [[ "${force_publish}" != "1" ]]; then
  echo "No Pages artifact changes to publish for ${source_short}."
  exit 0
fi

if git -C "${worktree_dir}" diff --cached --quiet; then
  git -C "${worktree_dir}" commit --allow-empty -m "Publish Pages from ${source_short}"
else
  git -C "${worktree_dir}" commit -m "Publish Pages from ${source_short}"
fi

git -C "${worktree_dir}" push "${remote}" "HEAD:${branch}"
echo "Published Pages artifact built from ${source_commit} to ${remote}/${branch}."
