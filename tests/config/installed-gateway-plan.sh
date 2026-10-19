#!/usr/bin/env bash
set -euo pipefail

export CI=1
for git_variable in "${!GIT_@}"; do
  unset "$git_variable"
done
export GIT_TERMINAL_PROMPT=0
export GIT_CONFIG_NOSYSTEM=1
export GIT_CONFIG_SYSTEM=/dev/null
export GIT_CONFIG_GLOBAL=/dev/null
export GIT_CONFIG_COUNT=0
exec </dev/null

repository_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd -P)"
gateway_command="${MPRLAB_GATEWAY_EXECUTABLE:-mprlab-gateway}"
fixture_root="$(mktemp -d)"
trap 'rm -rf "$fixture_root"' EXIT
fixture_root="$(cd "$fixture_root" && pwd -P)"
application_root="$fixture_root/application"
mkdir -p "$application_root"

while IFS= read -r -d '' relative_path; do
  mkdir -p "$application_root/$(dirname "$relative_path")"
  cp -P "$repository_root/$relative_path" "$application_root/$relative_path"
done < <(git -C "$repository_root" ls-files -z)

git -C "$application_root" init --quiet --initial-branch=master
git -C "$application_root" add --all
git -C "$application_root" -c core.hooksPath=/dev/null -c commit.gpgsign=false \
  -c user.name='Gateway integration test' -c user.email='gateway-test@example.invalid' \
  commit --quiet -m 'Selected manifest integration fixture'
git clone --quiet --bare "$application_root" "$fixture_root/origin.git"

cat >"$fixture_root/ssh-transport.sh" <<EOF
set -eu
if [ "\$#" -ne 2 ] || [ "\$1" != 'git@github.com' ] || [ "\$2" != "git-upload-pack 'tyemirov/pinguin.git'" ]; then
  printf 'unexpected fixture Git transport arguments\\n' >&2
  exit 2
fi
exec git upload-pack '$fixture_root/origin.git'
EOF

git -C "$application_root" remote add origin git@github.com:tyemirov/pinguin.git
export GIT_SSH_COMMAND="sh '$fixture_root/ssh-transport.sh'"
export GIT_SSH_VARIANT=simple
git -C "$application_root" config branch.master.remote origin
git -C "$application_root" config branch.master.merge refs/heads/master
git -C "$application_root" fetch --quiet origin
git -C "$application_root" remote set-head origin master

"$gateway_command" app-plan-release --app-root "$application_root" --operator-root "$fixture_root/operator" --ansible-inventory localhost,
