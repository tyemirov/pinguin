# Pinguin Notification Service

Pinguin is a notification service written in Go. It exposes a gRPC interface for **email** and **SMS** notifications. The service uses SQLite with GORM for persistent storage. A background worker retries errored notifications with exponential backoff. Go's built-in `slog` package provides structured logging.

Pinguin also ships an optional HTTP and browser workspace for notification and SMTP relay operations. Set `web.enabled: false` to run gRPC-only.

---

## Table of Contents

- [Features](#features)
- [Compatibility Policy](#compatibility-policy)
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

- **gRPC API + optional browser workspace:**
  Notifications are sent via gRPC. The optional HTTP UI manages tenants, API keys, notifications, and tenant-owned SMTP resources.

- **Email and SMS Notifications:**  
  - **Email:** Delivered through SMTP with the credentials for your selected mail provider.
  - **SMS:** Delivered with Twilio's REST API.
- **Authenticated SMTP Submission:**
  Optionally accepts Gmail-compatible SMTP AUTH submissions for exact sender identities and relays the raw message through the SMTP submission relay profile.
- **Email Attachments:**  
  Attach up to **10 files** to email notifications. Each file can be 5 MiB, with a 25 MiB total limit. Pinguin stores attachments for scheduled and retry operations. The server and CLI use a 32 MiB gRPC message limit.

- **Scheduled Delivery:**  
  Clients can provide an optional `scheduled_time` to defer dispatch until a specific timestamp. The background worker releases the notification when the scheduled time arrives.

- **Persistent Storage:**  
  Uses SQLite with GORM to store notifications and track their statuses.

- **Background Worker:**  
  Processes queued or errored notifications and retries them with exponential backoff.

- **Reusable Scheduler Package:**  
  The retry worker uses `github.com/tyemirov/utils/scheduler`. Its repository and dispatcher interfaces let other binaries use the same scheduler.

- **Structured Logging:**  
  Uses Go’s `slog` package for structured logging with configurable levels.

- **Tenant API-Key Authentication:**
  Each tenant has one bearer API key that authenticates gRPC requests and defines their tenant context.

- **SMTP Send-As Identities:**
  Dashboard users can create, view, rotate, and delete one-address SMTP credentials for Gmail Send-As. SMTP identity passwords are stored encrypted at rest and can be reopened from the SMTP relay page.

---

## Compatibility Policy

Pinguin supports only the current product contract and current schema. There is no backward compatibility layer.

Legacy data, legacy schemas, deprecated config keys, old endpoints, historical users, and obsolete behavior are invalid state. The service deletes or rejects them rather than preserving them, translating them, or routing through fallback behavior. New work must keep a single runtime code path.

---

## Requirements

- **Go 1.21+** (tested with Go 1.24)
- An SMTP-compatible service account (any provider that supports standard SMTP)
- For SMTP submission: a TLS certificate for the public SMTP hostname, plus SPF/DKIM/DMARC authorization through the upstream SMTP provider
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

Pinguin loads service settings from `configs/config.yml` locally or `/config/config.yml` in a container. The YAML supports `${VAR}` expansion. Tenant definitions and the gRPC credential are database data and are not service configuration.

```yaml
server:
  databasePath: ${DATABASE_PATH}
  logLevel: ${LOG_LEVEL}
  maxRetries: ${MAX_RETRIES}
  retryIntervalSec: ${RETRY_INTERVAL_SEC}
  masterEncryptionKey: ${MASTER_ENCRYPTION_KEY}
  connectionTimeoutSec: ${CONNECTION_TIMEOUT_SEC}
  operationTimeoutSec: ${OPERATION_TIMEOUT_SEC}
  tauth:
    signingKey: ${TAUTH_SIGNING_KEY}
    cookieName: ${TAUTH_COOKIE_NAME}

web:
  enabled: true
  listenAddr: ${HTTP_LISTEN_ADDR}
  allowedOrigins:
    - ${HTTP_ALLOWED_ORIGIN1}
```

Export each referenced variable before server start. A missing placeholder is a startup error. Set an intentionally unused placeholder to an empty value.

The principal service inputs are:

- `DATABASE_PATH`: SQLite database path.
- `MASTER_ENCRYPTION_KEY`: Hex-encoded 32-byte key for tenant provider secrets and SMTP identity passwords.
- `TAUTH_SIGNING_KEY`: HS256 key that Pinguin uses to validate TAuth sessions.
- `TAUTH_COOKIE_NAME`: TAuth session-cookie name. It defaults to `app_session` when the web API is enabled.
- `HTTP_LISTEN_ADDR`: Gin API listen address.
- `HTTP_ALLOWED_ORIGIN1/2/3`: Browser origins that can send authenticated cross-origin requests.
- `HTTP_TRUSTED_PROXY1/2/3`: Proxy addresses or CIDR ranges that can supply forwarding headers.
- `MAX_RETRIES` and `RETRY_INTERVAL_SEC`: Notification retry limits and scan interval.
- `CONNECTION_TIMEOUT_SEC` and `OPERATION_TIMEOUT_SEC`: Provider connection and operation limits.

See `configs/.env.pinguin.example` for all shared SMTP listener and relay inputs. The tracked example values are documentation only.

### Managed tenants

An authenticated TAuth user creates a tenant from the **Tenants** workspace. Creation requires:

- A display name.
- One complete external email delivery profile: SMTP host, port, username, password, and from address.
- One client-generated API credential ID and digest. The browser generates the raw API key and shows it once.
- An optional complete Twilio profile for SMS delivery.

Pinguin creates a UUID tenant ID and stores the owner from the validated TAuth user ID. Each owner can list and change only their tenants. Tenant deletion is permanent and removes all tenant-owned records.

Each tenant has exactly one API credential. Rotation immediately replaces the old credential. Pinguin stores only the SHA-256 digest of the raw API secret.

SMTP sender domains, identities, credentials, and forwarding routes also belong to the selected tenant. Shared SMTP listeners and upstream relay profiles remain in YAML service configuration.

See the [managed tenant configuration contract](docs/multitenancy-plan.md) for the HTTP routes, storage rules, conversion runbook, and acceptance criteria. See the [SMTP delivery plan](docs/smtp_delivery_plan.md) for the SMTP pipeline.

### Authenticated SMTP submission for Gmail Send-As

Pinguin can expose a Gmail-compatible SMTP submission endpoint for outbound “send as” operations. A separate SMTP forwarding listener handles inbound fanout. Pinguin does not host mailboxes.

Set the `smtpSubmission` section in `configs/config.pinguin.yml`:

```yaml
smtpSubmission:
  enabled: true
  hostname: pinguin-api.mprlab.com
  listenAddr: :587
  tlsListenAddr:
  tlsCertPath:
  tlsKeyPath:
  publicPort: 465
  publicSecurityMode: ssl
  deliveryMode: direct
  maxMessageBytes: 26214400
  maxRecipients: 100
  allowInsecureAuth: true
```

Sender domains are not configured in YAML. An authenticated owner selects a tenant and adds a sender domain. The owner publishes the displayed DNS records and clicks **Check DNS**. Pinguin creates SMTP identities only under a verified tenant domain. In `deliveryMode: direct`, Pinguin accepts the authenticated submission. It sends the raw message to each recipient domain's MX hosts with the authenticated envelope sender. Pinguin does not provide DKIM signing, bounce processing, or mailbox hosting.

The gateway accepts public SMTPS on `pinguin-api.mprlab.com:465`. Caddy terminates TLS and sends the decrypted session to private port `587`. As a result, the production config leaves the Pinguin TLS fields empty and sets `allowInsecureAuth: true`. Publish only the gateway TLS listener.

The public SMTPS listener uses Caddy Layer 4 and does not use the shared HTTP request limit. Pinguin applies SMTP controls in the submission server. `server.operationTimeoutSec` controls idle command and data deadlines. Session limits apply globally and to each backend-visible remote host. Authentication failure limits apply to each credential username. Message limits apply to each SMTP identity. Defaults permit 200 global sessions and 20 sessions for each remote host. Defaults also permit five authentication failures per 10 minutes and 60 accepted messages per identity per hour.

If you still have a provider SMTP account, set `deliveryMode: upstream` and provide:

```yaml
  relay:
    host: smtp.upstream.example.com
    port: 587
    username: upstream-user
    password: upstream-password
```

SMTP relay workflow:

1. Sign in to Pinguin.
2. Open **SMTP relay**.
3. Add the sender domain, for example `acme.example`.
4. Publish the DNS records shown by Pinguin:
   - TXT `_pinguin-challenge.acme.example` with the displayed verification token.
   - Add the displayed `a:<smtp-host>` mechanism to the `acme.example` SPF record before its final `all` directive.
   - TXT `_dmarc.acme.example` with a DMARC policy such as `v=DMARC1; p=none`.
5. Click **Check DNS**. Pinguin enables SMTP identity creation when the domain status becomes **Verified**.
6. Create an identity such as `alice@acme.example`.
7. Use the settings in the Gmail SMTP settings dialog. Open **View password** to see the settings again.
   - SMTP server: `pinguin-api.mprlab.com`
   - Port: `465`
   - Security: SSL
   - Username/password: values generated by Pinguin
8. Use **Rotate credentials** in that modal when Gmail needs a new SMTP username and password.

Pinguin validates that the SMTP login, envelope sender, and RFC 5322 `From` header all match the exact identity before accepting a message for delivery.

### Inbound SMTP forwarding for shared addresses

Pinguin can expose a separate unauthenticated SMTP listener for shared-address fanout. This listener is not a mailbox. Pinguin accepts only active identities that have forwarding recipients. It immediately sends the raw message to each `forward_to` recipient through `smtpForwarding.relay`. Pinguin stores no message body. Forwarded copies keep the original headers and use the shared address as the outbound envelope sender.

Set the `smtpForwarding` section in `configs/config.pinguin.yml`:

```yaml
smtpForwarding:
  enabled: true
  hostname: mx.pinguin.mprlab.com
  listenAddr: :25
  maxMessageBytes: 26214400
  maxRecipients: 100
  relay:
    host: smtp-relay.example.com
    port: 587
    username: relay-user
    password: relay-password
```

Because forwarding routes are stored as SMTP identities, shared-address domains use the same verified sender-domain gate as outbound SMTP relay identities.

Shared addresses are dynamic data, not YAML routes. Create or edit them from the SMTP relay page or the authenticated API:

```json
{
  "email_address": "support@help.example.com",
  "forward_to": ["alice@example.com", "maria@example.com"]
}
```

Pinguin rejects identity creation or forwarding updates unless `forward_to` contains at least one valid email address.

The inbound listener accepts `MAIL FROM:<>` null reverse-path messages so DSNs and other auto-generated loop-safe mail can be forwarded to configured shared addresses.

Use a dedicated mail subdomain for customer DNS when possible:

```dns
help.example.com. MX 10 mx.pinguin.mprlab.com.
_dmarc.help.example.com. TXT "v=DMARC1; p=none; rua=mailto:dmarc@example.com"
```

MX records apply to an entire domain, not one address. If `example.com` points to Pinguin, configure each accepted `@example.com` address. For the first release, use an address such as `support@help.example.com`. Keep the current apex-domain MX records.

Forwarded copies use the shared address as the outbound envelope sender. SPF must authorize the relay in `smtpForwarding.relay`. Use the relay provider's documented SPF include or a Pinguin-provided `ip4:` or `ip6:` mechanism.

Domain setup verification:

1. Verify DNS:
   ```sh
   dig +short MX help.example.com
   dig +short TXT _dmarc.help.example.com
   ```
	   The MX answer must include `mx.pinguin.mprlab.com`.
	   If the customer publishes SPF for forwarded copies, also verify `dig +short TXT help.example.com` returns the relay-authorizing SPF value.
2. For authenticated SMTP relay, click **Check DNS** on the SMTP relay page. Pinguin checks ownership, SPF, and DMARC records. Pinguin then updates the domain state.
3. Verify configuration:
   ```sh
   pinguin-doctor configs/config.pinguin.yml --expand-env
   ```
3. Send an external SMTP test to `support@help.example.com` and confirm that each configured forwarding recipient receives a copy.

The gateway accepts public MX traffic on `mx.pinguin.mprlab.com:25`. Caddy sends the SMTP session to Pinguin's private forwarding capability.

If forwarding through `smtpForwarding.relay` fails before Pinguin accepts `DATA`, Pinguin returns a temporary `451` SMTP response so the sender's mail server can retry. Pinguin does not provide IMAP, POP3, search, read/unread state, or retention for forwarded mail.

If your `config.yml` uses a companion `.env` file for placeholder values, load it before starting Pinguin so the YAML expansion has concrete values:

```bash
export $(cat .env | xargs)
```

### Docker Compose deployment

The repository ships with `docker-compose.yaml` to run Pinguin alongside TAuth and a static file server (ghttp). The stack exposes:

- gRPC: `localhost:50051`
- UI: `http://localhost:8080`
- HTTP API: `http://localhost:8081`
- SMTP forwarding: `localhost:8025` → container `:25`
- SMTP submission: `localhost:1587` → container `:587`, `localhost:8465` → container `:465`
- TAuth: `http://localhost:8082`

Open `http://localhost:8080` in your browser for the landing, Tenants, Event log, and SMTP relay pages. The HTTP API on `http://localhost:8081` serves the browser workspace. gRPC clients use port `50051`.

The Pinguin Docker image declares `/web` as a separate UI volume. Compose mounts the `pinguin-web` volume at `/web`.

### Release, publish, then deploy

GitHub Actions are disabled for Pinguin. `.mprlab/deploy/resources.yml` is the permanent versionless production declaration. It declares the server binary, container image, retained data, runtime capabilities, listeners, health check, Pages site, and TAuth tenant. `configs/config.production.yml` is the tracked production config template. The ignored mode-`0600` `.mprlab/deploy/.env` provides its private values. The Docker build context excludes this file.

The zero-argument `make release`, `make publish`, and `make deploy` commands delegate to the exact sibling `../mprlab-gateway`. The gateway validates and seals the selected manifest, publishes only the declared artifacts, and converges only Pinguin's declared runtime resources. This repository keeps `make up` and `make down` for local Compose development.

The first managed release requires the bounded [production data conversion](docs/multitenancy-plan.md#production-data-conversion) before deployment.

The conversion assigns all current production tenants to the TAuth account for `temirov@gmail.com`.

Prepare and publish the release in order:

```bash
docker login ghcr.io
make release
make publish
```

The gateway requires clean application and gateway checkouts at their exact remote revisions. `release` prepares the declared artifacts. `publish` publishes the sealed release. `deploy` converges the declared runtime resources. Use this operator command:

```bash
make deploy
```

Pinguin declares public SMTPS on `pinguin-api.mprlab.com:465` and public MX delivery on `mx.pinguin.mprlab.com:25`. The sibling gateway owns physical listeners, the shared network, Caddy reconciliation, and lifecycle receipts. Pinguin owns no separate Pages activation command.

1. Create private environment files. Use the tracked examples to review variable names. Add operational values to the private files. Use the same signing key in both files.

   ```bash
   install -m 0600 /dev/null configs/.env.pinguin
   install -m 0600 /dev/null configs/.env.tauth
   ${EDITOR:-vi} configs/.env.pinguin configs/.env.tauth
   ```

  - `configs/.env.pinguin` configures the database, master key, TAuth signing key, SMTP listeners, and SMTP relays.
   - `configs/.env.tauth` configures shared auth provider settings, signing key, and CORS settings for local development. Compose expands these values into `configs/config.tauth.yml` and passes that file to TAuth via `TAUTH_CONFIG_FILE`.
   - Keep `TAUTH_SIGNING_KEY` (Pinguin) identical to `TAUTH_TENANT_JWT_SIGNING_KEY_PINGUIN` (TAuth) so cookie validation succeeds.
   - `configs/config.pinguin.yml` controls `web.allowedOrigins`. Add `http://localhost:8080` for ghttp.
   - Match the same UI origin in `configs/.env.tauth` via `TAUTH_TENANT_ORIGIN_PINGUIN`/`TAUTH_CORS_ORIGIN_1` so the auth endpoints accept browser requests.

2. Build and start the stack (this creates the named Docker volume `pinguin-data` automatically). Use the `dev` profile to build Pinguin from the local Dockerfile:

   ```bash
   make up
   ```

   To pull the prebuilt Pinguin + ghttp images from GHCR, start the `docker` profile (TAuth still builds locally to load `configs/tauth/config.yaml`):

   ```bash
   COMPOSE_PROFILE=docker make up
   ```

   Pinguin writes its SQLite file to the Docker-managed volume. It validates browser sessions from the colocated TAuth instance. The HTTP API uses port 8081. ghttp serves the browser pages on `http://localhost:8080`.

3. Stop the stack when you are finished (use the same profile you started):

   ```bash
   make down
   ```

To inspect the persisted database file later, run:

```bash
docker volume inspect pinguin-data
```

### Docker quickstart (full stack)

1. Create the private env files explicitly:

   ```bash
   install -m 0600 /dev/null configs/.env.pinguin
   install -m 0600 /dev/null configs/.env.tauth
   ```

   Use the tracked examples to review variable names. Add operational values to private files.

2. Edit `configs/.env.pinguin` (SMTP/Twilio + shared signing key) and `configs/.env.tauth` (shared-shell auth settings + the same signing key + `TAUTH_TENANT_ORIGIN_PINGUIN=http://localhost:8080`).
3. Start the orchestration with the `dev` profile (which builds Pinguin locally):

   ```bash
   make up
   ```

   To run the prebuilt Pinguin + ghttp containers from GHCR instead, run `COMPOSE_PROFILE=docker make up` (TAuth still builds locally).

   - gRPC server → `localhost:50051`
   - UI (landing + Tenants + Event log + SMTP relay) → `http://localhost:8080`
   - HTTP API → `http://localhost:8081`
   - TAuth → `http://localhost:8082`

4. Visit `http://localhost:8080`, sign in through the shared shell, create a tenant, and then use Event log or SMTP relay. The UI uses the API on port 8081.
5. When finished, stop the stack (match the profile you started):

   ```bash
   make down
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

By default, the server listens on port `50051`. It creates the managed schema for an empty SQLite database. It rejects a non-empty obsolete schema. It starts the retry worker and registers tenant API-key authentication.

---

## Validating Configurations with `pinguin-doctor`

The `pinguin-doctor` command validates Pinguin configurations and reports issues. Use it to verify your configuration before deployment or to audit multiple project configurations:

```bash
# Build the doctor command
go build -o pinguin-doctor ./cmd/doctor

# Validate a single configuration
./pinguin-doctor config.yml

# Validate multiple configurations with cross-config checks
./pinguin-doctor config.yml other-config.yml --cross-validate

# Output as JSON for CI/CD pipelines
./pinguin-doctor config.yml --json

# Expand environment variables in config before validation
./pinguin-doctor config.yml --expand-env
```

The doctor command validates:

- Configuration file syntax and exact known fields.
- Server database, retry, timeout, logging, and master-key requirements.
- TAuth and web API settings when the web API is enabled.
- Shared SMTP submission and forwarding settings.
- Cross-config differences for the database path and shared listeners.

---

## Using the gRPC API

### Pinguin CLI

The repository includes an interactive CLI at `cmd/client` built with Cobra. It lives alongside the server so you can build it directly from the repository root:

```bash
go build -o pinguin-cli ./cmd/client
# or run directly
go run ./cmd/client send --help
```

Configuration values are passed explicitly as flags:

| Flag | Purpose | Default |
| --- | --- | --- |
| `--grpc-server-addr` | Target gRPC endpoint | `localhost:50051` |
| `--api-key` | Tenant API key used for authentication | _required_ |
| `--connection-timeout-sec` | Dial timeout in seconds | `5` |
| `--operation-timeout-sec` | Per-command timeout in seconds | `30` |
| `--log-level` | CLI log level (`DEBUG`, `INFO`, `WARN`, `ERROR`) | `INFO` |

Example command that schedules an email:

```bash
./pinguin-cli send \
  --api-key 'pgn_1_<credential-uuid>_<secret>' \
  --type email \
  --to someone@example.com \
  --subject "Meeting Reminder" \
  --message "See you at 10:00" \
  --scheduled-time "2025-01-02T15:04:05Z"
```

Attachments are added with the repeatable `--attachment` flag. Each value accepts either `path` or `path::content-type`. When the MIME type is omitted, the CLI infers it from the file extension (falling back to `application/octet-stream`).

```bash
./pinguin-cli send \
  --api-key 'pgn_1_<credential-uuid>_<secret>' \
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
}' -H "Authorization: Bearer pgn_1_<credential-uuid>_<secret>" localhost:50051 pinguin.NotificationService/SendNotification
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
}' -H "Authorization: Bearer pgn_1_<credential-uuid>_<secret>" localhost:50051 pinguin.NotificationService/SendNotification
```

To retrieve the status of a notification (replace `<notification_id>` with the actual ID):

```bash
grpcurl -d '{
  "notification_id": "<notification_id>"
}' -H "Authorization: Bearer pgn_1_<credential-uuid>_<secret>" localhost:50051 pinguin.NotificationService/GetNotificationStatus
```

---

## End-to-End Flow

1. **Submission:**  
   A client submits an email or SMS notification through the `SendNotification` RPC. Pinguin stores the notification with the `queued` status. If `scheduled_time` is in the future, the notification remains queued until that time.

2. **Immediate Dispatch:**  
   The server attempts to dispatch the notification immediately:
    - **Email:** Sent through SMTP with the tenant credentials. For port `465`, Pinguin starts the connection with TLS. For other ports, it uses STARTTLS.
    - **SMS:** Sent with Twilio's REST API.

3. **Background Worker:**  
   A background worker periodically polls the database for notifications that are still queued or errored and reattempts sending them with exponential backoff.

4. **Status Retrieval:**  
   Query the status with the `GetNotificationStatus` RPC or the selected tenant HTTP route. Continue until the status is `sent`, `cancelled`, or `errored`.

---

## HTTP API

The sibling Gin HTTP API:

- Serves tenant-independent runtime URLs at `GET /runtime-config`.
- Serves the unauthenticated readiness probe at `GET /healthz`.
- Validates the TAuth session cookie on each `/api` request.
- Uses `/api/tenants` for owner-scoped tenant list and create operations.
- Uses `/api/tenants/:tenant_id` and its nested profile, credential, notification, SMTP domain, and SMTP identity routes for all tenant resources.
- Requires `Idempotency-Key` for tenant creation and `If-Match` for versioned updates and deletion.
- Returns safe tenant and profile representations without provider secrets or raw API secrets.

Expected failures use `{ "error": { "code": "...", "message": "...", "request_id": "..." } }`. The API uses `401` for authentication failures and `404` for absent or foreign resources. It uses `409` for conflicts and `412` for stale versions. It uses `415` for unsupported media types and `422` for invalid domain values. CORS permits configured origins and credentials. HTTP logs honor forwarding headers only from configured trusted proxies.

### Browser UI (beta)

- Static assets live under `/web`. `index.html` provides sign-in. `tenants.html` manages tenant configuration and API keys. `event-log.html` manages notifications. `smtp-relay.html` manages tenant-owned SMTP resources.
- The UI uses compact MPR styles. It has dark surfaces, dense controls, status chips, and a narrow work surface.
- The shared header owns brand, Tenants, Event log, SMTP relay, Docs, authentication, profile, and theme controls. The active workspace destination uses `aria-current="page"`.
- The shared footer opens the seven-service **Built By Marco Polo Research Lab** catalog.
- Tenant management creates, updates, and permanently deletes a tenant. It also shows and rotates the one API key. Event log and SMTP relay require a selected tenant.
- Event log renders responsive notification records and queued-action dialogs. SMTP relay keeps sender domains collapsed, expands one DNS setup at a time, and separates identity creation from forwarding edits.
- The UI uses one Alpine component for each section. It uses the shared header, footer, events, and centralized strings.
- `web/js/runtime-config.js` buffers the latest public `mpr-ui` auth event before the application module loads. `js/core/sessionBridge.js` consumes that event and the optional public profile snapshot.
- `js/ui/tenantManagement.js` owns tenant operations. `js/ui/notificationsList.js` owns Event log behavior. `smtpDomains.js`, `smtpIdentities.js`, and `smtpCredentialsDialog.js` own SMTP workflows.
- The shared shell owns cross-tab authentication state. Pinguin consumes only the `mpr-ui` events and profile snapshot.
- For local testing, start the Compose stack and visit `http://localhost:8080`. The browser uses the Pinguin API on `http://localhost:8081`.

### Front-End Tests (Playwright)

Install the Node tooling once:

```bash
npm install
npx playwright install --with-deps
```

Then execute the browser suite for authentication, tenant lifecycle, responsive layout, Event log, SMTP relay, accessibility, and visual snapshots:

```bash
npm test
```

Visual baselines use one host-independent path with the `en-US` locale and `America/Los_Angeles` timezone. When an intentional visual change is accepted, refresh the Pinguin-owned snapshot baselines with `make test-frontend-update`, then run `make test-frontend` normally.

The Playwright harness starts a local server that provides the nested managed-tenant API and TAuth test boundary.

---

## Logging and Debugging

- **Structured Logging:**  
  Pinguin uses Go’s `slog` package for structured logging. Set the logging level with `server.logLevel` in `config.yml`.

- **Debug Output:**  
  When `server.logLevel` is `DEBUG`, Pinguin emits detailed diagnostic messages without raw API keys or provider secrets.

---

## License

This project is proprietary software. All rights reserved by Marco Polo Research Lab.  
See the [LICENSE](./LICENSE) file for details.
