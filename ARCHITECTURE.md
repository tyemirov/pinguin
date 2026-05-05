# Architecture

## System Overview
- Pinguin exposes two surfaces inside a single Go process: a gRPC notification service (port `50051`) and a Gin HTTP server that serves the REST-ish `/api` endpoints plus `/runtime-config`; static assets are hosted separately (GitHub Pages at `https://pinguin.mprlab.com` in production, or the ghttp container on `8080` for local dev).
- When enabled, the same process also exposes SMTP submission listeners for Gmail-compatible Send-As clients. The SMTP listener authenticates exact sender identities, accepts raw RFC 5322 messages, and either relays them through the independent `smtpSubmission.relay` upstream profile or delivers them directly to recipient-domain MX hosts in `smtpSubmission.deliveryMode: direct`.
- Docker Compose runs Pinguin alongside two support services:
  - **ghttp** (`:8080`) serves the static front-end when developing locally. Browsers always load the UI from this host; API traffic targets the Pinguin HTTP server on `:8081`.
  - **TAuth** (`:8082`) issues Google-backed sessions and signs `app_session` cookies.
- All persistent data lives in SQLite (`DATABASE_PATH` inside the container path `/app/data/pinguin.db`). Docker manages the volume so restarts keep the state.

## Authentication & Session Flow
- The browser UI never talks directly to Pinguin for authentication. Instead, the `<mpr-header>` component (from `mpr-ui`) coordinates Google Identity Services (GIS) and TAuth:
  1. `<mpr-header data-config-url="/config-ui.yaml">` declares the shared shell contract in `index.html` and `dashboard.html`.
  2. `mpr-ui-config.js` reads `web/config-ui.yaml`, selects the entry for the current page origin, applies Google/TAuth attributes to the header, and loads the `mpr-ui@latest` bundle from the bundle marker.
  3. `web/js/bootstrap.js` still fetches `/runtime-config` (from `pinguin-api.mprlab.com` when served from `.mprlab.com`) for the Pinguin API URL and tenant display metadata used by the app surface.
  4. `web/js/app.js` listens for `mpr-ui:auth:*` events to sync profile state, drive redirects, and guard the dashboard.
  5. Successful sign-in yields an HttpOnly `app_session` cookie issued by TAuth; Pinguin validates that cookie on every `/api` request.
- The Go backend needs the shared signing key (`TAUTH_SIGNING_KEY`) and optional cookie name override. TAuth issuer is handled inside the session validator; Pinguin should not configure it directly.
- Pinguin reads TAuth session roles and configured tenant admin emails for dashboard authorization. Users with the `admin` role or a configured `tenants[].admins` email can list, reschedule, and cancel notifications for any active tenant; non-admin users are limited to tenants whose configured domain matches the user's email domain.

## HTTP Server Responsibilities
- Routes defined in `internal/httpapi`:
- `GET /runtime-config` → `{ apiBaseUrl, tauthBaseUrl, tauthTenantId, googleClientId, tenant }`. The UI uses this to derive absolute API URLs and tenant display metadata; `mpr-ui` auth attributes come from `web/config-ui.yaml`.
  - `GET /healthz` – unauthenticated health probe.
  - Authenticated `/api/notifications` list/reschedule/cancel handlers guarded by the session middleware.
  - `/api/notifications*` accepts an explicit `tenant_id`, but the handler authorizes that tenant against the authenticated session before resolving tenant runtime config.
  - Authenticated `/api/smtp-identities` list/create/rotate/delete handlers for exact SMTP submission sender credentials.
- Static assets do not come from the Gin stack anymore; ghttp serves `/web` while the Go HTTP server keeps `/api/**` and `/runtime-config` free of wildcard conflicts.
- CORS defaults:
  - When `HTTP_ALLOWED_ORIGINS` is empty, requests are treated as same-origin only (credentials disabled while `AllowAllOrigins=true`).
  - When provided, Gin restricts origins to the explicit allowlist and enables credentials so browsers can send TAuth cookies.

## Front-End Structure
- `/web` hosts an Alpine.js-based bundle that follows `AGENTS.md` guidelines:
  - `index.html` (landing page) and `dashboard.html` both import `mpr-ui` CSS via CDN, load `/config-ui.yaml` through `mpr-ui-config.js`, and bootstrap via `/js/app.js`.
- `/js/bootstrap.js` centralizes runtime config resolution, GIS script injection, and lazy loading of the main app module.
  - Alpine factories live under `/js/ui/` and `/js/core/`. Notifications table logic dispatches DOM-scoped events for toast updates and API refreshes.
- GitHub Pages publishes `/web` through legacy branch-root publishing: `make publish` stages the static assets and pushes them to the `gh-pages` branch root. `web/CNAME` maps the site to `pinguin.mprlab.com`, and `web/.nojekyll` keeps GitHub Pages from running Jekyll over the static bundle.
- `<mpr-header>` renders the Google Sign-In button inside its own shadow tree. Playwright tests assert that the header shows exactly one visible “Google” button so UI regressions are caught early (`tests/e2e/landing.spec.ts`, `tests/e2e/utils.ts::expectHeaderGoogleButton`).

## Testing Strategy
- `npm test` runs Playwright against `tests/support/devServer.js`, which serves `/web` and mocks the `/api` + `/auth` endpoints.
- Key coverage includes:
  - Landing CTA + GIS/TAuth happy path redirects.
  - Header attribute parity with runtime config (guarantees `<mpr-header>` points to the right TAuth base).
  - Dashboard guards (unauthenticated redirect, BroadcastChannel logout).
  - Notification list, filtering, reschedule, cancel flows, and associated toasts.
- Go unit/integration tests cover configuration loading, HTTP handlers, domain scheduling logic, and the SQLite-backed scheduler worker (`go test ./...` gate).

## Configuration Files
- `configs/.env.pinguin.example`: defines the environment variables referenced by `configs/config.pinguin.yml` (database path, master encryption key, tenant bootstrap values, shared TAuth signing key, optional Twilio credentials).
- `smtpSubmission` controls optional SMTP submission listeners, the global sender-domain allowlist, public Gmail-facing SMTP settings, and the selected delivery mode. Production can run behind Caddy Layer 4 SMTPS termination by advertising `publicPort: 465` / `publicSecurityMode: ssl` while Pinguin listens privately on plaintext SMTP inside the Docker network.
- `configs/.env.tauth.example`: holds the Google OAuth client ID, signing key, cookie domain, and CORS allowlist (must include the UI origin such as `http://localhost:8080` or `https://pinguin.mprlab.com`, plus `https://accounts.google.com`, so GIS nonce exchanges succeed).
- Front-end TAuth details for `mpr-ui` live in `web/config-ui.yaml`; Pinguin runtime metadata still comes from `/runtime-config`.

## Docker Compose Topology
- `docker-compose.yaml` starts three services sharing the same network:
  - `pinguin`: Go server with `/web` bind-mounted for local iteration.
  - `tauth`: official `ghcr.io/tyemirov/tauth` image configured via `.env.tauth`.
  - `ghttp`: lightweight HTTP server serving `/web` on host port 8080.
- Ensure `.env.pinguin` and `.env.tauth` reuse the **same** signing key so cookie verification succeeds.
- Workflow:
  1. Copy the example env files and populate secrets.
  2. `make up`.
  3. Visit `http://localhost:8080` for the landing page; the UI talks to `http://localhost:8081/api`, and TAuth runs on `http://localhost:8082`.
