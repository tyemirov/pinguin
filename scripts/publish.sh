#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'USAGE'
Usage:
  scripts/publish.sh [options]

Publishes from master by:
  1. validating clean local master matches origin/master
  2. running make ci
  3. validating a pushed release tag points at HEAD
  4. building and pushing the linux/amd64 and linux/arm64 Docker image manifest to GHCR

This command does not publish GitHub Pages. Run make deploy after make publish.

Options:
  --image <value>       Full image name without tag. Default: $DOCKER_IMAGE or ghcr.io/tyemirov/pinguin
  --platforms <value>   Docker platforms. Default: $PUBLISH_PLATFORMS or linux/amd64,linux/arm64
  --tag <value>         Docker tag. Default: $DOCKER_TAG or v* tag at HEAD
  --context <value>     Docker build context directory. Default: $DOCKER_BUILD_CONTEXT or .
  --no-latest           Do not push :latest
  --dry-run             Validate and print image tags without pushing
  --username <value>    Registry username. Default: $GHCR_USERNAME or gh authenticated user
  --token <value>       Registry token/password. Default: $GHCR_TOKEN, $GITHUB_TOKEN, $GH_TOKEN, or gh auth token
  --skip-docker-login   Use the existing Docker credential store without logging in
  --help                Show this help text
USAGE
}

IMAGE="${DOCKER_IMAGE:-ghcr.io/tyemirov/pinguin}"
TAG="${DOCKER_TAG:-}"
PLATFORMS="${PUBLISH_PLATFORMS:-linux/amd64,linux/arm64}"
BUILDER="${DOCKER_BUILDX_BUILDER:-pinguin-builder}"
DOCKERFILE_PATH="${DOCKERFILE:-Dockerfile}"
DOCKER_CONTEXT_DIR="${DOCKER_BUILD_CONTEXT:-.}"
USERNAME="${GHCR_USERNAME:-}"
TOKEN="${GHCR_TOKEN:-${GITHUB_TOKEN:-${GH_TOKEN:-}}}"
SKIP_DOCKER_LOGIN="${PUBLISH_SKIP_DOCKER_LOGIN:-0}"
PUSH_LATEST="${PUBLISH_LATEST:-1}"
DRY_RUN="${PUBLISH_DRY_RUN:-0}"
PUBLISH_BRANCH="${PUBLISH_BRANCH:-master}"
PUBLISH_REMOTE="${PUBLISH_REMOTE:-origin}"

while [[ $# -gt 0 ]]; do
  case "$1" in
    --image)
      [[ $# -ge 2 ]] || { echo "error: --image requires a value" >&2; exit 1; }
      IMAGE="$2"
      shift 2
      ;;
    --platforms)
      [[ $# -ge 2 ]] || { echo "error: --platforms requires a value" >&2; exit 1; }
      PLATFORMS="$2"
      shift 2
      ;;
    --tag)
      [[ $# -ge 2 ]] || { echo "error: --tag requires a value" >&2; exit 1; }
      TAG="$2"
      shift 2
      ;;
    --context|--build-context)
      [[ $# -ge 2 ]] || { echo "error: $1 requires a value" >&2; exit 1; }
      DOCKER_CONTEXT_DIR="$2"
      shift 2
      ;;
    --no-latest)
      PUSH_LATEST="0"
      shift
      ;;
    --dry-run)
      DRY_RUN="1"
      shift
      ;;
    --username)
      [[ $# -ge 2 ]] || { echo "error: --username requires a value" >&2; exit 1; }
      USERNAME="$2"
      shift 2
      ;;
    --token)
      [[ $# -ge 2 ]] || { echo "error: --token requires a value" >&2; exit 1; }
      TOKEN="$2"
      shift 2
      ;;
    --skip-docker-login)
      SKIP_DOCKER_LOGIN="1"
      shift
      ;;
    --help|-h)
      usage
      exit 0
      ;;
    *)
      echo "error: unknown argument: $1" >&2
      usage
      exit 1
      ;;
  esac
done

command -v docker >/dev/null 2>&1 || { echo "error: docker is required" >&2; exit 1; }
command -v gh >/dev/null 2>&1 || { echo "error: gh is required for registry authentication" >&2; exit 1; }

timeout -k 30s -s SIGKILL 30s git fetch "${PUBLISH_REMOTE}" "${PUBLISH_BRANCH}" --prune

current_branch="$(git branch --show-current)"
if [[ "${current_branch}" != "${PUBLISH_BRANCH}" ]]; then
  echo "error: publishing is allowed only from branch '${PUBLISH_BRANCH}' (current: '${current_branch:-detached HEAD}')" >&2
  exit 1
fi

if [[ -n "$(git status --porcelain)" ]]; then
  echo "error: working tree is dirty; commit or stash changes before publishing" >&2
  exit 1
fi

head_sha="$(git rev-parse HEAD)"
remote_sha="$(git rev-parse "${PUBLISH_REMOTE}/${PUBLISH_BRANCH}")"
if [[ "${head_sha}" != "${remote_sha}" ]]; then
  echo "error: local ${PUBLISH_BRANCH} is not at ${PUBLISH_REMOTE}/${PUBLISH_BRANCH}; push or pull first" >&2
  exit 1
fi

if [[ -z "${TAG}" ]]; then
  TAG="$(git tag --points-at HEAD --list 'v*' --sort=-version:refname | head -n 1)"
fi
[[ -n "${TAG}" ]] || { echo "error: no v* release tag points at HEAD; create/push a release tag before publishing" >&2; exit 1; }

git rev-parse -q --verify "refs/tags/${TAG}" >/dev/null || { echo "error: release tag ${TAG} does not exist locally" >&2; exit 1; }
tag_sha="$(git rev-list -n 1 "${TAG}")"
if [[ "${tag_sha}" != "${head_sha}" ]]; then
  echo "error: release tag ${TAG} does not point at HEAD" >&2
  exit 1
fi

remote_tag_refs="$(git ls-remote --tags "${PUBLISH_REMOTE}" "refs/tags/${TAG}" "refs/tags/${TAG}^{}")"
remote_tag_sha="$(awk '$2 ~ /\^\{\}$/ { peeled = $1 } $2 !~ /\^\{\}$/ { direct = $1 } END { if (peeled != "") print peeled; else print direct }' <<<"${remote_tag_refs}")"
if [[ "${remote_tag_sha}" != "${head_sha}" ]]; then
  echo "error: release tag ${TAG} is not pushed to ${PUBLISH_REMOTE} at HEAD" >&2
  exit 1
fi

echo "==> [publish] Running make ci before publishing"
timeout -k 350s -s SIGKILL 350s make ci

if [[ "${DRY_RUN}" == "1" || "${DRY_RUN}" == "true" ]]; then
  echo "publish_dry_run=true"
  echo "release_branch=${PUBLISH_BRANCH}"
  echo "release_tag=${TAG}"
  echo "image=${IMAGE}:${TAG}"
  if [[ "${PUSH_LATEST}" == "1" || "${PUSH_LATEST}" == "true" ]]; then
    echo "image=${IMAGE}:latest"
  fi
  exit 0
fi

registry_host="$(printf '%s' "${IMAGE}" | cut -d'/' -f1)"
[[ -n "${registry_host}" ]] || { echo "error: unable to determine registry host from image: ${IMAGE}" >&2; exit 1; }

if [[ "${SKIP_DOCKER_LOGIN}" != "1" ]]; then
  if [[ -z "${TOKEN}" ]]; then
    TOKEN="$(gh auth token 2>/dev/null || true)"
  fi
  [[ -n "${TOKEN}" ]] || { echo "error: registry token is required (use --token, GHCR_TOKEN, GITHUB_TOKEN, GH_TOKEN, or --skip-docker-login)" >&2; exit 1; }
  if [[ -z "${USERNAME}" ]]; then
    USERNAME="$(gh api user --jq '.login')"
  fi
  [[ -n "${USERNAME}" ]] || { echo "error: could not infer registry username; use --username or GHCR_USERNAME" >&2; exit 1; }
  echo "==> [publish] Logging in to ${registry_host}"
  echo "${TOKEN}" | timeout -k 30s -s SIGKILL 30s docker login "${registry_host}" -u "${USERNAME}" --password-stdin
fi

if ! docker buildx inspect "${BUILDER}" >/dev/null 2>&1; then
  docker buildx create --name "${BUILDER}" --driver docker-container >/dev/null
fi
docker buildx inspect --bootstrap --builder "${BUILDER}" >/dev/null

tag_args=(--tag "${IMAGE}:${TAG}")
if [[ "${PUSH_LATEST}" == "1" || "${PUSH_LATEST}" == "true" ]]; then
  tag_args+=(--tag "${IMAGE}:latest")
fi

echo "==> [publish] Building and pushing ${IMAGE}:${TAG} for ${PLATFORMS}"
timeout -k 1200s -s SIGKILL 1200s docker buildx build \
  --builder "${BUILDER}" \
  --pull \
  --platform "${PLATFORMS}" \
  --file "${DOCKERFILE_PATH}" \
  "${tag_args[@]}" \
  --push \
  "${DOCKER_CONTEXT_DIR}"

echo "Published image: ${IMAGE}:${TAG}"
if [[ "${PUSH_LATEST}" == "1" || "${PUSH_LATEST}" == "true" ]]; then
  echo "Published image: ${IMAGE}:latest"
fi
