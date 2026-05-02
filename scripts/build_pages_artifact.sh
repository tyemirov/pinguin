#!/usr/bin/env bash
set -euo pipefail

output_dir="${1:?usage: build_pages_artifact.sh <output-dir>}"
source_dir="${PAGES_SOURCE_DIR:-web}"

[[ -f "${source_dir}/index.html" ]] || { echo "error: ${source_dir}/index.html is required" >&2; exit 1; }
[[ -f "${source_dir}/dashboard.html" ]] || { echo "error: ${source_dir}/dashboard.html is required" >&2; exit 1; }
[[ -f "${source_dir}/CNAME" ]] || { echo "error: ${source_dir}/CNAME is required" >&2; exit 1; }

rm -rf "${output_dir}"
mkdir -p "${output_dir}"
cp -R "${source_dir}/." "${output_dir}/"
touch "${output_dir}/.nojekyll"
find "${output_dir}" -name .DS_Store -delete
