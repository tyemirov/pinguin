#!/usr/bin/env bash
set -euo pipefail

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

PAGES_URL="$(env_or_default PAGES_URL https://pinguin.mprlab.com/)"
PAGES_BRANCH="$(env_or_default PAGES_PUBLISH_BRANCH gh-pages)"
PAGES_REPOSITORY="$(env_or_default PAGES_REPOSITORY tyemirov/pinguin)"
PAGES_VERIFY_ATTEMPTS="$(env_or_default PAGES_VERIFY_ATTEMPTS 18)"
PAGES_VERIFY_DELAY_SEC="$(env_or_default PAGES_VERIFY_DELAY_SEC 10)"
PAGES_VERIFY_SKIP="$(env_or_default PAGES_VERIFY_SKIP false)"

configure_legacy_pages() {
  command -v gh >/dev/null 2>&1 || { echo "error: gh is required to configure legacy GitHub Pages" >&2; exit 1; }
  echo "==> [pages] Configuring GitHub Pages legacy source ${PAGES_BRANCH}/ for ${PAGES_REPOSITORY}"
  local pages_payload
  pages_payload="$(mktemp)"
  printf '{"build_type":"legacy","source":{"branch":"%s","path":"/"}}\n' "${PAGES_BRANCH}" > "${pages_payload}"

  if gh api "repos/${PAGES_REPOSITORY}/pages" >/dev/null 2>&1; then
    gh api --method PUT "repos/${PAGES_REPOSITORY}/pages" --input "${pages_payload}" >/dev/null
  else
    gh api --method POST "repos/${PAGES_REPOSITORY}/pages" --input "${pages_payload}" >/dev/null
  fi
  rm -f "${pages_payload}"

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
  echo "==> [pages] Triggering GitHub Pages build for ${PAGES_REPOSITORY}"
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
        echo "==> [pages] Verified Pages artifact source ${expected_commit}"
        return 0
      fi
      echo "==> [pages] Waiting for Pages artifact source ${expected_commit}; marker returned ${payload}" >&2
    else
      last_payload="${payload}"
      echo "==> [pages] Waiting for Pages artifact source ${expected_commit}; marker fetch failed: ${payload}" >&2
    fi

    if [[ "${attempt}" != "${PAGES_VERIFY_ATTEMPTS}" ]]; then
      sleep "${PAGES_VERIFY_DELAY_SEC}"
    fi
  done

  echo "error: ${PAGES_URL} did not serve Pages artifact source ${expected_commit}; last marker response: ${last_payload}" >&2
  exit 1
}

command -v git >/dev/null 2>&1 || { echo "error: git is required" >&2; exit 1; }
repo_root="$(git rev-parse --show-toplevel)"
cd "${repo_root}"

source_commit="$(git rev-parse --verify HEAD)"
source_short="$(git rev-parse --short HEAD)"

./scripts/publish_pages_branch.sh
configure_legacy_pages
trigger_legacy_pages_deploy

if [[ "${PAGES_VERIFY_SKIP}" != "true" && "${PAGES_VERIFY_SKIP}" != "1" ]]; then
  command -v curl >/dev/null 2>&1 || { echo "error: curl is required for Pages verification" >&2; exit 1; }
  echo "==> [pages] Verifying ${PAGES_URL}"
  timeout -k 60s -s SIGKILL 60s curl --fail --silent --show-error --location --max-time 30 "${PAGES_URL}" >/dev/null
  verify_live_pages_source_commit "${source_commit}" "$(pages_build_marker_url)"
fi
