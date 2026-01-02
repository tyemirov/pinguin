# Architecture

## System Overview
- Pinguin exposes two surfaces inside a single Go process: a gRPC notification service (port `50051`) and a Gin HTTP server (default `:8080`) that serves the REST-ish `/api` endpoints plus `/runtime-config`; static assets are hosted separately (GitHub Pages at `https://pinguin.mprlab.com` in production, or the ghttp container on `4173` for local dev).
- Docker Compose runs Pinguin alongside two support services:
  - **TAuth** (`:8081`) issues Google-backed sessions and signs `app_session` cookies.
  - **ghttp** (`:4173`) serves the static front-end when developing locally. Browsers always load the UI from this host; API traffic targets the Pinguin HTTP server.
- All persistent data lives in SQLite (`DATABASE_PATH` inside the container path `/app/data/pinguin.db`). Docker manages the volume so restarts keep the state.

## Authentication & Session Flow
- The browser UI never talks directly to Pinguin for authentication. Instead, the `<mpr-header>` component (from `mpr-ui`) coordinates Google Identity Services (GIS) and TAuth:
  1. `web/js/tauth-config.js` defines `PINGUIN_TAUTH_CONFIG` with the TAuth base URL and Google OAuth Web Client ID. Environment-specific variants of this file are shipped with deployments.
  2. `web/js/tauth-config-apply.js` copies those values into `window.__PINGUIN_CONFIG__`, sets the TAuth tenant id hints, and mirrors them onto every `<mpr-header>` instance (`google-site-id`, `tauth-url`, `tauth-login-path`, `tauth-logout-path`, `tauth-nonce-path`, `tauth-tenant-id`).
  3. `web/js/tauth-helper.js` loads `tauthBaseUrl/tauth.js` and then injects the `mpr-ui` bundle so the helper is ready before the header boots; `web/js/bootstrap.js` still fetches `/runtime-config` (from `pinguin-api.mprlab.com` when served from `.mprlab.com`) and then boots the app, which listens for `mpr-ui:auth:*` events.
  4. Successful sign-in yields an HttpOnly `app_session` cookie issued by TAuth; Pinguin validates that cookie on every `/api` request.
- The Go backend only needs the shared signing key (`TAUTH_SIGNING_KEY`), expected issuer, and optional cookie name override. There is **no** backend environment variable for the TAuth base URL or Google client ID.
- Admin gating:
  - Tenant admins are provisioned via the YAML tenant bootstrap (`tenants.tenants[].admins` in `configs/config.yml` by default).
  - `internal/httpapi/sessionMiddleware` rejects any request whose claims email is not on that list (HTTP 403).

## HTTP Server Responsibilities
- Routes defined in `internal/httpapi`:
  - `GET /runtime-config` → `{ apiBaseUrl: "<scheme>://<host>/api" }`. The UI uses this to derive absolute API URLs when loaded from GitHub Pages or ghttp.
  - `GET /healthz` – unauthenticated health probe.
  - Authenticated `/api/notifications` list/reschedule/cancel handlers guarded by the session middleware.
- Static assets do not come from the Gin stack anymore; ghttp serves `/web` while the Go HTTP server keeps `/api/**` and `/runtime-config` free of wildcard conflicts.
- CORS defaults:
  - When `HTTP_ALLOWED_ORIGINS` is empty, requests are treated as same-origin only (credentials disabled while `AllowAllOrigins=true`).
  - When provided, Gin restricts origins to the explicit allowlist and enables credentials so browsers can send TAuth cookies.

## Front-End Structure
- `/web` hosts an Alpine.js-based bundle that follows `AGENTS.md` guidelines:
  - `index.html` (landing page) and `dashboard.html` both import `mpr-ui` CSS via CDN, load the TAuth config + helper scripts (which inject the `mpr-ui` bundle), and bootstrap via `/js/app.js`.
  - `/js/bootstrap.js` centralizes runtime config resolution, GIS script injection, and lazy loading of the main app module.
  - Alpine factories live under `/js/ui/` and `/js/core/`. Notifications table logic dispatches DOM-scoped events for toast updates and API refreshes.
- GitHub Pages publishes `/web` via `.github/workflows/frontend-deploy.yml`, and `web/CNAME` maps the site to `pinguin.mprlab.com`.
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
- `.env.pinguin.example`: defines the environment variables referenced by `configs/config.yml` (database path, master encryption key, tenant bootstrap values, shared TAuth signing key, optional Twilio credentials).
- `.env.tauth.example`: holds the Google OAuth client ID, signing key, cookie domain, and CORS allowlist (must include the UI origin such as `http://localhost:4173` or `https://pinguin.mprlab.com`, plus `https://accounts.google.com`, so GIS nonce exchanges succeed).
- Front-end TAuth details (base URL + Google client ID) live in `web/js/tauth-config.js`; deployments swap this file per environment rather than injecting env vars at runtime.

## Docker Compose Topology
- `docker-compose.yaml` starts three services sharing the same network:
  - `pinguin`: Go server with `/web` bind-mounted for local iteration.
  - `tauth`: official `ghcr.io/tyemirov/tauth` image configured via `.env.tauth`.
  - `ghttp`: lightweight Python HTTP server serving `/web` on host port 4173.
- Ensure `.env.pinguin` and `.env.tauth` reuse the **same** signing key so cookie verification succeeds.
- Workflow:
  1. Copy the example env files and populate secrets.
  2. `docker compose up --build`.
  3. Visit `http://localhost:4173` for the landing page; the UI talks to `http://localhost:8080/api`, and TAuth runs on `http://localhost:8081`.
