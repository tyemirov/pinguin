# Architecture

## System Overview
- Pinguin exposes two surfaces inside a single Go process: a gRPC notification service (port `50051`) and a Gin HTTP server that serves the REST-ish `/api` endpoints plus `/runtime-config`; static assets are hosted separately (GitHub Pages at `https://pinguin.mprlab.com` in production, or the ghttp container on `8080` for local dev).
- When enabled, the same process also exposes SMTP submission listeners for Gmail-compatible Send-As clients. The SMTP listener authenticates exact sender identities, accepts raw RFC 5322 messages, and either relays them through the independent `smtpSubmission.relay` upstream profile or delivers them directly to recipient-domain MX hosts in `smtpSubmission.deliveryMode: direct`.
- When enabled, a separate inbound SMTP forwarding listener accepts unauthenticated MX delivery only for active SMTP identities with forwarding owners, forwards the raw accepted message through `smtpForwarding.relay`, accepts null reverse-path DSNs, and stores no mailbox state or message body.
- Docker Compose runs Pinguin alongside two support services:
  - **ghttp** (`:8080`) serves the static front-end when developing locally. Browsers always load the UI from this host; API traffic targets the Pinguin HTTP server on `:8081`.
  - **TAuth** (`:8082`) issues shared-shell sessions and signs `app_session` cookies.
- All persistent data lives in SQLite (`DATABASE_PATH` inside the container path `/app/data/pinguin.db`). Docker manages the volume so restarts keep the state.

## Authentication & Session Flow
- The browser UI never talks directly to Pinguin for authentication. Instead, the `<mpr-header>` component (from `mpr-ui`) coordinates provider-specific sign-in and TAuth:
  1. `<mpr-header data-config-url="/config-ui.yaml">` declares the shared shell contract in `index.html`, `event-log.html`, and `smtp-relay.html`.
  2. `mpr-ui-config.js` reads `web/config-ui.yaml`, selects the entry for the current page origin, applies shared-shell auth attributes to the header, and loads the `mpr-ui@latest` bundle from the bundle marker.
  3. `web/js/bootstrap.js` still fetches `/runtime-config` (from `pinguin-api.mprlab.com` when served from `.mprlab.com`) for the Pinguin API URL and tenant display metadata used by the app surface.
  4. `web/js/app.js` listens for `mpr-ui:auth:*` events and reads `MPRUI.resolveAuthProfileSnapshot` to sync profile state, drive redirects, and guard the authenticated pages.
  5. Successful sign-in yields an HttpOnly `app_session` cookie issued by TAuth; Pinguin validates that cookie on every `/api` request.
- The Go backend needs the shared signing key (`TAUTH_SIGNING_KEY`) and optional cookie name override. TAuth issuer is handled inside the session validator; Pinguin should not configure it directly.
- Pinguin reads TAuth session roles and configured tenant admin emails for browser workspace authorization. Users with the `admin` role or a configured `tenants[].admins` email can list, reschedule, and cancel notifications for any active tenant; non-admin users are limited to tenants whose configured domain matches the user's email domain.

## HTTP Server Responsibilities
- Routes defined in `internal/httpapi`:
- `GET /runtime-config` → `{ apiBaseUrl, eventLogUrl, smtpRelayUrl, tenant }`. The UI uses this to derive absolute API URLs, named page destinations, and tenant display metadata; shared-shell auth attributes come from `web/config-ui.yaml`, not Pinguin runtime metadata.
  - `GET /healthz` – unauthenticated health probe.
  - Authenticated `/api/notifications` list/reschedule/cancel handlers guarded by the session middleware.
  - `/api/notifications*` accepts an explicit `tenant_id`, but the handler authorizes that tenant against the authenticated session before resolving tenant runtime config.
  - Authenticated `/api/smtp-identities` list/create/view-credentials/rotate/delete handlers for exact SMTP submission sender credentials and dynamic inbound forwarding owners. Passwords are stored encrypted at rest under the server master encryption key; list responses remain secret-free, and the credentials endpoint returns the current password only to authorized admins.
- Static assets do not come from the Gin stack anymore; ghttp serves `/web` while the Go HTTP server keeps `/api/**` and `/runtime-config` free of wildcard conflicts.
- CORS defaults:
  - When `web.allowedOrigins` is empty, requests are treated as same-origin only (credentials disabled while `AllowAllOrigins=true`).
  - When provided, Gin restricts origins to the explicit allowlist and enables credentials so browsers can send TAuth cookies.
- HTTP request logs emit `source_ip`, `remote_addr`, and `user_agent`; `source_ip` uses forwarding headers only when the direct peer matches `web.trustedProxies`.

## Front-End Structure
- `/web` hosts an Alpine.js-based bundle that follows `AGENTS.md` guidelines:
  - `index.html` (landing page), `event-log.html`, and `smtp-relay.html` import `mpr-ui` CSS via CDN, load `/config-ui.yaml` through `mpr-ui-config.js`, initialize Pinguin runtime URLs through `/js/runtime-config.js`, and bootstrap via `/js/app.js`.
- `/js/bootstrap.js` centralizes runtime config resolution and lazy loading of the main app module.
  - Alpine factories live under `/js/ui/` and `/js/core/`. Notifications table logic dispatches DOM-scoped events for toast updates and API refreshes.
- GitHub Pages publishes `/web` through legacy branch-root publishing: gateway Ansible owns a Pinguin `pages_resources` entry that invokes `make pages-deploy`, which stages the static assets, writes a `pinguin-pages-build.json` source-commit marker, pushes the artifact to the `gh-pages` branch root, and triggers a GitHub Pages build. The Ansible resource then verifies the live marker against the deployed commit. `web/CNAME` maps the site to `pinguin.mprlab.com`, and `web/.nojekyll` keeps GitHub Pages from running Jekyll over the static bundle.
- `<mpr-header>` renders the shared sign-in control inside its own shadow tree. Playwright tests assert that the header shows exactly one visible shared sign-in button and that Pinguin runtime config contains no auth-provider metadata (`tests/e2e/landing.spec.ts`, `tests/e2e/utils.ts::expectSharedHeaderSignInButton`).

## Testing Strategy
- `npm test` runs Playwright against `tests/support/devServer.js`, which serves `/web` and mocks the `/api` + `/auth` endpoints.
- Key coverage includes:
  - Landing CTA + shared-shell happy path redirects.
  - Header contract coverage through `web/config-ui.yaml` and `mpr-ui-config.js`.
  - Dashboard guards (unauthenticated redirect, shared-shell logout).
  - Notification list, filtering, reschedule, cancel flows, and associated toasts.
- Go unit/integration tests cover configuration loading, HTTP handlers, domain scheduling logic, and the SQLite-backed scheduler worker (`go test ./...` gate).

## Configuration Files
- `configs/.env.pinguin.example`: defines the environment variables referenced by `configs/config.pinguin.yml` (database path, master encryption key, tenant bootstrap values, shared TAuth signing key, optional Twilio credentials).
- `smtpSubmission` controls optional SMTP submission listeners, user-owned sender-domain DNS verification, public Gmail-facing SMTP settings, and the selected delivery mode. Sender domains live in SQLite through the SMTP relay API instead of YAML. Production runs behind gateway-owned Caddy Layer 4 SMTPS termination by advertising edge `465` / `ssl` while the edge gateway forwards to `tutosh:8465`, Docker publishes that host port to Caddy `:465`, and Pinguin listens privately on plaintext SMTP inside the Docker network.
- `smtpForwarding` controls the optional MX-facing forwarding listener and outbound relay used to deliver copies. Dynamic shared addresses and forwarding owners live in SQLite as part of active SMTP identities and use the same verified sender-domain gate as outbound SMTP relay identities. Its public MX target is `mx.pinguin.mprlab.com`; the edge gateway forwards public `25` to `tutosh:8025`, Docker publishes that host port to Caddy `:25`, and customer onboarding should prefer a dedicated subdomain such as `help.customer.com` because MX records are domain-wide.
- Caddy's shared HTTP `rate_limit` snippet applies to the HTTPS API route, not to the Layer 4 SMTPS route. The SMTP submission server owns protocol-aware throttling: command/data deadlines, global and backend-visible per-remote-host session caps, SMTP AUTH failure windows keyed by credential username, and accepted-message windows keyed by SMTP identity.
- `configs/.env.tauth.example`: holds shared auth provider settings, signing key, cookie domain, and CORS allowlist for the colocated TAuth service.
- Front-end auth details for `mpr-ui` live in `web/config-ui.yaml`; Pinguin runtime metadata still comes from `/runtime-config` and does not include auth provider fields.

## Docker Compose Topology
- `docker-compose.yaml` starts three services sharing the same network:
  - `pinguin`: Go server with `/web` bind-mounted for local iteration.
  - `tauth`: official `ghcr.io/tyemirov/tauth` image configured via `.env.tauth`.
  - `ghttp`: lightweight HTTP server serving `/web` on host port 8080.
- Local SMTP testing mirrors the gateway high-port shape: `localhost:8025` maps to container `:25`, `localhost:1587` maps to container `:587`, and `localhost:8465` maps to container `:465`.
- Ensure `.env.pinguin` and `.env.tauth` reuse the **same** signing key so cookie verification succeeds.
- Workflow:
  1. Copy the example env files and populate secrets.
  2. `make up`.
  3. Visit `http://localhost:8080` for the landing page; the UI talks to `http://localhost:8081/api`, and TAuth runs on `http://localhost:8082`.
