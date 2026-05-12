#!/usr/bin/env bash
set -euo pipefail

output_dir="${1:?usage: build_pages_artifact.sh <output-dir>}"
source_dir="${PAGES_SOURCE_DIR:-web}"
source_commit="${PAGES_SOURCE_COMMIT:-}"
source_short="${PAGES_SOURCE_SHORT:-}"

[[ -f "${source_dir}/index.html" ]] || { echo "error: ${source_dir}/index.html is required" >&2; exit 1; }
[[ -f "${source_dir}/dashboard.html" ]] || { echo "error: ${source_dir}/dashboard.html is required" >&2; exit 1; }
[[ -f "${source_dir}/event-log.html" ]] || { echo "error: ${source_dir}/event-log.html is required" >&2; exit 1; }
[[ -f "${source_dir}/smtp-relay.html" ]] || { echo "error: ${source_dir}/smtp-relay.html is required" >&2; exit 1; }
[[ -f "${source_dir}/CNAME" ]] || { echo "error: ${source_dir}/CNAME is required" >&2; exit 1; }

if [[ -z "${source_commit}" ]]; then
  source_commit="$(git rev-parse --verify HEAD 2>/dev/null || printf "unknown")"
fi
if [[ -z "${source_short}" ]]; then
  if [[ "${source_commit}" == "unknown" ]]; then
    source_short="unknown"
  else
    source_short="${source_commit:0:12}"
  fi
fi

rm -rf "${output_dir}"
mkdir -p "${output_dir}"
cp -R "${source_dir}/." "${output_dir}/"
touch "${output_dir}/.nojekyll"
printf '{"sourceCommit":"%s","sourceShort":"%s"}\n' "${source_commit}" "${source_short}" > "${output_dir}/pinguin-pages-build.json"
find "${output_dir}" -name .DS_Store -delete
