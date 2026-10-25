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

# Supply private inputs only in this owned fixture after its tracked-source commit.
selected_input_file="$application_root/.mprlab/deploy/.env"
cat >"$selected_input_file" <<'INPUTS'
CONNECTION_TIMEOUT_SEC=5
DATABASE_PATH=/app/data/fixture.db
HTTP_ALLOWED_ORIGIN1=https://pinguin.fixture.invalid
HTTP_ALLOWED_ORIGIN2=http://127.0.0.1:4174
HTTP_ALLOWED_ORIGIN3=http://localhost:4174
HTTP_LISTEN_ADDR=:8080
LOG_LEVEL=info
MASTER_ENCRYPTION_KEY=aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa
MAX_RETRIES=2
OPERATION_TIMEOUT_SEC=15
RETRY_INTERVAL_SEC=1
SMTP_FORWARDING_ENABLED=false
SMTP_FORWARDING_HOSTNAME=mx.fixture.invalid
SMTP_FORWARDING_LISTEN_ADDR=:25
SMTP_FORWARDING_MAX_MESSAGE_BYTES=1048576
SMTP_FORWARDING_MAX_RECIPIENTS=10
SMTP_FORWARDING_RELAY_HOST=relay.fixture.invalid
SMTP_FORWARDING_RELAY_PASSWORD=fixture-forward-password
SMTP_FORWARDING_RELAY_PORT=587
SMTP_FORWARDING_RELAY_USERNAME=fixture-forward-user
SMTP_SUBMISSION_ALLOW_INSECURE_AUTH=true
SMTP_SUBMISSION_DELIVERY_MODE=upstream
SMTP_SUBMISSION_HOSTNAME=smtp.fixture.invalid
SMTP_SUBMISSION_LISTEN_ADDR=:587
SMTP_SUBMISSION_MAX_MESSAGE_BYTES=1048576
SMTP_SUBMISSION_MAX_RECIPIENTS=10
SMTP_SUBMISSION_PUBLIC_PORT=465
SMTP_SUBMISSION_PUBLIC_SECURITY_MODE=ssl
SMTP_SUBMISSION_RELAY_HOST=relay.fixture.invalid
SMTP_SUBMISSION_RELAY_PASSWORD=fixture-submit-password
SMTP_SUBMISSION_RELAY_PORT=587
SMTP_SUBMISSION_RELAY_USERNAME=fixture-submit-user
TAUTH_COOKIE_NAME=app_session_pinguin_fixture
TAUTH_SIGNING_KEY=fixture-signing-key-with-at-least-32-bytes
TAUTH_TENANT_GOOGLE_WEB_CLIENT_ID_PINGUIN=fixture-client.apps.googleusercontent.com
DEPLOY_SUDO_PASSWORD_TUTOSH=fixture-not-used-for-planning
MPRLAB_TAUTH_MANAGEMENT_URL=https://tauth.fixture.invalid
MPRLAB_TAUTH_PROVISIONING_CREDENTIALS='{"pinguin":{"authentication":"tauthp_fixture"}}'
INPUTS
while IFS='=' read -r fixture_input_name fixture_input_value; do
  unset "$fixture_input_name"
done <"$selected_input_file"
git -C "$application_root" check-ignore --quiet .mprlab/deploy/.env
if [ -n "$(git -C "$application_root" status --porcelain)" ]; then
  printf 'synthetic selected inputs changed the tracked-source fixture\n' >&2
  exit 2
fi

"$gateway_command" app-plan-release --app-root "$application_root" --operator-root "$fixture_root/operator" --ansible-inventory localhost,
