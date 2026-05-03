#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'USAGE'
Usage:
  scripts/publish.sh [options]

Publishes from master by:
  1. validating clean local master matches origin/master
  2. running make ci
  3. configuring GitHub Pages for legacy gh-pages branch-root publishing
  4. building and pushing the linux/amd64 and linux/arm64 Docker image manifest to GHCR
  5. publishing web/ to the gh-pages branch root

Options:
  --image <value>       Full image name without tag. Default: $DOCKER_IMAGE or ghcr.io/tyemirov/pinguin
  --platforms <value>   Docker platforms. Default: $PUBLISH_PLATFORMS or linux/amd64,linux/arm64
  --tag <value>         Docker tag. Default: $DOCKER_TAG or latest
  --context <value>     Docker build context directory. Default: $DOCKER_BUILD_CONTEXT or .
  --username <value>    Registry username. Default: $GHCR_USERNAME or gh authenticated user
  --token <value>       Registry token/password. Default: $GHCR_TOKEN, $GITHUB_TOKEN, $GH_TOKEN, or gh auth token
  --skip-docker-login   Use the existing Docker credential store without logging in
  --help                Show this help text
USAGE
}

IMAGE="${DOCKER_IMAGE:-ghcr.io/tyemirov/pinguin}"
TAG="${DOCKER_TAG:-latest}"
PLATFORMS="${PUBLISH_PLATFORMS:-linux/amd64,linux/arm64}"
BUILDER="${DOCKER_BUILDX_BUILDER:-pinguin-builder}"
DOCKERFILE_PATH="${DOCKERFILE:-Dockerfile}"
DOCKER_CONTEXT_DIR="${DOCKER_BUILD_CONTEXT:-.}"
USERNAME="${GHCR_USERNAME:-}"
TOKEN="${GHCR_TOKEN:-${GITHUB_TOKEN:-${GH_TOKEN:-}}}"
SKIP_DOCKER_LOGIN="${PUBLISH_SKIP_DOCKER_LOGIN:-0}"
PUBLISH_BRANCH="${PAGES_PUBLISH_SOURCE_BRANCH:-master}"
PUBLISH_REMOTE="${PAGES_PUBLISH_REMOTE:-origin}"
PAGES_BRANCH="${PAGES_PUBLISH_BRANCH:-gh-pages}"
PAGES_REPOSITORY="${PAGES_REPOSITORY:-tyemirov/pinguin}"

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
command -v gh >/dev/null 2>&1 || { echo "error: gh is required to configure legacy GitHub Pages" >&2; exit 1; }

configure_legacy_pages() {
  echo "==> [publish] Configuring GitHub Pages legacy source ${PAGES_BRANCH}/ for ${PAGES_REPOSITORY}"
  pages_payload="$(mktemp)"
  trap 'rm -f "${pages_payload}"' EXIT
  cat > "${pages_payload}" <<JSON
{"build_type":"legacy","source":{"branch":"${PAGES_BRANCH}","path":"/"}}
JSON

  if gh api "repos/${PAGES_REPOSITORY}/pages" >/dev/null 2>&1; then
    gh api --method PUT "repos/${PAGES_REPOSITORY}/pages" --input "${pages_payload}" >/dev/null
  else
    gh api --method POST "repos/${PAGES_REPOSITORY}/pages" --input "${pages_payload}" >/dev/null
  fi

  pages_build_type="$(gh api "repos/${PAGES_REPOSITORY}/pages" --jq ".build_type")"
  pages_branch="$(gh api "repos/${PAGES_REPOSITORY}/pages" --jq ".source.branch")"
  pages_path="$(gh api "repos/${PAGES_REPOSITORY}/pages" --jq ".source.path")"
  if [[ "${pages_build_type}" != "legacy" || "${pages_branch}" != "${PAGES_BRANCH}" || "${pages_path}" != "/" ]]; then
    echo "error: GitHub Pages is not configured for legacy ${PAGES_BRANCH}/ publishing; got build_type=${pages_build_type} source=${pages_branch}${pages_path}" >&2
    exit 1
  fi
}

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

[[ -f "web/index.html" ]] || { echo "error: web/index.html is required for Pages publishing" >&2; exit 1; }
[[ -f "web/dashboard.html" ]] || { echo "error: web/dashboard.html is required for Pages publishing" >&2; exit 1; }
[[ -f "web/CNAME" ]] || { echo "error: web/CNAME is required for Pages custom domain publishing" >&2; exit 1; }
[[ -f "web/.nojekyll" ]] || { echo "error: web/.nojekyll is required for branch Pages publishing without Jekyll" >&2; exit 1; }

echo "==> [publish] Running make ci before publishing"
timeout -k 350s -s SIGKILL 350s make ci

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

echo "==> [publish] Building and pushing ${IMAGE}:${TAG} for ${PLATFORMS}"
timeout -k 1200s -s SIGKILL 1200s docker buildx build \
  --builder "${BUILDER}" \
  --pull \
  --platform "${PLATFORMS}" \
  --file "${DOCKERFILE_PATH}" \
  --tag "${IMAGE}:${TAG}" \
  --push \
  "${DOCKER_CONTEXT_DIR}"

echo "Published image: ${IMAGE}:${TAG}"

echo "==> [publish] Publishing GitHub Pages artifact to ${PUBLISH_REMOTE}/${PAGES_BRANCH}"
PAGES_PUBLISH_SOURCE_BRANCH="${PUBLISH_BRANCH}" PAGES_PUBLISH_REMOTE="${PUBLISH_REMOTE}" PAGES_PUBLISH_BRANCH="${PAGES_BRANCH}" ./scripts/publish_pages_branch.sh
configure_legacy_pages
echo "Published Pages: https://pinguin.mprlab.com/"
