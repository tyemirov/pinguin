# Pinguin Notification Service

Pinguin is a notification service written in Go. It exposes a gRPC interface for sending **email** and **SMS** notifications. The service uses SQLite (via GORM) for persistent storage and runs a background worker to retry failed notifications using exponential backoff. Structured logging is provided using Go’s built‑in `slog` package.

Pinguin also ships an optional HTTP + browser dashboard for inspecting and managing queued notifications; set `DISABLE_WEB_INTERFACE=true` (or start with `--disable-web-interface`) to run gRPC-only.

---

## Table of Contents

- [Features](#features)
- [Requirements](#requirements)
- [Installation](#installation)
- [Configuration](#configuration)
- [Running the Server](#running-the-server)
- [Using the gRPC API](#using-the-grpc-api)
  - [Command‑Line Client Test](#command-line-client-test)
  - [Using grpcurl](#using-grpcurl)
- [End-to-End Flow](#end-to-end-flow)
- [Logging and Debugging](#logging-and-debugging)
- [License](#license)

---

## Features

- **gRPC API + optional dashboard:**  
  Notifications are sent via gRPC; the optional HTTP UI provides a dashboard and JSON endpoints for listing/rescheduling/cancelling queued notifications.

- **Email and SMS Notifications:**  
  - **Email:** Delivered via SMTP using the credentials you configure for your preferred mail provider.
  - **SMS:** Delivered using Twilio’s REST API.
- **Email Attachments:**  
  Attach up to **10 files** (5 MiB each, 25 MiB aggregate) to email notifications. Attachments are persisted so scheduled or retried jobs keep their payloads, and both the server and CLI bump the gRPC message size limit to 32 MiB so the larger payloads are accepted end-to-end.

- **Scheduled Delivery:**  
  Clients can provide an optional `scheduled_time` to defer dispatch until a specific timestamp. The background worker releases the notification when the scheduled time arrives.

- **Persistent Storage:**  
  Uses SQLite with GORM to store notifications and track their statuses.

- **Background Worker:**  
  Processes queued or failed notifications and retries them with exponential backoff.

- **Reusable Scheduler Package:**  
  The retry worker is built on `github.com/tyemirov/utils/scheduler`, exposing repository and dispatcher interfaces so other binaries can embed the same persistence-agnostic scheduler without reimplementing the ticker, backoff, or status bookkeeping logic.

- **Structured Logging:**  
  Uses Go’s `slog` package for structured logging with configurable levels.

- **Bearer Token Authentication:**  
  Secure access to the gRPC endpoints via a bearer token.

---

## Requirements

- **Go 1.21+** (tested with Go 1.24)
- An SMTP-compatible service account (any provider that supports standard SMTP)
- A Twilio account for SMS notifications (if needed)
- SQLite (or any GORM‑compatible database)

---

## Installation

Clone the repository and navigate to the project directory:

```bash
git clone https://github.com/tyemirov/pinguin.git
cd pinguin
```

Install dependencies:

```bash
go mod tidy
```

Build the Pinguin server:

```bash
go build -o pinguin ./cmd/server
```

---

## Configuration

Pinguin loads settings from `configs/config.yml` (override with `PINGUIN_CONFIG_PATH`). The YAML supports `${VAR}` expansion so you can keep secrets in your shell or `.env` file instead of the repository. A minimal example:

```yaml
server:
  databasePath: ${DATABASE_PATH}
  masterEncryptionKey: ${MASTER_ENCRYPTION_KEY}
tenants:
  - id: tenant-local
    displayName: Local Sandbox
    domains: [${TENANT_LOCAL_DOMAIN_PRIMARY}, ${TENANT_LOCAL_DOMAIN_SECONDARY}]
    admins:
      - ${TENANT_LOCAL_ADMIN_EMAIL}
      - ${TENANT_LOCAL_ADMIN_EMAIL_2}
    identity:
      googleClientId: ${TENANT_LOCAL_GOOGLE_CLIENT_ID}
      tauthBaseUrl: ${TENANT_LOCAL_TAUTH_BASE_URL}
    emailProfile:
      host: ${TENANT_LOCAL_SMTP_HOST}
      port: ${TENANT_LOCAL_SMTP_PORT}
      username: ${TENANT_LOCAL_SMTP_USERNAME}
      password: ${TENANT_LOCAL_SMTP_PASSWORD}
      fromAddress: ${TENANT_LOCAL_FROM_EMAIL}
    smsProfile:
      accountSid: ${TWILIO_ACCOUNT_SID}
      authToken: ${TWILIO_AUTH_TOKEN}
      fromNumber: ${TWILIO_FROM_NUMBER}
```

Export the referenced environment variables before starting the server. The default config references or sets the following keys:

- See `.env.pinguin.example` for a full list of variables to seed your environment when using the default config template.
- **PINGUIN_CONFIG_PATH:**  
  Optional override for the service configuration file (defaults to `configs/config.yml`).

- **DATABASE_PATH:**  
  Path to the SQLite database file (e.g., `app.db`).

- **LOG_LEVEL:**  
  Logging level. Possible values: `DEBUG`, `INFO`, `WARN`, `ERROR`.

- **GRPC_AUTH_TOKEN:**  
  Bearer token used for authenticating gRPC requests. All clients must supply this token.  
  Generate a value with `openssl rand -base64 32` (or an equivalent secure random command) and store it in a password manager.

- **CONNECTION_TIMEOUT_SEC:**  
  Number of seconds to wait when establishing outbound SMTP/Twilio connections. A value of `5` seconds works well for most deployments.

- **OPERATION_TIMEOUT_SEC:**  
  Maximum number of seconds to wait for a send attempt before treating it as failed. Set this to `30` seconds unless your provider requires longer operations.
- **HTTP_LISTEN_ADDR:**  
  Address used by the Gin HTTP server that exposes runtime config and the JSON `/api/*` endpoints (e.g. `:8080`). The HTTP stack no longer serves static assets directly—use a separate host such as GitHub Pages at `https://pinguin.mprlab.com` (production) or ghttp (`http://localhost:4173`) for `/web`.
- **HTTP_ALLOWED_ORIGINS:**  
  Comma-separated list of origins allowed to call the JSON API when running cross-origin (leave empty to allow same-origin only). The docker-compose workflow serves the UI via ghttp on `http://localhost:4173`, and production uses `https://pinguin.mprlab.com`, so include the relevant UI origins here.
- **DISABLE_WEB_INTERFACE:**  
  Set to `true`, `1`, `yes`, or `on` (or start the server with `--disable-web-interface`) to skip booting the Gin/HTML stack entirely. When disabled, Pinguin runs the gRPC service only and skips Google Identity/TAuth/HTTP configuration checks, which is useful for backends that never expose the dashboard.
- **MASTER_ENCRYPTION_KEY:**  
  Hex-encoded 32-byte key used to encrypt SMTP/Twilio secrets stored in the tenant config. Generate one with `openssl rand -hex 32` and keep it secret.
- **TAuth CORS allowlist:**  
  When you serve the UI from a different origin (ghttp on `http://localhost:4173`, GitHub Pages on `https://pinguin.mprlab.com`, a CDN, etc.), TAuth must enable CORS and allow both the UI origin *and* `https://accounts.google.com`. Google Identity Services performs the nonce/login exchange from the `accounts.google.com` origin, so omitting it results in `auth.login.nonce_mismatch` errors. The sample `.env.tauth.example` includes `APP_CORS_ALLOWED_ORIGINS="http://localhost:4173,https://accounts.google.com"` for this reason—extend the list with any additional UI origins you deploy.
- **Front-end TAuth config:**  
  The web bundle reads `/js/tauth-config.js` (see the file for the default values) to learn the TAuth base URL + Google client ID, and it now defaults runtime config + API calls to `https://pinguin-api.mprlab.com` when served from `.mprlab.com`. Update that file—or serve a different version per environment—to point the UI at your TAuth deployment; `/js/tauth-helper.js` uses the same values to load `tauth.js` before the `mpr-ui` bundle runs. Pinguin itself does not need to know this URL; only the shared signing key matters to the backend.
- **Web authentication flow:**  
  The browser UI relies on `<mpr-header>` and `<mpr-login-button>` from the `mpr-ui` package. Both components expect a Google OAuth Web Client ID (`google-site-id`) plus the TAuth endpoints noted above; `tauth.js` is loaded ahead of `mpr-ui` via `/js/tauth-helper.js`, and the app listens for `mpr-ui:auth:*` events to drive redirects and profile state. Update the attributes in `web/index.html` / `web/dashboard.html` when deploying to a new TAuth instance. See `docs/mprui-integration-guide.md` for the header wiring details and `docs/tauth-usage.md` for the TAuth helper/nonce contract.
- **Google Identity Client ID:**  
  The default Google OAuth client ID lives alongside the TAuth config in `web/js/tauth-config.js`. Update that file (and your TAuth environment) when running against a different Google project; the backend only needs the shared signing key.
- **TAUTH_SIGNING_KEY:**  
  HS256 signing key shared with the TAuth deployment. Used to validate the `app_session` cookie.
- **TAUTH_ISSUER:**  
  Expected JWT issuer written by TAuth (usually `tauth`).
- **TAUTH_COOKIE_NAME:**  
  Optional override for the session cookie name. Defaults to `app_session`.

- **MAX_RETRIES:**  
  Maximum number of times the background worker will retry sending a failed notification.

- **RETRY_INTERVAL_SEC:**  
  Base interval (in seconds) between retry scans. The actual backoff is exponential.

- **SMTP_USERNAME:**  
  SMTP username provided by your email service. Some providers require the full email address.

- **SMTP_PASSWORD:**  
  SMTP password or application-specific password issued by your provider.

- **FROM_EMAIL:**  
  The email address from which notifications are sent. This must be a verified sender with your SMTP provider.

- **SMTP_HOST:**  
  The hostname of the SMTP server (e.g., `smtp.yourdomain.com`).

- **SMTP_PORT:**  
  The SMTP port. Use `587` for STARTTLS or `465` for implicit TLS; the service will initiate TLS automatically when you specify `465`.

- **TWILIO_ACCOUNT_SID:**  
  Your Twilio Account SID, used for sending SMS messages.

- **TWILIO_AUTH_TOKEN:**  
  Your Twilio Auth Token.

- **TWILIO_FROM_NUMBER:**  
  The phone number (in E.164 format) from which SMS messages are sent.

  When any of the Twilio variables are omitted, the server starts with SMS delivery disabled and logs a warning that text notifications are unavailable.

### Tenant configuration (single YAML)

Pinguin keeps all configuration—including tenants—in a single YAML file (`configs/config.yml` by default). The `tenants` section defines which tenants exist, which domains map to each tenant, who can access the web UI, and what delivery credentials each tenant uses.

`tenants[].status` is not supported. Use `tenants[].enabled: true|false`.

Example (inline tenants):

```yaml
tenants:
  - id: tenant-acme
    displayName: Acme Corp
    supportEmail: support@acme.example
    enabled: true
    domains:
      - acme.example
      - portal.acme.example
    admins:
      - admin@acme.example
      - viewer@acme.example
    identity:
      googleClientId: google-client-id.apps.googleusercontent.com
      tauthBaseUrl: https://auth.acme.example
    emailProfile:
      host: smtp.acme.example
      port: 587
      username: smtp-user
      password: smtp-password
      fromAddress: noreply@acme.example
    smsProfile:
      accountSid: ACxxxxxxxx
      authToken: twilio-secret
      fromNumber: "+12015550123"
```

See `configs/config.yml` for a ready-to-use sample. `MASTER_ENCRYPTION_KEY` encrypts tenant SMTP/Twilio secrets at rest in SQLite.

#### Tenant keys

- `tenants`: list of tenant objects. Must contain at least one enabled tenant (`enabled: true`) or the server exits during startup.
- `tenants[].id` (string, required): stable tenant identifier.
  - Used by gRPC callers (`tenant_id`) and as the database partition key.
  - Avoid leaving it empty: an empty id is auto-generated during bootstrap and will drift between runs.
- `tenants[].enabled` (bool, optional): whether the tenant is enabled.
  - `true` → persisted as tenant status `active`.
  - `false` → persisted as tenant status `suspended`.
  - Defaults to `true` when omitted.
- `tenants[].displayName` (string, required): tenant name shown in the UI (e.g. the header label).
- `tenants[].supportEmail` (string, optional): tenant support contact (reserved for future use in UI/templates).
- `tenants[].domains` (list of strings, required): hostnames that map HTTP requests to this tenant.
  - The first domain is treated as the tenant’s default domain.
  - Matching is case-insensitive; ports are ignored (e.g. `localhost:8080` matches `localhost`).
- `tenants[].admins` (list of emails):
  - Required when `web.enabled: true`.
  - These emails are allowed to access the HTTP API after TAuth session validation.
- `tenants[].identity`:
  - Required when `web.enabled: true`.
  - `googleClientId` (string): Google OAuth client id for the tenant.
  - `tauthBaseUrl` (string): base URL for the tenant’s TAuth instance (used by the UI header).
- `tenants[].emailProfile` (required): tenant SMTP settings.
  - `host` (string), `port` (int), `username` (string), `password` (string), `fromAddress` (string).
  - `username` and `password` are encrypted with `MASTER_ENCRYPTION_KEY` before storing in SQLite.
- `tenants[].smsProfile` (optional): tenant Twilio settings.
  - If omitted, SMS delivery is disabled for that tenant.
  - `accountSid` and `authToken` are encrypted with `MASTER_ENCRYPTION_KEY`; `fromNumber` is stored as-is.

Example `.env` file:

```bash
DATABASE_PATH=app.db
LOG_LEVEL=DEBUG
GRPC_AUTH_TOKEN=my-secret-token
MAX_RETRIES=3
RETRY_INTERVAL_SEC=30
CONNECTION_TIMEOUT_SEC=5
OPERATION_TIMEOUT_SEC=30

SMTP_USERNAME=apikey
SMTP_PASSWORD=super-secret-password
FROM_EMAIL=support@yourdomain.com
SMTP_HOST=smtp.yourdomain.com
SMTP_PORT=587

TWILIO_ACCOUNT_SID=ACxxxxxxxxxxxx
TWILIO_AUTH_TOKEN=yyyyyyyyyyyyyy
TWILIO_FROM_NUMBER=+12015550123
```

For a deeper walkthrough of the SMTP delivery pipeline, see [`docs/smtp_delivery_plan.md`](docs/smtp_delivery_plan.md). A multi-tenant expansion roadmap now lives in [`docs/multitenancy-plan.md`](docs/multitenancy-plan.md) and outlines the schema, API, and operational work required to host multiple clients from different domains.

Load the environment variables:

```bash
export $(cat .env | xargs)
```

### Docker Compose deployment

The repository ships with `docker-compose.yaml` to run Pinguin alongside TAuth and a static file server (ghttp). The stack exposes:

- gRPC: `localhost:50051`
- HTTP API: `http://localhost:8080`
- TAuth: `http://localhost:8081`
- Front-end bundle via ghttp: `http://localhost:4173`

Open `http://localhost:4173` in your browser for the landing/dashboard UI. The HTTP API on `http://localhost:8080` remains available for CLI/grpcurl clients, but browsers should never point to that port directly.

The Pinguin Docker image declares `/web` as a separate volume for the UI bundle; the compose workflow mounts the `pinguin-web` volume (bound to `./web`) at `/web` for you.

1. Copy the sample environment files and update the placeholders. **Use the same signing key in both files** so TAuth and Pinguin agree on JWT validation.

   ```bash
   cp .env.pinguin.example .env.pinguin
   cp .env.tauth.example .env.tauth
   ${EDITOR:-vi} .env.pinguin .env.tauth
   ```

   - `.env.pinguin` configures the environment variables referenced by `configs/config.yml` (including `PINGUIN_CONFIG_PATH=/configs/config.yml`, tenant domains/admins, SMTP/Twilio credentials, and `TAUTH_SIGNING_KEY`).
   - If `GET http://localhost:8080/runtime-config` returns `{"error":"tenant_not_found"}`, the tenant domain env vars (`TENANT_LOCAL_DOMAIN_PRIMARY` / `TENANT_LOCAL_DOMAIN_SECONDARY`) are missing/mismatched.
   - `.env.tauth` configures the Google OAuth client, signing key, and CORS settings for local development. In both compose profiles, these values are expanded into `configs/tauth/config.yaml` and passed to TAuth via `TAUTH_CONFIG_FILE`.
   - Keep `TAUTH_SIGNING_KEY` (Pinguin) identical to `APP_JWT_SIGNING_KEY` (TAuth) so cookie validation succeeds.
   - `configs/config.yml` controls the Pinguin web allowlist (`web.allowedOrigins`); keep `http://localhost:4173` there when using ghttp.
   - Match the same UI origin in `.env.tauth` via `APP_CORS_ALLOWED_ORIGINS` so the auth endpoints accept browser requests (use `http://localhost:4173` for the default setup, plus `https://accounts.google.com`).

2. Build and start the stack (this creates the named Docker volume `pinguin-data` automatically). Use the `dev` profile to build Pinguin from the local Dockerfile:

   ```bash
   docker compose --profile dev up --build
   ```

   To pull the prebuilt Pinguin + ghttp images from GHCR, start the `docker` profile (TAuth still builds locally to load `configs/tauth/config.yaml`):

   ```bash
   docker compose --profile docker up -d
   ```

   Pinguin writes its SQLite file to the Docker-managed volume, validates browser sessions issued by the colocated TAuth instance, and exposes the HTTP API on port 8080. The static landing/dashboard bundle is served by ghttp on `http://localhost:4173`.
   Note: TAuth currently builds from a pinned upstream commit in both profiles so it can consume `configs/tauth/config.yaml`.

3. Stop the stack when you are finished (use the same profile you started):

   ```bash
   docker compose --profile dev down
   ```

To inspect the persisted database file later, run:

```bash
docker volume inspect pinguin-data
```

### Docker quickstart (full stack)

1. Copy the sample env files (one command per file so you can edit secrets immediately):

   ```bash
   timeout -k 5s -s SIGKILL 5s cp .env.pinguin.example .env.pinguin
   timeout -k 5s -s SIGKILL 5s cp .env.tauth.example .env.tauth
   ```

2. Edit `.env.pinguin` (SMTP/Twilio + shared signing key) and `.env.tauth` (Google client ID + the same signing key + `APP_CORS_ALLOWED_ORIGINS=["http://localhost:4173"]`).
3. Start the orchestration with the `dev` profile (which builds Pinguin locally):

   ```bash
   docker compose --profile dev up --build
   ```

   To run the prebuilt Pinguin + ghttp containers from GHCR instead, run `docker compose --profile docker up -d` (TAuth still builds locally).

   - gRPC server → `localhost:50051`
   - HTTP API → `http://localhost:8080`
   - TAuth → `http://localhost:8081`
   - UI (landing + dashboard) → `http://localhost:4173`

4. Visit `http://localhost:4173` in your browser, sign in via Google/TAuth, and interact with the dashboard (the UI automatically talks to the API on port 8080).
5. When finished, stop the stack (match the profile you started):

   ```bash
   docker compose --profile dev down
   ```

---

## Running the Server

Start the Pinguin gRPC server by running the built executable:

```bash
./pinguin
```

During development you can also execute it directly without building first:

```bash
go run ./cmd/server
# or simply
go run ./...
```

By default, the server listens on port `50051`. The server initializes the SQLite database, starts the background retry worker, and registers the gRPC NotificationService with bearer token authentication.

---

## Using the gRPC API

### Pinguin CLI

The repository includes an interactive CLI at `cmd/client` built with Cobra and Viper. It lives alongside the server so you can build it directly from the repository root:

```bash
go build -o pinguin-cli ./cmd/client
# or run directly
go run ./cmd/client send --help
```

Configuration values are read from environment variables prefixed with `PINGUIN_`:

| Variable | Purpose | Default |
| --- | --- | --- |
| `PINGUIN_GRPC_SERVER_ADDR` | Target gRPC endpoint | `localhost:50051` |
| `PINGUIN_GRPC_AUTH_TOKEN` | Bearer token used for authentication | _required_ |
| `PINGUIN_TENANT_ID` | Tenant identifier for the authenticated user | _required_ |
| `PINGUIN_CONNECTION_TIMEOUT_SEC` | Dial timeout in seconds | `5` |
| `PINGUIN_OPERATION_TIMEOUT_SEC` | Per-command timeout in seconds | `30` |
| `PINGUIN_LOG_LEVEL` | CLI log level (`DEBUG`, `INFO`, `WARN`, `ERROR`) | `INFO` |

The CLI also accepts the unprefixed variants (`GRPC_SERVER_ADDR`, `GRPC_AUTH_TOKEN`, `TENANT_ID`, `CONNECTION_TIMEOUT_SEC`, `OPERATION_TIMEOUT_SEC`, `LOG_LEVEL`) which is handy for ad-hoc testing and CI scripts.

Example command that schedules an email:

```bash
PINGUIN_GRPC_AUTH_TOKEN=my-secret-token \
PINGUIN_TENANT_ID=tenant-acme \
./pinguin-cli send \
  --type email \
  --to someone@example.com \
  --subject "Meeting Reminder" \
  --message "See you at 10:00" \
  --scheduled-time "2025-01-02T15:04:05Z"
```

Attachments are added with the repeatable `--attachment` flag. Each value accepts either `path` or `path::content-type`. When the MIME type is omitted, the CLI infers it from the file extension (falling back to `application/octet-stream`).

```bash
PINGUIN_GRPC_AUTH_TOKEN=my-secret-token \
PINGUIN_TENANT_ID=tenant-acme \
./pinguin-cli send \
  --type email \
  --recipient someone@example.com \
  --subject "Weekly Report" \
  --message "See attached report." \
  --attachment /tmp/report.pdf \
  --attachment "/tmp/notes.txt::text/plain"
```

### Using grpcurl

You can also use [grpcurl](https://github.com/fullstorydev/grpcurl) to interact directly with the gRPC API. The canonical protobuf definition lives at `pkg/proto/pinguin.proto`. For example, to send an email notification:

```bash
grpcurl -d '{
  "notification_type": "EMAIL",
  "recipient": "someone@example.com",
  "subject": "Test Email",
  "message": "Hello from Pinguin!",
  "scheduled_time": "2024-05-03T17:00:00Z"
}' -H "Authorization: Bearer my-secret-token" localhost:50051 pinguin.NotificationService/SendNotification
```

To attach files, populate the repeated `attachments` field (protobuf encodes the `bytes` field as base64 in JSON):

```bash
grpcurl -d '{
  "notification_type": "EMAIL",
  "recipient": "someone@example.com",
  "subject": "Project Plan",
  "message": "See attached proposal.",
  "attachments": [
    {
      "filename": "proposal.pdf",
      "content_type": "application/pdf",
      "data": "JVBERi0xLjcKJc..."
    }
  ]
}' -H "Authorization: Bearer my-secret-token" localhost:50051 pinguin.NotificationService/SendNotification
```

To retrieve the status of a notification (replace `<notification_id>` with the actual ID):

```bash
grpcurl -d '{
  "notification_id": "<notification_id>"
}' -H "Authorization: Bearer my-secret-token" localhost:50051 pinguin.NotificationService/GetNotificationStatus
```

---

## End-to-End Flow

1. **Submission:**  
   A client submits a notification (email or SMS) via gRPC using the `SendNotification` RPC. The notification is stored in the SQLite database with a status of `queued`. If `scheduled_time` is in the future, the notification remains queued until the target time.

2. **Immediate Dispatch:**  
   The server attempts to dispatch the notification immediately:
    - **Email:** Sent via SMTP using the configured credentials. When you supply port `465`, Pinguin initiates the connection over TLS before issuing SMTP commands; otherwise it uses STARTTLS on demand.
    - **SMS:** Sent using Twilio’s REST API.

3. **Background Worker:**  
   A background worker periodically polls the database for notifications that are still queued or have failed and reattempts sending them with exponential backoff.

4. **Status Retrieval:**  
   Clients can query the notification’s status using the `GetNotificationStatus` RPC or the `/api/notifications` HTTP endpoint until the status changes to `sent`, `cancelled`, or `errored` (legacy `failed` values are still returned for historical rows).

---

## HTTP API

The gRPC server now ships with a sibling Gin HTTP server that:

- Serves runtime configuration (`/runtime-config`) and the REST-ish JSON `/api/*` endpoints the browser UI consumes. Static assets under `/web` are hosted separately (GitHub Pages at `https://pinguin.mprlab.com` in production; ghttp on `http://localhost:4173` during local dev).
- Validates every authenticated request by reading the TAuth `app_session` cookie (via `TAUTH_*` settings and the shared signing key).
- Exposes JSON endpoints for the UI:
  - `GET /api/notifications?status=queued&status=errored` – lists stored notifications filtered by status.
  - `PATCH /api/notifications/:id/schedule` – accepts `{"scheduled_time":"RFC3339"}` to move a queued notification.
  - `POST /api/notifications/:id/cancel` – cancels queued notifications so workers skip them.
  - `GET /healthz` – liveness probe (no auth required).

All endpoints emit structured JSON errors (`401` for auth failures, `400` for invalid payloads, `404` when a notification does not exist, `409` when edits are requested for non-queued notifications). CORS is enabled for the origins listed via `HTTP_ALLOWED_ORIGINS`, and credentials are required so the browser sends the TAuth cookie.

### Browser UI (beta)

- Static assets live under `/web` and are served by GitHub Pages at `https://pinguin.mprlab.com` in production, with ghttp on `http://localhost:4173` for local development (the Go HTTP server keeps `/api`/`/runtime-config` in this arrangement). `index.html` provides the marketing + Google Sign-In landing experience, and `dashboard.html` renders the authenticated notifications table.
- The UI follows AGENTS.md: Alpine components per section, mpr-ui header/footer, DOM-scoped events (`notifications:*`) for toasts + table refreshes, and all strings centralized in `js/constants.js`.
- `js/app.js` bootstraps Alpine, registers the UI components, and reacts to `mpr-ui:auth:*` events to sync profile state and guard routes. The header handles auth via `tauth.js` (loaded ahead of `mpr-ui` by `js/tauth-helper.js`), and components talk to `/api/notifications` via the shared `apiClient`.
- Authentication state is broadcast across tabs via TAuth’s `BroadcastChannel("auth")`, so signing out in one tab logs out the others automatically.
- Handy for local testing: start the Compose stack so ghttp (`http://localhost:4173`) serves the `/web` bundle while the Go server handles `/api`/`/runtime-config`, then visit the ghttp host to exercise the landing and dashboard flows without needing an external client.

### Front-End Tests (Playwright)

Install the Node tooling once:

```bash
npm install
npx playwright install --with-deps
```

Then execute the browser smoke tests (landing auth CTA, dashboard cancel/reschedule flows) with:

```bash
npm test
```

The Playwright harness spins up a lightweight local server that mocks the `/api/notifications` + TAuth endpoints so the UI can be exercised without external services.

---

## Logging and Debugging

- **Structured Logging:**  
  Pinguin uses Go’s `slog` package for structured logging. Set the logging level via the `LOG_LEVEL` environment variable.

- **Debug Output:**  
  When `LOG_LEVEL` is set to `DEBUG`, detailed messages (including SMTP debug output and fallback warnings) are logged. Sensitive data (such as API keys) is masked in the logs.

---

## License

This project is proprietary software. All rights reserved by Marco Polo Research Lab.  
See the [LICENSE](./LICENSE) file for details.
