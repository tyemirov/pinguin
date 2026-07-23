#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'USAGE'
Usage:
  scripts/deploy.sh [options]

Deploys the published Pinguin backend through mprlab-gateway, then activates
the published Pages artifact. Deployment is allowed only from clean local
master matching origin/master with zero open PRs.

Options:
  --gateway-dir <path>       Gateway checkout. Default: $GATEWAY_DIR or sibling ../mprlab-gateway
  --image <value>            Backend image repository. Default: $DOCKER_IMAGE or ghcr.io/tyemirov/pinguin
  --tag <value>              Release tag to verify. Default: v* tag pointing at HEAD
  --skip-image-verify        Skip release tag/latest image digest verification
  --skip-backend             Skip gateway backend deployment
  --skip-pages               Skip GitHub Pages branch publication
  --skip-pages-verify        Skip public Pages URL verification
  --pages-url <url>          Pages URL to verify. Default: $PAGES_URL or https://pinguin.mprlab.com/
  --help                     Show this help text
USAGE
}

env_or_default() {
  local name="$1"
  local fallback="$2"
  local value=""
  if [[ -v "${name}" ]]; then
    value="${!name}"
  fi
  if [[ -n "${value}" ]]; then
    printf "%s\n" "${value}"
  else
    printf "%s\n" "${fallback}"
  fi
}

GATEWAY_DIR="$(env_or_default GATEWAY_DIR "")"
IMAGE_REPOSITORY="$(env_or_default DOCKER_IMAGE ghcr.io/tyemirov/pinguin)"
PAGES_URL="$(env_or_default PAGES_URL https://pinguin.mprlab.com/)"
PUBLISH_BRANCH="$(env_or_default PAGES_PUBLISH_SOURCE_BRANCH master)"
PUBLISH_REMOTE="$(env_or_default PAGES_PUBLISH_REMOTE origin)"
TAG="$(env_or_default DEPLOY_TAG "")"
SKIP_IMAGE_VERIFY="false"
SKIP_BACKEND="false"
SKIP_PAGES="false"
SKIP_PAGES_VERIFY="false"

image_digest() {
  local image_ref="$1"
  docker buildx imagetools inspect "$image_ref" | awk '/^Digest:/ { print $2; exit }'
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --gateway-dir)
      [[ $# -ge 2 ]] || { echo "error: --gateway-dir requires a value" >&2; exit 1; }
      GATEWAY_DIR="$2"
      shift 2
      ;;
    --image)
      [[ $# -ge 2 ]] || { echo "error: --image requires a value" >&2; exit 1; }
      IMAGE_REPOSITORY="$2"
      shift 2
      ;;
    --tag)
      [[ $# -ge 2 ]] || { echo "error: --tag requires a value" >&2; exit 1; }
      TAG="$2"
      shift 2
      ;;
    --skip-image-verify)
      SKIP_IMAGE_VERIFY="true"
      shift
      ;;
    --skip-backend)
      SKIP_BACKEND="true"
      shift
      ;;
    --skip-pages)
      SKIP_PAGES="true"
      shift
      ;;
    --skip-pages-verify)
      SKIP_PAGES_VERIFY="true"
      shift
      ;;
    --pages-url)
      [[ $# -ge 2 ]] || { echo "error: --pages-url requires a value" >&2; exit 1; }
      PAGES_URL="$2"
      shift 2
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

command -v git >/dev/null 2>&1 || { echo "error: git is required" >&2; exit 1; }

repo_root="$(git rev-parse --show-toplevel)"
cd "${repo_root}"
source "${repo_root}/scripts/production_git_guard.sh"
if [[ "${SKIP_BACKEND}" != "true" || "${SKIP_PAGES}" != "true" ]]; then
  verify_production_git_state "deploy" "${PUBLISH_BRANCH}" "${PUBLISH_REMOTE}"
fi

resolve_gateway_dir() {
  if [[ -n "${GATEWAY_DIR}" ]]; then
    printf "%s\n" "${GATEWAY_DIR}"
    return
  fi
  printf "%s\n" "${repo_root}/../mprlab-gateway"
}

GATEWAY_DIR="$(resolve_gateway_dir)"
[[ -n "${GATEWAY_DIR}" ]] || { echo "error: gateway checkout not found; set GATEWAY_DIR=/path/to/mprlab-gateway or pass --gateway-dir" >&2; exit 1; }
[[ -d "${GATEWAY_DIR}" ]] || { echo "error: gateway checkout not found: ${GATEWAY_DIR}" >&2; exit 1; }

if [[ -z "${TAG}" ]]; then
  TAG="$(git tag --points-at HEAD --list 'v*' --sort=-version:refname | head -n 1)"
fi
if [[ "${SKIP_BACKEND}" != "true" || "${SKIP_PAGES}" != "true" ]]; then
  [[ -n "${TAG}" ]] || { echo "error: no v* release tag points at HEAD; pass --tag or run make publish first" >&2; exit 1; }
fi

if [[ "${SKIP_IMAGE_VERIFY}" != "true" && "${SKIP_BACKEND}" != "true" ]]; then
  command -v docker >/dev/null 2>&1 || { echo "error: docker is required for image verification" >&2; exit 1; }
  docker buildx version >/dev/null 2>&1 || { echo "error: docker buildx is required for image verification" >&2; exit 1; }
  echo "==> [deploy] Verifying ${IMAGE_REPOSITORY}:latest matches ${TAG}"
  release_digest="$(image_digest "${IMAGE_REPOSITORY}:${TAG}")"
  latest_digest="$(image_digest "${IMAGE_REPOSITORY}:latest")"
  [[ -n "${release_digest}" ]] || { echo "error: could not resolve digest for ${IMAGE_REPOSITORY}:${TAG}" >&2; exit 1; }
  [[ -n "${latest_digest}" ]] || { echo "error: could not resolve digest for ${IMAGE_REPOSITORY}:latest" >&2; exit 1; }
  if [[ "${release_digest}" != "${latest_digest}" ]]; then
    echo "error: ${IMAGE_REPOSITORY}:latest digest ${latest_digest} does not match ${TAG} digest ${release_digest}; run make publish first" >&2
    exit 1
  fi
fi

if [[ "${SKIP_BACKEND}" != "true" ]]; then
  echo "==> [deploy] Deploying Pinguin backend through mprlab-gateway"
  timeout --foreground -k 1200s -s SIGKILL 1200s make -C "${GATEWAY_DIR}" deploy-pinguin-backend
  echo "==> [deploy] Gateway SMTP host ports are ready; remaining operator mapping is edge 25 -> tutosh:8025 and edge 465 -> tutosh:8465"
fi

if [[ "${SKIP_PAGES}" != "true" ]]; then
  pages_args=()
  [[ "${SKIP_PAGES_VERIFY}" == "true" ]] && pages_args+=(--skip-verify)
  echo "==> [deploy] Activating the published Pages artifact for ${TAG}"
  PAGES_URL="${PAGES_URL}" PAGES_VERSION="${TAG}" PAGES_DEPLOY_ARGS="${pages_args[*]}" make --no-print-directory pages-deploy
fi

echo "Pinguin deploy complete"
