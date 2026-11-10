# Configuration

## Service settings

Pingwin stores service settings in YAML and tenant settings in SQLite.
The local template is [`configs/config.pingwin.yml`](../configs/config.pingwin.yml).
Compose mounts that template at `/config/config.yml`.
Native startup reads `configs/config.yml` first, then `/config/config.yml`.

The YAML supports `${VAR}` expansion.
Every referenced variable must exist in the process environment.
A missing placeholder is a startup error, including placeholders under disabled SMTP listeners.
An unused string can be empty.
Unknown YAML fields are rejected.

The tracked `.env` examples describe names and types only.
Their values are deliberately non-operational.
Create private inputs with the [local setup procedure](getting-started.md#prepare-the-local-stack).

| Input | Purpose |
| --- | --- |
| `DATABASE_PATH` | SQLite file path. |
| `MASTER_ENCRYPTION_KEY` | Hex-encoded 32-byte key for provider secrets and SMTP identity passwords. |
| `LOG_LEVEL` | `DEBUG`, `INFO`, `WARN`, or `ERROR`. |
| `MAX_RETRIES`, `RETRY_INTERVAL_SEC` | Worker retry limit and scan interval. |
| `CONNECTION_TIMEOUT_SEC`, `OPERATION_TIMEOUT_SEC` | Connection and operation budgets. Path-specific behavior is in the delivery guide. |
| `TAUTH_SIGNING_KEY` | HS256 key for TAuth session validation. |
| `TAUTH_COOKIE_NAME` | Session cookie name. The default is `app_session` when HTTP is enabled. |
| `HTTP_LISTEN_ADDR` | Gin HTTP API address. |
| `HTTP_ALLOWED_ORIGIN1/2/3` | Browser origins permitted to send authenticated requests. |
| `HTTP_TRUSTED_PROXY1/2/3` | Proxy addresses or CIDR ranges permitted to supply forwarding headers. |
| `SMTP_SUBMISSION_*`, `SMTP_FORWARDING_*` | Shared listeners and relay profiles. See the SMTP setup guide. |

`web.enabled: false` disables the HTTP API.
The `smtpSubmission.enabled` and `smtpForwarding.enabled` flags independently control SMTP listeners.
A gRPC-only process requires all three optional listeners to be disabled.
An API key and an existing tenant remain necessary for notification requests.

## Local private inputs

Create `configs/.env.pingwin` with the values below.
Replace both key placeholders with generated values before startup.
Generate each key with `openssl rand -hex 32`.
Keep the same TAuth signing key in both service files.

```dotenv
DATABASE_PATH=/app/data/pingwin.db
LOG_LEVEL=INFO
MAX_RETRIES=5
RETRY_INTERVAL_SEC=10
CONNECTION_TIMEOUT_SEC=5
OPERATION_TIMEOUT_SEC=30
MASTER_ENCRYPTION_KEY=<generated-master-key>
TAUTH_SIGNING_KEY=<generated-shared-signing-key>
TAUTH_COOKIE_NAME=pingwin_session
HTTP_LISTEN_ADDR=:8081
HTTP_ALLOWED_ORIGIN1=http://localhost:8080
HTTP_ALLOWED_ORIGIN2=
HTTP_ALLOWED_ORIGIN3=
HTTP_TRUSTED_PROXY1=
HTTP_TRUSTED_PROXY2=
HTTP_TRUSTED_PROXY3=
SMTP_SUBMISSION_ENABLED=false
SMTP_SUBMISSION_HOSTNAME=
SMTP_SUBMISSION_LISTEN_ADDR=
SMTP_SUBMISSION_TLS_LISTEN_ADDR=
SMTP_SUBMISSION_TLS_CERT_PATH=
SMTP_SUBMISSION_TLS_KEY_PATH=
SMTP_SUBMISSION_PUBLIC_PORT=465
SMTP_SUBMISSION_PUBLIC_SECURITY_MODE=ssl
SMTP_SUBMISSION_DELIVERY_MODE=upstream
SMTP_SUBMISSION_MAX_MESSAGE_BYTES=26214400
SMTP_SUBMISSION_MAX_RECIPIENTS=100
SMTP_SUBMISSION_ALLOW_INSECURE_AUTH=false
SMTP_SUBMISSION_RELAY_HOST=
SMTP_SUBMISSION_RELAY_PORT=0
SMTP_SUBMISSION_RELAY_USERNAME=
SMTP_SUBMISSION_RELAY_PASSWORD=
SMTP_FORWARDING_ENABLED=false
SMTP_FORWARDING_HOSTNAME=
SMTP_FORWARDING_LISTEN_ADDR=
SMTP_FORWARDING_MAX_MESSAGE_BYTES=26214400
SMTP_FORWARDING_MAX_RECIPIENTS=100
SMTP_FORWARDING_RELAY_HOST=
SMTP_FORWARDING_RELAY_PORT=0
SMTP_FORWARDING_RELAY_USERNAME=
SMTP_FORWARDING_RELAY_PASSWORD=
```

Create `configs/.env.tauth` with the values below.
Replace the Google client placeholder with your OAuth web client ID.
Replace the signing key placeholder with the same value as `TAUTH_SIGNING_KEY`.

```dotenv
TAUTH_CONFIG_FILE=/config/config.yml
TAUTH_LISTEN_ADDR=:8082
TAUTH_DATABASE_URL=sqlite:///data/tauth.db
TAUTH_ENABLE_CORS=true
TAUTH_CORS_ORIGIN_1=http://localhost:8080
TAUTH_CORS_EXCEPTION_1=
TAUTH_ENABLE_TENANT_HEADER_OVERRIDE=false
TAUTH_TENANT_ID_PINGWIN=pingwin
TAUTH_TENANT_DISPLAY_NAME_PINGWIN=Pingwin
TAUTH_TENANT_ORIGIN_PINGWIN=http://localhost:8080
TAUTH_TENANT_GOOGLE_WEB_CLIENT_ID_PINGWIN=<your-google-web-client-id>
TAUTH_TENANT_JWT_SIGNING_KEY_PINGWIN=<generated-shared-signing-key>
TAUTH_TENANT_SESSION_COOKIE_NAME_PINGWIN=pingwin_session
TAUTH_TENANT_REFRESH_COOKIE_NAME_PINGWIN=pingwin_refresh
TAUTH_ALLOW_INSECURE_HTTP=true
TAUTH_COOKIE_DOMAIN=
```

These values enable local HTTP and disable both SMTP listeners.
They are local development settings, separate from the production template.
Tenant SMTP and Twilio credentials are entered through tenant management, rather than these environment files.

## Managed tenants

An authenticated TAuth user creates a tenant from the **Tenants** workspace.
Creation requires a display name, a complete email profile, and one API credential ID and digest.
The email profile contains the SMTP host, port, username, password, and from address.
An optional SMS profile contains the Twilio account, token, and from number.
The browser generates the API key and displays it once.

Pingwin assigns the tenant UUID and owner from the validated TAuth user ID.
Each owner can list and change only their tenants.
Provider changes take effect from the database without a restart.
Each tenant has exactly one API credential.
Rotation immediately replaces that credential.
Pingwin stores only the SHA-256 digest of its raw secret.

Sender domains, SMTP identities, identity credentials, and forwarding routes belong to the selected tenant.
Shared listeners and upstream SMTP relays remain service configuration.
Permanent tenant deletion removes all tenant-owned records.
The [managed tenant contract](multitenancy-plan.md) defines routes, storage, and conversion acceptance.

## Current schema

Pingwin supports SQLite and one current schema.
GORM is the database library, rather than a promise of other supported database engines.
An empty database receives the managed schema at startup.
A non-empty obsolete schema is rejected.

There is no backward compatibility layer for obsolete data, config keys, endpoints, users, or behavior.
The bounded [production conversion](multitenancy-plan.md#production-data-conversion) is an explicit operator procedure.
Startup does not run that conversion.

## Configuration validation

Build the doctor command:

```bash
go build -o bin/pingwin-doctor ./cmd/doctor
```

For private inputs already loaded into the process, run:

```bash
./bin/pingwin-doctor configs/config.pingwin.yml --expand-env
./bin/pingwin-doctor configs/config.pingwin.yml --expand-env --json
```

For the local Compose environment, run:

```bash
docker compose --profile dev run --rm --no-deps pingwin-dev   pingwin-doctor /config/config.yml --expand-env
```

The command validates exact YAML fields, database settings, retries, timeouts, logs, encryption keys, and enabled listener settings.
It also validates TAuth and web settings when HTTP is enabled.
A validation failure requires correction before startup.

For multiple configurations, run:

```bash
./bin/pingwin-doctor config.yml other-config.yml --cross-validate --expand-env
```

Cross-config checks compare database paths and shared listeners.

## Logs and diagnostics

Pingwin uses Go `slog` for structured logs.
Set `server.logLevel` through `LOG_LEVEL` in the tracked templates.
`DEBUG` enables additional diagnostics.
API keys and provider secrets are excluded from the intended diagnostic output.
HTTP logs trust forwarding headers only from configured proxies.

The [delivery guide](smtp_delivery_plan.md) describes transport limits and message storage.
The [SMTP setup guide](smtp-setup.md) describes shared listener inputs.
