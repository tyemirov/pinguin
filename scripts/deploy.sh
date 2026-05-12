#!/usr/bin/env bash
set -euo pipefail

usage() {
  cat <<'USAGE'
Usage:
  scripts/deploy.sh [options]

Deploys the Pinguin backend through mprlab-gateway, then publishes legacy
gh-pages only after backend verification succeeds.

Options:
  --gateway-dir <path>       Gateway checkout. Default: $GATEWAY_DIR or sibling ../mprlab-gateway
  --image <value>            Backend image repository. Default: $DOCKER_IMAGE or ghcr.io/tyemirov/pinguin
  --tag <value>              Release tag to verify. Default: v* tag pointing at HEAD
  --skip-ci                  Skip local make ci deployment gate
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
PAGES_BRANCH="$(env_or_default PAGES_PUBLISH_BRANCH gh-pages)"
PAGES_REPOSITORY="$(env_or_default PAGES_REPOSITORY tyemirov/pinguin)"
PAGES_VERIFY_ATTEMPTS="$(env_or_default PAGES_VERIFY_ATTEMPTS 18)"
PAGES_VERIFY_DELAY_SEC="$(env_or_default PAGES_VERIFY_DELAY_SEC 10)"
TAG="$(env_or_default DEPLOY_TAG "")"
SKIP_CI="false"
SKIP_IMAGE_VERIFY="false"
SKIP_BACKEND="false"
SKIP_PAGES="false"
SKIP_PAGES_VERIFY="false"

image_digest() {
  local image_ref="$1"
  docker buildx imagetools inspect "$image_ref" | awk '/^Digest:/ { print $2; exit }'
}

configure_legacy_pages() {
  command -v gh >/dev/null 2>&1 || { echo "error: gh is required to configure legacy GitHub Pages" >&2; exit 1; }
  echo "==> [deploy] Configuring GitHub Pages legacy source ${PAGES_BRANCH}/ for ${PAGES_REPOSITORY}"
  pages_payload="$(mktemp)"
  trap 'rm -f "${pages_payload}"' RETURN
  printf '{"build_type":"legacy","source":{"branch":"%s","path":"/"}}\n' "${PAGES_BRANCH}" > "${pages_payload}"

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

trigger_legacy_pages_deploy() {
  command -v gh >/dev/null 2>&1 || { echo "error: gh is required to trigger GitHub Pages deployment" >&2; exit 1; }
  echo "==> [deploy] Triggering GitHub Pages build for ${PAGES_REPOSITORY}"
  gh api --method POST "repos/${PAGES_REPOSITORY}/pages/builds" >/dev/null
}

pages_build_marker_url() {
  local base_url="${PAGES_URL%/}"
  printf "%s/pinguin-pages-build.json?source=%s\n" "${base_url}" "${source_short}"
}

verify_live_pages_source_commit() {
  local expected_commit="$1"
  local marker_url="$2"
  local attempt
  local payload
  local last_payload=""

  for attempt in $(seq 1 "${PAGES_VERIFY_ATTEMPTS}"); do
    if payload="$(curl --fail --silent --show-error --location --max-time 30 "${marker_url}" 2>&1)"; then
      last_payload="${payload}"
      if [[ "${payload}" == *"\"sourceCommit\":\"${expected_commit}\""* ]]; then
        echo "==> [deploy] Verified Pages artifact source ${expected_commit}"
        return 0
      fi
      echo "==> [deploy] Waiting for Pages artifact source ${expected_commit}; marker returned ${payload}" >&2
    else
      last_payload="${payload}"
      echo "==> [deploy] Waiting for Pages artifact source ${expected_commit}; marker fetch failed: ${payload}" >&2
    fi

    if [[ "${attempt}" != "${PAGES_VERIFY_ATTEMPTS}" ]]; then
      sleep "${PAGES_VERIFY_DELAY_SEC}"
    fi
  done

  echo "error: ${PAGES_URL} did not serve Pages artifact source ${expected_commit}; last marker response: ${last_payload}" >&2
  exit 1
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
    --skip-ci)
      SKIP_CI="true"
      shift
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
source_commit="$(git rev-parse --verify HEAD)"
source_short="$(git rev-parse --short HEAD)"
resolve_gateway_dir() {
  local candidate
  if [[ -n "${GATEWAY_DIR}" ]]; then
    printf "%s\n" "${GATEWAY_DIR}"
    return
  fi
  for candidate in "${repo_root}/../mprlab-gateway" "../mprlab-gateway"; do
    if [[ -d "${candidate}" ]]; then
      printf "%s\n" "${candidate}"
      return
    fi
  done
}

GATEWAY_DIR="$(resolve_gateway_dir)"
[[ -n "${GATEWAY_DIR}" ]] || { echo "error: gateway checkout not found; set GATEWAY_DIR=/path/to/mprlab-gateway or pass --gateway-dir" >&2; exit 1; }
[[ -d "${GATEWAY_DIR}" ]] || { echo "error: gateway checkout not found: ${GATEWAY_DIR}" >&2; exit 1; }

require_gateway_contract_snippet() {
  local relative_path="$1"
  local expected_snippet="$2"
  local file_path="${GATEWAY_DIR}/${relative_path}"

  [[ -f "${file_path}" ]] || { echo "error: gateway contract file missing: ${file_path}" >&2; exit 1; }
  if ! grep -F -- "${expected_snippet}" "${file_path}" >/dev/null; then
    echo "error: gateway checkout ${GATEWAY_DIR} is missing required SMTP port contract in ${relative_path}: ${expected_snippet}" >&2
    echo "       update mprlab-gateway so Caddy publishes high host ports before deploying Pinguin." >&2
    exit 1
  fi
}

verify_gateway_smtp_port_contract() {
  require_gateway_contract_snippet "docker-compose.yml" '${PINGUIN_SMTP_HOST_PORT}:${PINGUIN_SMTP_PUBLIC_PORT}'
  require_gateway_contract_snippet "docker-compose.yml" '${PINGUIN_SMTP_FORWARDING_HOST_PORT}:${PINGUIN_SMTP_FORWARDING_PUBLIC_PORT}'
  require_gateway_contract_snippet "configs/.env.caddy" "PINGUIN_SMTP_HOST_PORT=8465"
  require_gateway_contract_snippet "configs/.env.caddy" "PINGUIN_SMTP_FORWARDING_HOST_PORT=8025"
  require_gateway_contract_snippet "deploy/ansible/inventory/group_vars/gateway.yml" "mprlab_verify_pinguin_smtp_port: 8465"
  require_gateway_contract_snippet "deploy/ansible/inventory/group_vars/gateway.yml" "mprlab_verify_pinguin_mx_port: 8025"
}

if [[ -z "${TAG}" ]]; then
  TAG="$(git tag --points-at HEAD --list 'v*' --sort=-version:refname | head -n 1)"
fi
[[ -n "${TAG}" ]] || { echo "error: no v* release tag points at HEAD; pass --tag or deploy from a release commit" >&2; exit 1; }

if [[ "${SKIP_CI}" != "true" && ( "${SKIP_BACKEND}" != "true" || "${SKIP_PAGES}" != "true" ) ]]; then
  echo "==> [deploy] Running make ci before deployment"
  timeout -k 1200s -s SIGKILL 1200s make ci
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
  verify_gateway_smtp_port_contract
  echo "==> [deploy] Deploying Pinguin backend through mprlab-gateway"
  timeout --foreground -k 1200s -s SIGKILL 1200s make -C "${GATEWAY_DIR}" deploy TARGET=pinguin
  echo "==> [deploy] Gateway SMTP host ports are ready; remaining operator mapping is edge 25 -> tutosh:8025 and edge 465 -> tutosh:8465"
fi

if [[ "${SKIP_PAGES}" != "true" ]]; then
  [[ -f "web/index.html" ]] || { echo "error: web/index.html is required for Pages publishing" >&2; exit 1; }
  [[ -f "web/dashboard.html" ]] || { echo "error: web/dashboard.html is required for Pages compatibility redirect" >&2; exit 1; }
  [[ -f "web/event-log.html" ]] || { echo "error: web/event-log.html is required for Pages publishing" >&2; exit 1; }
  [[ -f "web/smtp-relay.html" ]] || { echo "error: web/smtp-relay.html is required for Pages publishing" >&2; exit 1; }
  [[ -f "web/CNAME" ]] || { echo "error: web/CNAME is required for Pages custom domain publishing" >&2; exit 1; }
  [[ -f "web/.nojekyll" ]] || { echo "error: web/.nojekyll is required for branch Pages publishing without Jekyll" >&2; exit 1; }
  echo "==> [deploy] Publishing GitHub Pages after backend verification"
  PAGES_PUBLISH_SOURCE_BRANCH="${PUBLISH_BRANCH}" PAGES_PUBLISH_REMOTE="${PUBLISH_REMOTE}" PAGES_PUBLISH_BRANCH="${PAGES_BRANCH}" ./scripts/publish_pages_branch.sh
  configure_legacy_pages
  trigger_legacy_pages_deploy
fi

if [[ "${SKIP_PAGES_VERIFY}" != "true" ]]; then
  command -v curl >/dev/null 2>&1 || { echo "error: curl is required for Pages verification" >&2; exit 1; }
  echo "==> [deploy] Verifying ${PAGES_URL}"
  timeout -k 60s -s SIGKILL 60s curl --fail --silent --show-error --location --max-time 30 "${PAGES_URL}" >/dev/null
  if [[ "${SKIP_PAGES}" != "true" ]]; then
    verify_live_pages_source_commit "${source_commit}" "$(pages_build_marker_url)"
  fi
fi

echo "Pinguin deploy complete"
