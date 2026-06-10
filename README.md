# Pinguin Notification Service

Pinguin is a notification service written in Go. It exposes a gRPC interface for sending **email** and **SMS** notifications. The service uses SQLite (via GORM) for persistent storage and runs a background worker to retry errored notifications using exponential backoff. Structured logging is provided using Go’s built‑in `slog` package.

Pinguin also ships an optional HTTP + browser workspace for inspecting queued notifications and managing SMTP relay access; set `web.enabled: false` in `config.yml` to run gRPC-only.

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
  Notifications are sent via gRPC; the optional HTTP UI provides separate Event log and SMTP relay pages plus JSON endpoints for listing/rescheduling/cancelling queued notifications.

- **Email and SMS Notifications:**  
  - **Email:** Delivered via SMTP using the credentials you configure for your preferred mail provider.
  - **SMS:** Delivered using Twilio’s REST API.
- **Authenticated SMTP Submission:**
  Optionally accepts Gmail-compatible SMTP AUTH submissions for exact sender identities and relays the raw message through the SMTP submission relay profile.
- **Email Attachments:**  
  Attach up to **10 files** (5 MiB each, 25 MiB aggregate) to email notifications. Attachments are persisted so scheduled or retried jobs keep their payloads, and both the server and CLI bump the gRPC message size limit to 32 MiB so the larger payloads are accepted end-to-end.

- **Scheduled Delivery:**  
  Clients can provide an optional `scheduled_time` to defer dispatch until a specific timestamp. The background worker releases the notification when the scheduled time arrives.

- **Persistent Storage:**  
  Uses SQLite with GORM to store notifications and track their statuses.

- **Background Worker:**  
  Processes queued or errored notifications and retries them with exponential backoff.

- **Reusable Scheduler Package:**  
  The retry worker is built on `github.com/tyemirov/utils/scheduler`, exposing repository and dispatcher interfaces so other binaries can embed the same persistence-agnostic scheduler without reimplementing the ticker, backoff, or status bookkeeping logic.

- **Structured Logging:**  
  Uses Go’s `slog` package for structured logging with configurable levels.

- **Bearer Token Authentication:**  
  Secure access to the gRPC endpoints via a bearer token.

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

Pinguin loads settings from `configs/config.yml` locally or `/config/config.yml` in container deployments. The YAML supports `${VAR}` expansion so you can keep secrets in your shell or `.env` file instead of the repository. A minimal example:

```yaml
server:
  databasePath: ${DATABASE_PATH}
  masterEncryptionKey: ${MASTER_ENCRYPTION_KEY}
  tauth:
    signingKey: ${TAUTH_SIGNING_KEY}
    cookieName: app_session
tenants:
  - id: tenant-local
    displayName: Local Sandbox
    domains: [${TENANT_LOCAL_DOMAIN_PRIMARY}, ${TENANT_LOCAL_DOMAIN_SECONDARY}]
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

Export the referenced environment variables before starting the server only when `config.yml` contains `${VAR}` placeholders. Pinguin does not read these keys directly; `internal/config.LoadConfig` expands the YAML and then all runtime values come from the parsed config. Missing placeholder variables are startup errors; define intentionally unused optional placeholders as `KEY=` so the parser can distinguish blank values from absent configuration. The default config references or sets the following keys:

- See `configs/.env.pinguin.example` for a full list of variables to seed your environment when using the default config template.
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
  Maximum number of seconds to wait for a send attempt before treating it as errored. Set this to `30` seconds unless your provider requires longer operations.
- **HTTP_LISTEN_ADDR:**  
  Address used by the Gin HTTP server that exposes runtime config and the JSON `/api/*` endpoints (local Compose uses `:8081`). The HTTP stack no longer serves static assets directly—use a separate host such as GitHub Pages at `https://pinguin.mprlab.com` (production) or ghttp (`http://localhost:8080`) for `/web`.
- **HTTP_ALLOWED_ORIGIN1/2/3:**
  Origins allowed to call the JSON API when running cross-origin (leave empty to allow same-origin only). The docker-compose workflow serves the UI via ghttp on `http://localhost:8080`, and production uses `https://pinguin.mprlab.com`, so include the relevant UI origins here.
- **HTTP_TRUSTED_PROXY1/2/3:**
  Reverse proxy IP addresses or CIDR ranges whose `X-Forwarded-For` / `X-Real-IP` headers may determine `source_ip` in HTTP request logs. Leave these empty for direct local access; deployments behind Caddy or another trusted proxy should set the proxy peer address or network so spoofed client-supplied forwarding headers are ignored.
- **web.enabled:**
  Set to `false` in `config.yml` to skip booting the Gin/HTML stack entirely. When disabled, Pinguin runs the gRPC service only and skips browser HTTP configuration checks, which is useful for backends that never expose the browser workspace.
- **MASTER_ENCRYPTION_KEY:**  
  Hex-encoded 32-byte key used to encrypt SMTP/Twilio secrets stored in the tenant config. Generate one with `openssl rand -hex 32` and keep it secret.
- **TAuth CORS allowlist:**  
  When you serve the UI from a different origin (ghttp on `http://localhost:8080`, GitHub Pages on `https://pinguin.mprlab.com`, a CDN, etc.), TAuth must enable CORS for the UI origin and any provider-origin exceptions required by the shared shell. The sample `configs/.env.tauth.example` keeps those provider details in TAuth-owned configuration, not in Pinguin runtime config.
- **Shared-shell auth config:**
  Browser authentication is configured outside Pinguin's runtime config. The shared shell reads auth settings from `web/config-ui.yaml`, which is selected by the current page origin and consumed by `mpr-ui-config.js`. `web/js/runtime-config.js` only supplies the Pinguin runtime-config URL and API base for hosted deployments; it does not configure `<mpr-header>` auth.
- **Web authentication flow:**  
  The browser UI relies on `<mpr-header>` from the `mpr-ui` package. `mpr-ui-config.js` applies `/config-ui.yaml` to the header, loads the `mpr-ui@latest` bundle, and the app listens for `mpr-ui:auth:*` events plus `MPRUI.resolveAuthProfileSnapshot` to drive redirects and profile state. Pinguin does not load `tauth.js`, call TAuth profile endpoints, or expose auth-provider metadata from `/runtime-config`.
- **TAUTH_SIGNING_KEY:**  
  HS256 signing key shared with the TAuth deployment. Used to validate the `app_session` cookie.
- **Authorization:**  
  Pinguin reads TAuth `user_roles` from the signed session and configured `tenants[].admins` emails. Sessions with the `admin` role or a configured admin email can view and manage notifications for every tenant. Other authenticated sessions can only list, reschedule, or cancel notifications for tenants whose `tenants[].domains` entry matches the user's email domain.

- **MAX_RETRIES:**  
  Maximum number of times the background worker will retry sending an errored notification.

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

Pinguin keeps all configuration—including tenants—in a single YAML file (`configs/config.yml` by default). The `tenants` section defines which tenants exist, which domains map to each tenant, and what delivery credentials each tenant uses.

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
  - Bootstrap treats this list as the source of truth for tenant configuration. Removing a tenant from config removes its tenant, domain, admin, SMTP profile, and SMS profile records on the next startup.
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
  - The same normalized values authorize non-admin browser workspace users by email domain.
- `tenants[].admins` (list of strings, optional): email addresses that grant browser workspace admin access for the deployment.
  - Matching is case-insensitive.
  - Admin users can list every active tenant and manage global SMTP identities.
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

### Authenticated SMTP submission for Gmail Send-As

Pinguin can optionally expose a Gmail-compatible SMTP submission endpoint for outbound “send as” use cases. Inbound fanout is handled by the separate SMTP forwarding listener described below; mailbox hosting remains outside Pinguin.

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

Sender domains are not configured in YAML. Authenticated users add a sender domain in the SMTP relay page, publish the DNS records Pinguin shows, and click **Check DNS**. Pinguin marks the domain verified only when the ownership TXT, SPF authorization, and DMARC records match the displayed specification. Users can create SMTP relay identities only for their own verified domains. In `deliveryMode: direct`, Pinguin accepts the authenticated submission and delivers the raw message to each recipient domain's MX hosts using the authenticated identity as the envelope sender. DKIM signing, bounce processing, and mailbox hosting remain outside Pinguin.

The Marco Polo gateway deployment accepts public SMTPS on edge port `465`, forwards it to `tutosh:8465`, publishes that high host port to Caddy's container `:465`, and proxies the decrypted SMTP session to Pinguin's private `listenAddr` on the Docker network. That is why production direct-relay config leaves `tlsListenAddr`, `tlsCertPath`, and `tlsKeyPath` empty and sets `allowInsecureAuth: true`; do not publish the private Pinguin SMTP listener directly to the internet.

The public SMTPS listener does not inherit the shared HTTP Caddy request limiter because it is routed through Caddy Layer 4. Pinguin applies SMTP-aware controls in the submission server instead: idle command/data deadlines use `server.operationTimeoutSec`, concurrent sessions are capped globally and per backend-visible remote host, repeated SMTP AUTH failures are throttled by credential username, and accepted messages are rate-limited per SMTP identity. Built-in defaults allow 200 concurrent SMTP sessions globally, 20 per backend-visible remote host, 5 AUTH failures per credential username per 10 minutes, and 60 accepted messages per SMTP identity per hour.

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
   - TXT `acme.example` SPF with the displayed `a:<smtp-host>` mechanism, or add that mechanism to the existing SPF record before its final `all` directive.
   - TXT `_dmarc.acme.example` with a DMARC policy such as `v=DMARC1; p=none`.
5. Click **Check DNS**. Pinguin enables SMTP identity creation when the domain status becomes **Verified**.
6. Create an identity such as `alice@acme.example`.
7. Use the settings shown in the Gmail SMTP settings modal, or reopen them later with **View password**, in Gmail → Settings → Accounts → **Send mail as**:
   - SMTP server: `pinguin-api.mprlab.com`
   - Port: `465`
   - Security: SSL
   - Username/password: values generated by Pinguin
8. Use **Rotate credentials** in that modal when Gmail needs a new SMTP username and password.

Pinguin validates that the SMTP login, envelope sender, and RFC 5322 `From` header all match the exact identity before accepting a message for delivery.

### Inbound SMTP forwarding for shared addresses

Pinguin can also expose a separate unauthenticated inbound SMTP listener for shared-address fanout. This is not a mailbox: Pinguin accepts only active SMTP identities that have forwarding owners, immediately forwards the raw message to every configured `forward_to` recipient through `smtpForwarding.relay`, and stores no message body. Forwarded copies preserve the original message headers and use the shared address as the outbound SMTP envelope sender.

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

Customer DNS should use a dedicated mail subdomain whenever possible:

```dns
help.example.com. MX 10 mx.pinguin.mprlab.com.
_dmarc.help.example.com. TXT "v=DMARC1; p=none; rua=mailto:dmarc@example.com"
```

MX records apply to an entire domain, not a single address. If a customer points `example.com` MX at Pinguin, Pinguin becomes the inbound front door for all `@example.com` mail and must be configured for every accepted address. For a first rollout, prefer addresses like `support@help.example.com` over changing the apex domain's existing Google Workspace, Zoho, or Mailgun MX records.

Forwarded copies use the shared address as the outbound envelope sender, so SPF must authorize the actual relay configured in `smtpForwarding.relay`. Do not publish a Pinguin SPF include unless Pinguin operators have first published that TXT record; use the relay provider's documented SPF include or an explicit Pinguin-provided `ip4:`/`ip6:` mechanism instead.

Domain setup verification:

1. Verify DNS:
   ```sh
   dig +short MX help.example.com
   dig +short TXT _dmarc.help.example.com
   ```
	   The MX answer must include `mx.pinguin.mprlab.com`.
	   If the customer publishes SPF for forwarded copies, also verify `dig +short TXT help.example.com` returns the relay-authorizing SPF value.
2. For authenticated SMTP relay, prefer the SMTP relay page **Check DNS** button. It checks the exact ownership TXT, SPF mechanism, and DMARC policy Pinguin issued for that sender domain and updates the domain's verified state.
3. Verify configuration:
   ```sh
   pinguin-doctor configs/config.pinguin.yml --expand-env
   ```
3. Send an external SMTP test to `support@help.example.com` and confirm every configured forwarding owner receives a copy.

The Marco Polo gateway accepts public MX traffic on edge port `25`, forwards it to `tutosh:8025`, publishes that high host port to Caddy's container `:25`, and proxies the raw SMTP session to Pinguin's private forwarding listener.

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

Open `http://localhost:8080` in your browser for the landing page, Event log, and SMTP relay UI. The HTTP API on `http://localhost:8081` remains available for CLI/grpcurl clients, but browsers should use the UI port.

The Pinguin Docker image declares `/web` as a separate volume for the UI bundle; the compose workflow mounts the `pinguin-web` volume (bound to `./web`) at `/web` for you.

### Publish Docker, then deploy backend and Pages

GitHub Actions are disabled for Pinguin. Release, publish, and deploy are explicit local production operations from a clean local `master` branch that exactly matches `origin/master` with zero open pull requests. These commands print the branch and commit they verified before doing production work; any other branch, dirty worktree, local/remote mismatch, or open PR is a hard failure.

Use the publish target to run validation, then build and push the `linux/amd64,linux/arm64` Docker image manifest to GHCR:

```bash
docker login ghcr.io
make publish
```

Use the deploy target after `make publish` to deploy the backend through `mprlab-gateway`, publish the current `web/` assets to the legacy `gh-pages` branch root, trigger a GitHub Pages build, and verify the live Pages source marker matches the deployed commit. Production deployment is intentionally parameterless; the operator command is:

```bash
make deploy
```

`make publish` defaults to `ghcr.io/tyemirov/pinguin:latest` and `linux/amd64,linux/arm64`. `make deploy` defaults to the sibling `mprlab-gateway` checkout, the `tyemirov/pinguin` Pages repository, and the `gh-pages` branch. The deploy script verifies the production Git state, then verifies that the gateway checkout publishes Caddy's SMTP listeners on high host ports `8025` and `8465` before it runs `make -C ../mprlab-gateway deploy-pinguin` with this repo's `deploy/app.yml`. That app manifest owns the GitHub Pages resource that invokes Pinguin's `pages-deploy` target and verifies `pinguin-pages-build.json`, while gateway Ansible owns execution. After `make deploy`, configure the edge gateway to forward `25 -> tutosh:8025` and `465 -> tutosh:8465`; no Pinguin app port mapping is required. Override `DOCKER_IMAGE`, `DOCKER_TAG`, `PUBLISH_PLATFORMS`, `PAGES_REPOSITORY`, `PAGES_PUBLISH_REMOTE`, or `PAGES_PUBLISH_BRANCH` only for non-production targets. `gh` must be authenticated with repository and Pages access so deploy can verify open PRs and update/verify the legacy Pages source.

1. Copy the sample environment files and update the placeholders. **Use the same signing key in both files** so TAuth and Pinguin agree on JWT validation.

   ```bash
   cp configs/.env.pinguin.example configs/.env.pinguin
   cp configs/.env.tauth.example configs/.env.tauth
   ${EDITOR:-vi} configs/.env.pinguin configs/.env.tauth
   ```

  - `configs/.env.pinguin` configures the environment variables referenced by `configs/config.pinguin.yml` (including tenant domains, SMTP/Twilio credentials, and `TAUTH_SIGNING_KEY`).
   - If `GET http://localhost:8081/runtime-config` returns `{"error":"tenant_not_found"}`, the tenant domain env vars (`TENANT_LOCAL_DOMAIN_PRIMARY` / `TENANT_LOCAL_DOMAIN_SECONDARY`) are missing/mismatched.
   - `configs/.env.tauth` configures shared auth provider settings, signing key, and CORS settings for local development. Compose expands these values into `configs/config.tauth.yml` and passes that file to TAuth via `TAUTH_CONFIG_FILE`.
   - Keep `TAUTH_SIGNING_KEY` (Pinguin) identical to `TAUTH_TENANT_JWT_SIGNING_KEY_PINGUIN` (TAuth) so cookie validation succeeds.
   - `configs/config.pinguin.yml` controls the Pinguin web allowlist (`web.allowedOrigins`); keep `http://localhost:8080` there when using ghttp.
   - Match the same UI origin in `configs/.env.tauth` via `TAUTH_TENANT_ORIGIN_PINGUIN`/`TAUTH_CORS_ORIGIN_1` so the auth endpoints accept browser requests.

2. Build and start the stack (this creates the named Docker volume `pinguin-data` automatically). Use the `dev` profile to build Pinguin from the local Dockerfile:

   ```bash
   make up
   ```

   To pull the prebuilt Pinguin + ghttp images from GHCR, start the `docker` profile (TAuth still builds locally to load `configs/tauth/config.yaml`):

   ```bash
   COMPOSE_PROFILE=docker make up
   ```

   Pinguin writes its SQLite file to the Docker-managed volume, validates browser sessions issued by the colocated TAuth instance, and exposes the HTTP API on port 8081. The static landing, Event log, and SMTP relay bundle is served by ghttp on `http://localhost:8080`.

3. Stop the stack when you are finished (use the same profile you started):

   ```bash
   make down
   ```

To inspect the persisted database file later, run:

```bash
docker volume inspect pinguin-data
```

### Docker quickstart (full stack)

1. Copy the sample env files (one command per file so you can edit secrets immediately):

   ```bash
   timeout -k 5s -s SIGKILL 5s cp configs/.env.pinguin.example configs/.env.pinguin
   timeout -k 5s -s SIGKILL 5s cp configs/.env.tauth.example configs/.env.tauth
   ```

2. Edit `configs/.env.pinguin` (SMTP/Twilio + shared signing key) and `configs/.env.tauth` (shared-shell auth settings + the same signing key + `TAUTH_TENANT_ORIGIN_PINGUIN=http://localhost:8080`).
3. Start the orchestration with the `dev` profile (which builds Pinguin locally):

   ```bash
   make up
   ```

   To run the prebuilt Pinguin + ghttp containers from GHCR instead, run `COMPOSE_PROFILE=docker make up` (TAuth still builds locally).

   - gRPC server → `localhost:50051`
   - UI (landing + Event log + SMTP relay) → `http://localhost:8080`
   - HTTP API → `http://localhost:8081`
   - TAuth → `http://localhost:8082`

4. Visit `http://localhost:8080` in your browser, sign in through the shared shell, and use Event log or SMTP relay (the UI automatically talks to the API on port 8081).
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

By default, the server listens on port `50051`. The server initializes the SQLite database, starts the background retry worker, and registers the gRPC NotificationService with bearer token authentication.

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

The doctor command performs comprehensive validation including:
- Configuration file syntax and structure
- Server requirements (database path, gRPC auth token, encryption key)
- Web interface configuration (when enabled)
- Tenant requirements (domains, admins)
- Cross-config validation (conflicting domains)

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
| `--grpc-auth-token` | Bearer token used for authentication | _required_ |
| `--tenant-id` | Tenant identifier for the authenticated user | _required_ |
| `--connection-timeout-sec` | Dial timeout in seconds | `5` |
| `--operation-timeout-sec` | Per-command timeout in seconds | `30` |
| `--log-level` | CLI log level (`DEBUG`, `INFO`, `WARN`, `ERROR`) | `INFO` |

Example command that schedules an email:

```bash
./pinguin-cli send \
  --grpc-auth-token my-secret-token \
  --tenant-id tenant-acme \
  --type email \
  --to someone@example.com \
  --subject "Meeting Reminder" \
  --message "See you at 10:00" \
  --scheduled-time "2025-01-02T15:04:05Z"
```

Attachments are added with the repeatable `--attachment` flag. Each value accepts either `path` or `path::content-type`. When the MIME type is omitted, the CLI infers it from the file extension (falling back to `application/octet-stream`).

```bash
./pinguin-cli send \
  --grpc-auth-token my-secret-token \
  --tenant-id tenant-acme \
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
   A background worker periodically polls the database for notifications that are still queued or errored and reattempts sending them with exponential backoff.

4. **Status Retrieval:**  
   Clients can query the notification’s status using the `GetNotificationStatus` RPC or the `/api/notifications` HTTP endpoint until the status changes to `sent`, `cancelled`, or `errored`.

---

## HTTP API

The gRPC server now ships with a sibling Gin HTTP server that:

- Serves runtime configuration (`/runtime-config`) and the REST-ish JSON `/api/*` endpoints the browser UI consumes. Static assets under `/web` are hosted separately (GitHub Pages at `https://pinguin.mprlab.com` in production; ghttp on `http://localhost:8080` during local dev).
- Validates every authenticated request by reading the TAuth `app_session` cookie (via `TAUTH_*` settings and the shared signing key).
- Exposes JSON endpoints for the UI:
  - `GET /api/notifications?status=queued&status=errored` – lists stored notifications filtered by status.
  - `PATCH /api/notifications/:id/schedule` – accepts `{"scheduled_time":"RFC3339"}` to move a queued notification.
  - `POST /api/notifications/:id/cancel` – cancels queued notifications so workers skip them.
  - `GET /healthz` – liveness probe (no auth required).

All endpoints emit structured JSON errors (`401` for auth failures, `400` for invalid payloads, `404` when a notification does not exist, `409` when edits are requested for non-queued notifications). CORS is enabled for the origins listed via `HTTP_ALLOWED_ORIGIN1/2/3`, and credentials are required so the browser sends the TAuth cookie. HTTP request logs include `source_ip`, `remote_addr`, and `user_agent`; `source_ip` only honors forwarding headers from `HTTP_TRUSTED_PROXY1/2/3`.

### Browser UI (beta)

- Static assets live under `/web` and are served by GitHub Pages at `https://pinguin.mprlab.com` in production, with ghttp on `http://localhost:8080` for local development (the Go HTTP server keeps `/api`/`/runtime-config` on `http://localhost:8081` in this arrangement). `index.html` provides the sign-in landing experience, `event-log.html` renders notification delivery events, and `smtp-relay.html` renders SMTP relay identity management.
- The UI follows AGENTS.md: Alpine components per section, mpr-ui header/footer, DOM-scoped events (`notifications:*`) for toasts + table refreshes, and all strings centralized in `js/constants.js`.
- `js/app.js` bootstraps Alpine, registers the UI components, and reacts to `mpr-ui:auth:*` events plus the shared `MPRUI.resolveAuthProfileSnapshot` verifier to sync profile state and guard routes. The header handles auth through `/config-ui.yaml` plus `mpr-ui-config.js`, and components talk to `/api/notifications` via the shared `apiClient`.
- Cross-tab authentication state is owned by the shared shell; Pinguin consumes only the resulting `mpr-ui` events and profile snapshot.
- Handy for local testing: start the Compose stack so ghttp (`http://localhost:8080`) serves the `/web` bundle while the Go server handles `/api`/`/runtime-config` on `http://localhost:8081`, then visit the ghttp host to exercise the landing, Event log, and SMTP relay flows without needing an external client.

### Front-End Tests (Playwright)

Install the Node tooling once:

```bash
npm install
npx playwright install --with-deps
```

Then execute the browser smoke tests (landing auth CTA, Event log cancel/reschedule flows, and SMTP relay flows) with:

```bash
npm test
```

The Playwright harness spins up a lightweight local server that mocks the `/api/notifications` + TAuth endpoints so the UI can be exercised without external services.

---

## Logging and Debugging

- **Structured Logging:**  
  Pinguin uses Go’s `slog` package for structured logging. Set the logging level with `server.logLevel` in `config.yml`.

- **Debug Output:**  
  When `server.logLevel` resolves to `DEBUG`, detailed messages (including SMTP debug output and fallback warnings) are logged. Sensitive data (such as API keys) is masked in the logs.

---

## License

This project is proprietary software. All rights reserved by Marco Polo Research Lab.  
See the [LICENSE](./LICENSE) file for details.
