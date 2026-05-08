# Issues

In this file the entries (issues) record newly discovered requests or changes, with their outcomes. No instructive content lives here. Read @NOTES.md for the process to follow when fixing issues.

Read @AGENTS.md, @ARCHITECTURE.md, @README.md, @issues.md/POLICY.md, @issues.md/PLANNING.md, @issues.md/NOTES.md, and @issues.md/ISSUES.md, following the internal links. Start working on open issues, prioritizing bug fixes. Work autonomously and stack up PRs.

## Features  (102–199)

- [x] [PG-106] Add shared-address inbound forwarding for SMTP fanout so configured addresses such as `support@help.customer.com` can accept inbound replies and immediately forward copies to all configured owners without storing mailbox state. Resolved with UI/API-driven SMTP identity forwarding owners, an unauthenticated inbound SMTP forwarding listener, immediate raw-message fanout through a relay, no mailbox persistence, DNS setup docs for `smtp.pinguin.mprlab.com`, doctor/config validation, and passing `make ci`.
- [x] [PG-104] Add an authenticated tenant switcher so Pinguin admins can choose a tenant and view that tenant's notification events from the dashboard. Resolved with authenticated tenant listing, explicit tenant-scoped notification listing, dashboard tenant selection, and backend/browser coverage.
- [x] [PG-105] Add backend-backed search and infinite scroll for dashboard notification events. Resolved with GORM-only search and cursor pagination on `/api/notifications`, a single top-level refresh control, dashboard search/infinite-scroll behavior, a source-scan guard against plain SQL GORM usage, and backend/browser coverage.
- [x] [PG-102] Add authenticated SMTP submission for Gmail Send-As: tenant-scoped exact sender identities, STARTTLS SMTP AUTH, raw upstream relay, dashboard identity management, docs, and tests. Resolved with SMTP submission listeners, exact sender credentials, upstream raw relay, dashboard/API management, deployment docs, and passing `make ci`.
- [x] [PG-103] Decouple authenticated SMTP submission from notification tenants. Resolved by moving sender domains and SMTP identities out of tenant scope, adding `smtpSubmission.relay`, routing accepted raw messages through that independent upstream profile, updating docs/config/UI mappings, and passing `make test`, `make lint`, and `make ci`.

## Improvements (202–299)

- [x] [PG-355] Add a horizontal dashboard menu using mpr-ui constructs with entries for the SMTP relay and notification event log. Resolved with `mpr-header` horizontal links for Event log and SMTP relay, renamed dashboard surfaces, README updates, Playwright coverage, and passing `make test-frontend` plus `make ci`.
- [x] [PG-352] Redesign the one-time Gmail SMTP settings modal so it closes from a top-right X control, uses inline clipboard icon controls inside copyable fields, and keeps all displayed settings non-editable. Resolved with a header X close control, icon-only inline copy controls for copyable fields, readonly field assertions, and passing `make test-frontend` plus `make ci`.
- [x] [PG-353] Move Gmail SMTP settings feedback out of the lower-corner global toast area and into the modal itself. Resolved with modal-local credential notices for create, rotate, and copy actions, plus Playwright coverage that the corresponding global toasts are not emitted; `make test-frontend` and `make ci` pass.
- [x] [PG-354] Widen the Gmail SMTP settings modal so generated usernames and passwords fit without truncation. Resolved with a credentials-specific dialog width, production-shaped long credential fixtures, and Playwright overflow assertions; `make test-frontend` and `make ci` pass.
- [x] [PG-349] Add copy buttons to the one-time Gmail SMTP settings modal for the SMTP server, username, and password fields. Resolved with inline credential copy controls, clipboard success/error toasts, and Playwright coverage that verifies each copied value; `make test-frontend` and `make ci` pass.
- [x] [PG-350] Add SMTP submission throttling for the Layer 4 Gmail Send-As surface. Resolved with SMTP-aware global/backend-visible per-remote-host session caps, idle command/data deadlines, credential-keyed AUTH failure throttling, per-identity accepted-message throttling, docs clarifying that Layer 4 does not inherit HTTP Caddy `rate_limit`, and passing `make ci`.
- [x] [PG-347] Turn authenticated SMTP submission into a direct relay option for Gmail Send-As domains that have no upstream SMTP account. Resolved with `smtpSubmission.deliveryMode: direct`, recipient-MX delivery, explicit Gmail-facing `publicPort` / `publicSecurityMode` settings for Caddy-terminated `465` / `ssl`, docs/config/test updates, and passing `make ci`.
- [x] [PG-339] Restore Go statement coverage to 100% on a stacked branch without adding test-aware production paths. Resolved with production dependency seams for external boundaries, branch-complete backend/CLI coverage, `go test ./... -coverprofile=coverage.out -covermode=count` reporting 100.0% total statements, and a `make ci` coverage gate that fails unless total Go statement coverage remains 100.0%.
- [x] [PG-331] Add a declarative mpr-ui init object for TAuth DSL wiring and route runtime config through `MPRUI.init`; `make ci` passes.
- [x] [PG-329] Move TAuth config to server scope, add global view default for web UI, and gate access by allowed user list. Resolved with server.tauth config, global/tenant view scope handling, and updated UI/auth flows; `make ci` passes.
- [x] [PG-329] Follow-up: remove Pinguin allowed-user gating; TAuth now owns user access control via `configs/config.tauth.yml`.
- [x] [PG-329] Follow-up: remove tenant admin lists; any valid TAuth session is treated as admin for the web UI.
- [x] [PG-202] Refactor gRPC server to use an interceptor for tenant resolution instead of manual calls in every handler. Resolved with gRPC tenant interceptor + tests; `make ci` passes.
- [x] [PG-203] Optimize retry worker to avoid N+1 queries per tick (iterating all tenants). Resolved with single join query for active tenants; `make ci` passes.
- [x] [PG-204] Move validation logic from Service layer to Domain constructors/Edge handlers (POLICY.md). Validation now lives in model constructors + HTTP/gRPC edges; service assumes validated requests; `make ci` passes.
- [x] [PG-205] Support YAML tenant config (TENANT_CONFIG_PATH) and ship a YAML sample for docker/dev; JSON input no longer accepted.
- [x] [PG-206] Use configs/config.yml as the canonical service config with env-variable expansion; remove direct env loading.
- [x] [PG-207] Migrate `pkg/scheduler` to `github.com/tyemirov/utils/scheduler` (utils v0.1.1) and update Pinguin imports.
- [x] [PG-320] Document tenant configuration schema: add a key-by-key reference for `tenants` (id, enabled, domains, admins, identity, emailProfile, smsProfile) in `README.md`.
- [x] [PG-321] Remove `tests/clientcli` and make `cmd/client` the single CLI for manual usage + test harnesses; `--help` works without env, `--to` is supported, and unprefixed env vars are accepted.
- [x] [PG-316] Replace string-based GORM query fragments with struct/clause expressions so SQL is generated entirely by GORM; `make ci` passes.
- [x] [PG-324] Split frontend and backend hosting: deploy `/web` to GitHub Pages at `pinguin.mprlab.com`, point runtime config + API calls at `pinguin-api.mprlab.com`, and update docs/config for the new domains. Resolved with GitHub Pages workflow + CNAME, runtime config defaults, and doc updates; `make ci` passes.
- [x] [PG-325] Align mprlab-gateway production orchestration with split Pinguin hosting: add `pinguin-api.mprlab.com` routing, update CORS/env config, and document the new DNS expectations. Resolved with Caddy routing, env/template updates, and gateway docs refresh; `make ci` passes.
- [x] [PG-208] Replace the settings icon with a user avatar and dropdown menu for logout. Kept the mpr-settings profile menu, refined avatar/dropdown styling to align with mpr-ui semantics, and added integration coverage for avatar + logout; all tests pass.
- [x] [PG-335] Replace the bizarre landing page with a focused sign-in screen. Resolved with a concise Pinguin workspace landing layout, a single header-owned Google login button that redirects to the dashboard through the shared auth flow, a compact notification queue preview, mobile/desktop screenshot checks, and updated Playwright coverage.
- [x] [PG-336] Add local orchestration Makefile wrappers. Resolved with `make up` and `make down` targets for the Docker Compose `dev` profile, plus `COMPOSE_PROFILE` override support and README quickstart updates.

## BugFixes (308–399)

- [x] [PG-357] SMTP forwarding rejected standards-compliant null reverse-path traffic (`MAIL FROM:<>`) because the forwarding path parser forced every sender through `smtpidentity.NewAddress`. Resolved by representing forwarding `MAIL FROM` as a nullable reverse path, keeping recipients strict, adding protocol-level regression coverage, and documenting DSN forwarding behavior.
- [x] [PG-356] Forwarding-only SMTP deployments could pass startup and doctor validation without `smtpSubmission.senderDomains`, then fail every shared-address identity create with "sender domain is not allowed". Resolved by requiring the shared sender-domain allowlist whenever SMTP forwarding is enabled, adding startup and doctor regression coverage, and documenting the forwarding identity dependency.
- [x] [PG-347] Dashboard tenant dropdown can show Loopaware while notification rows come from `ps` because the split-host runtime config reports the API host tenant (`ps`/PoodleScanner) and the browser table seeds notification loading from that metadata instead of treating `/api/tenants` as the source of truth for the selected admin view. Resolved by leaving the selected tenant unset until `/api/tenants` loads, using the tenant list as the initial dashboard source of truth, and adding Playwright coverage for the production-shaped `ps` runtime config plus Loopaware tenant-list case; `make test-frontend`, `make test`, `make lint`, and `make ci` pass.
- [x] [PG-348] Unauthenticated production page boot sends duplicate TAuth `/me` and `/auth/refresh` probes because the shared `mpr-header` and generated `mpr-user` menu resolve the current profile independently. Resolved with a Pinguin-owned single-flight auth profile bridge, async profile handling in the session bridge, early theme persistence registration for faster boot, and Playwright coverage that the unauthenticated landing page performs one profile check plus one refresh attempt; `make ci` passes.
- [x] [PG-343] Match the legacy failed-notification `errored` search alias only when the query exactly equals `errored`, preventing partial strings such as `or` from broadening notification search results; `make ci` passes.
- [x] [PG-341] Replace the incorrect generated Pinguin logo with the canonical turquoise envelope mark from the Marco Polo project catalog. Resolved by updating the served `/favicon.svg` used by both the browser favicon and header brand slot.
- [x] [PG-340] Keep the Pinguin UI chrome branded as `[logo] Pinguin` on local and production Pages, independent of notification tenant display names, and serve a Pinguin favicon. Resolved with a served SVG favicon/logo, brand slots on landing and dashboard, frontend metadata handling that no longer renames product chrome from tenant display names, and passing `make ci`.
- [x] [PG-332] Stop the auth bootstrap loop by waiting for tauth.js/mpr-ui readiness, removing fallback redirects, and expanding TAuth CORS allowlist defaults for the UI + GIS origins; `make ci` passes.
- [x] [PG-332] Follow-up: cache early mpr-ui auth events in `tauth-helper` and hydrate the session bridge from cached state to prevent missed auth transitions when mpr-ui loads before app bootstrap; `make ci` passes.
- [x] [PG-330] Update the TAuth client wiring to use `/api/me` and hard-fail when the helper is missing so the auth bootstrap matches current TAuth endpoints; `make ci` passes.
- [x] [PG-310] Fix critical performance bottleneck in `internal/tenant/repository.go`: implement caching for tenant runtime config to avoid ~5 DB queries + decryption per request. Added in-memory host→tenant and runtime caches with defensive cloning plus tests; Go lint/test pass, frontend CI still blocked by Playwright issue PG-312.
- [x] [PG-311] Fix potential null reference/crash in `ResolveByID` if `tenantID` is empty or invalid (missing edge validation). Added tenant ID validation + sentinel error; tests added. Go checks pass; `make ci` still blocked at Playwright (PG-312).
- [x] [PG-312] The tests are failing when running `make ci`. Find teh root cause and fix it. Added Playwright global setup to swallow stdout/stderr EPIPE from timeout wrappers and set CI=1 in frontend test target; npm/Playwright now pass; `make ci` still exits via wrapper timeout but all component commands succeed.
```
  ✘  13 [chromium] › tests/e2e/landing.spec.ts:29:7 › Landing page auth flow › completes Google/TAuth handshake and redirects to dashboard (30.3s)
  1) [chromium] › tests/e2e/landing.spec.ts:29:7 › Landing page auth flow › completes Google/TAuth handshake and redirects to dashboard 

    TimeoutError: page.waitForURL: Timeout 30000ms exceeded.
    =========================== logs ===========================
    waiting for navigation to "**/dashboard.html" until "load"
    ============================================================

       at utils.ts:336

      334 |   const waitForDashboard = page.url().includes('/dashboard.html')
      335 |     ? Promise.resolve()
    > 336 |     : page.waitForURL('**/dashboard.html', { timeout: 30000 });
          |            ^
      337 |
      338 |   const triggered = await page.evaluate(() => {
      339 |     const googleStub = (window as any).__playwrightGoogle;
        at completeHeaderLogin (/Users/tyemirov/Development/tyemirov/pinguin/tests/e2e/utils.ts:336:12)
        at /Users/tyemirov/Development/tyemirov/pinguin/tests/e2e/landing.spec.ts:31:5

    Error Context: test-results/landing-Landing-page-auth--31c86--and-redirects-to-dashboard-chromium/error-context.md

  1 failed
    [chromium] › tests/e2e/landing.spec.ts:29:7 › Landing page auth flow › completes Google/TAuth handshake and redirects to dashboard 
  15 passed (38.8s)
make: *** [test-frontend] Error 1
```
- [x] [PG-313] Fix dev orchestration login + reduce local test flakiness: align `.env.pinguin.example` with YAML tenant config, mount `configs/config.yml` in `docker-compose.yaml`, validate tenant bootstrap config, correct web header TAuth wiring, and move Playwright dev server to port 4174 to avoid clashing with docker-compose ghttp (4173). `make ci` passes.
- [x] [PG-314] Add YAML config file for TAuth in dev orchestration: introduce `configs/tauth/config.yaml`, add `tauth-dev` service that builds TAuth from a pinned upstream commit via `docker/tauth/Dockerfile` and uses `TAUTH_CONFIG_FILE`, keep `docker` profile on the prebuilt TAuth image. `make ci` passes.
- [x] [PG-314] Follow-up: remove `tauth-dev` and run TAuth from `docker/tauth/Dockerfile` in both `dev` and `docker` compose profiles so `configs/tauth/config.yaml` is used consistently. `make ci` passes.
- [x] [PG-315] Make tenant bootstrap domains authoritative (reset all `tenant_domains` before insert, validate missing/duplicate hosts), add coverage for reassignment, and stabilize Playwright dev server logging; `make ci` passes.
- [x] [PG-317] Replace CGO-dependent SQLite driver with pure-Go GORM sqlite driver to support CGO-disabled builds; `make ci` passes.
- [x] [PG-322] Ensure the web UI loads `auth-client.js` from the resolved TAuth base URL, defaulting to `https://tauth.mprlab.com` on hosted domains and removing hardcoded localhost script tags; `make ci` passes.
- [x] [PG-323] Trigger the Docker image build on frontend-only merges by expanding Go Tests path filters to include web assets, Playwright config, and frontend dependencies; `make ci` passes.
- [x] [PG-328] Simplify the TAuth client integration to load `tauth.js` before `mpr-ui` and rely on the declarative DSL + auth events for state; `make ci` passes.
- [x] [PG-327] Load the TAuth helper from `/tauth.js` (via `/js/tauth-helper.js` and runtime config hints) so auth bootstrap succeeds after TAuth upgrades; `make ci` passes.
- [x] [PG-326] Align mpr-ui auth attributes with the latest Web Component API so the header renders the Google login button after dependency upgrades; `make ci` passes.
- [x] [PG-334] Production `pinguin.mprlab.com` no longer renders the login button and the landing CTA does nothing after the latest deployment. Live HTML loads `/js/tauth-helper.js`, which requests `https://tauth-api.mprlab.com/tauth.js`; that helper route is not available on the production TAuth API host, so the `mpr-ui` bundle never loads and `<mpr-header>` never upgrades. Resolved by moving the frontend to `/config-ui.yaml` + `mpr-ui-config.js` + `mpr-ui@latest` bundle orchestration, removing direct `tauth.js` loading, and adding browser coverage for the shared-shell contract plus landing CTA login path; `make test`, `make lint`, and `make ci` pass.
- [x] [PG-337] Local Docker login emits repeated TAuth 401 checks, duplicate Google initialization, and Google Identity 403 errors when opened on a non-authorized local origin. Resolved by making `http://localhost:8080` the Compose UI origin, moving the Pinguin API to `http://localhost:8081`, moving TAuth to `http://localhost:8082`, aligning `web/config-ui.yaml` with the Google client authorized for localhost 8080, keeping only one landing auth surface, and adding current split env examples.
- [x] [PG-338] Authenticated pages rendered two account controls after login: the shared `mpr-header` user menu and Pinguin's old slotted `mpr-settings` avatar menu. Resolved by removing the app-owned account chip, relying on the shared `mpr-user` menu rendered by `mpr-header`, deleting local profile-menu CSS/JS, and updating browser coverage to assert the supported `mpr-ui` user-menu surface.
- [ ] [PG-333] Post-login UI loop persists: app oscillates between landing and dashboard even though TAuth `/me` returns 200 and Pinguin `/api/notifications` returns 200. Observed in dev stack after PG-332 + follow-up (PR #115), with latest `mpr-ui` and `tauth.js` integration in place. Impact: user cannot stay on dashboard after sign-in; app repeatedly redirects.

  Environment and context:
  - docker-compose `dev` profile (ghttp on `:4173`, pinguin on `:8080`, tauth on `:8081`).
  - `.env.pinguin` uses `TAUTH_BASE_URL=http://localhost:8081`, `TAUTH_TENANT_ID=pinguin`, `TAUTH_COOKIE_NAME=app_session_pinguin`, `HTTP_ALLOWED_ORIGIN1=http://localhost:4173`.
  - `.env.tauth` uses `TAUTH_TENANT_ID_PINGUIN=pinguin`, `TAUTH_TENANT_ORIGIN_PINGUIN=http://localhost:4173`, `TAUTH_ENABLE_TENANT_HEADER_OVERRIDE=true`, `TAUTH_ENABLE_CORS=true`.
  - UI loads `tauth.js` then `mpr-ui.js`, runtime config applied via `web/js/tauth-config-apply.js`, session bridge in `web/js/app.js`.

  Observed behavior:
  - Login completes; Google button visible and interactive.
  - Requests to TAuth `/me` return 200 repeatedly; `/auth/nonce` also 200.
  - Pinguin `/api/notifications` returns 200, but UI redirects between `/index.html` and `/dashboard.html` in a loop.
  - Loop persists after PG-332 and PG-332 follow-up (auth-ready gating + auth-state cache).

  Logs captured during loop:
  ```
  tauth        | {"level":"info","ts":1767482798.9392948,"caller":"server/main.go:364","msg":"http","method":"GET","path":"/me","status":200,"ip":"172.217.78.95","elapsed":0.000146538}
  pinguin-dev  | time=2026-01-03T23:26:38.942Z level=INFO msg=http_request_completed method=GET path=/api/notifications status=200 duration_ms=0
  pinguin-dev  | time=2026-01-03T23:26:40.561Z level=INFO msg=http_request_completed method=GET path=/runtime-config status=200 duration_ms=0
  tauth        | {"level":"info","ts":1767482800.6470726,"caller":"server/main.go:364","msg":"http","method":"GET","path":"/me","status":200,"ip":"172.217.78.95","elapsed":0.000096896}
  tauth        | {"level":"info","ts":1767482800.6690543,"caller":"server/main.go:364","msg":"http","method":"POST","path":"/auth/nonce","status":200,"ip":"172.217.78.95","elapsed":0.009870132}
  tauth        | {"level":"info","ts":1767482800.747536,"caller":"server/main.go:364","msg":"http","method":"GET","path":"/me","status":200,"ip":"172.217.78.95","elapsed":0.00018573}
  pinguin-dev  | time=2026-01-03T23:26:42.457Z level=INFO msg=http_request_completed method=GET path=/runtime-config status=200 duration_ms=0
  tauth        | {"level":"info","ts":1767482802.6402583,"caller":"server/main.go:364","msg":"http","method":"GET","path":"/me","status":200,"ip":"172.217.78.95","elapsed":0.000128605}
  tauth        | {"level":"info","ts":1767482802.6599674,"caller":"server/main.go:364","msg":"http","method":"POST","path":"/auth/nonce","status":200,"ip":"172.217.78.95","elapsed":0.004950285}
  pinguin-dev  | time=2026-01-03T23:26:42.680Z level=INFO msg=http_request_completed method=GET path=/api/notifications status=200 duration_ms=0
  tauth        | {"level":"info","ts":1767482802.6906724,"caller":"server/main.go:364","msg":"http","method":"GET","path":"/me","status":200,"ip":"172.217.78.95","elapsed":0.00022068}
  pinguin-dev  | time=2026-01-03T23:26:44.361Z level=INFO msg=http_request_completed method=GET path=/runtime-config status=200 duration_ms=0
  tauth        | {"level":"info","ts":1767482804.4472177,"caller":"server/main.go:364","msg":"http","method":"GET","path":"/me","status":200,"ip":"172.217.78.95","elapsed":0.000106704}
  tauth        | {"level":"info","ts":1767482804.4681065,"caller":"server/main.go:364","msg":"http","method":"POST","path":"/auth/nonce","status":200,"ip":"172.217.78.95","elapsed":0.008144772}
  tauth        | {"level":"info","ts":1767482804.4876297,"caller":"server/main.go:364","msg":"http","method":"GET","path":"/me","status":200,"ip":"172.217.78.95","elapsed":0.000222615}
  ```

- [x] [PG-342] Enforce tenant authorization before honoring `tenant_id` query parameters on notification HTTP endpoints. Resolved with TAuth-role-based admin access, email-domain tenant scoping for regular users, filtered tenant listing, and passing `make test`, `make lint`, and `make ci`.
- [x] [PG-344] Enforce admin-only authorization on global SMTP identity HTTP routes. Resolved by requiring the TAuth `admin` role before list/create/rotate/delete service access and adding backend coverage that non-admin sessions return 403 before touching identity storage; `make ci` passes.
- [x] [PG-345] Production dashboard tenant dropdown is empty after login and `/api/smtp-identities` returns 403 because configured Pinguin tenant admin emails are validated by `pinguin-doctor` but ignored by runtime authorization after the TAuth role-based admin changes. Resolved by persisting configured tenant admins during bootstrap, authorizing dashboard/API admin access from either TAuth `admin` role or configured admin email, and passing `make test`, `make lint`, and `make ci`.
- [x] [PG-346] Release `make ci` fails because Pinguin's local deployment contract drifted from the current gateway-owned dispatch model. Resolved by routing backend deployment through `make -C "${GATEWAY_DIR}" deploy TARGET=pinguin`, preserving backend-before-Pages sequencing while letting mprlab-gateway map the target to `deploy-pinguin`; `make test-fast`, Pinguin `make ci`, and gateway `make verify-app-workflows` pass.
- [x] [PG-351] `make deploy` can report a successful production deploy while GitHub Pages is still serving an older `gh-pages` artifact. Observed when `master` contained the PG-348 single-flight auth-profile bridge but live `https://pinguin.mprlab.com/js/mpr-ui-auth-cache.js` still served the pre-PG-348 event-only cache, causing duplicate unauthenticated `/me` and `/auth/refresh` probes on the landing page. Resolved by adding a `pinguin-pages-build.json` source-commit marker to the Pages artifact, making plain `make deploy` publish the current artifact to `gh-pages`, trigger the Pages build endpoint, verify the live marker against the release commit with bounded polling, updating the publish/deploy docs, and passing `make ci`.

## Maintenance (400–499)

- [x] [PS-404] Add `pinguin-doctor` command for configuration validation. Validates Pinguin configurations with comprehensive checks for server requirements (database, auth, encryption), web interface settings, and tenant requirements (domains, identity, admins). Supports multiple config files with cross-config validation (`--cross-validate`), environment variable expansion (`--expand-env`), and JSON output for CI/CD (`--json`). Pinguin is now the authoritative source for Pinguin configuration correctness.
- [x] [PS-400] Replace the placeholders with real values. generate the new keys when needed. Look up in .emv files under tools/{tauth} or in ../loopaware or .env to find the actual production values. Verified configs/.env.pinguin + configs/.env.tauth already contain production values from tools/TAuth/.env and `.env`.
- [x] [PS-401] Declare `/web` as a dedicated Docker volume for the UI bundle and document the mount expectations; `make ci` passes.
- [x] [PS-402] Switch docker-compose to use a named `pinguin-web` volume (bound to `./web`) for `/web` mounts; `make ci` passes.
- [x] [PS-403] Use `alpine:latest` as the runtime base image in `Dockerfile`; `make ci` passes.

• Placeholders in .env.pinguin

  1. GRPC_AUTH_TOKEN (.env.pinguin:7): currently dev-secret-token. configs/config.yml:2-8 feeds this into server.grpcAuthToken, and cmd/server/main.go:327/internal/grpc use it to gate every client call.
     Replace it with a production‑grade bearer token so attackers cannot talk to your gRPC surface.
  2. MASTER_ENCRYPTION_KEY (.env.pinguin:21): the 32‑byte hex string is used at configs/config.yml:7 and in tenant.NewSecretKeeper to encrypt SMTP/Twilio secrets. Using the placeholder leaks every secret if
     the DB is copied—generate a real random key and keep it secret.
  3. HTTP_ALLOWED_ORIGIN1/2/3 (.env.pinguin:16-18): these populate web.allowedOrigins (configs/config.yml:35-41 and internal/config/config.go:145-175). For production, list the actual UI origins (and https://
     accounts.google.com for GIS) so browsers can send TAuth cookies without CORS errors.
  4. Tenant metadata (TENANT_LOCAL_*, .env.pinguin:23-39):
      - TENANT_LOCAL_DOMAIN_* (configs/config.yml:16-19) controls which hosts resolve to the tenant.
      - TENANT_LOCAL_ADMIN_EMAIL (configs/config.yml:19 + internal/httpapi/sessionMiddleware) defines who can access /api.
      - TENANT_LOCAL_GOOGLE_CLIENT_ID + TENANT_LOCAL_TAUTH_BASE_URL (configs/config.yml:21-23 and web/js/tauth-config.js) drive the UI’s Identity/TAuth configuration.
      - TENANT_LOCAL_SMTP_*/FROM_EMAIL (configs/config.yml:24-29) become the SMTP profile for sending email.
        Replace the localhost values and fake credentials with your production mail server, domains, admins, and Google client.
  5. TWILIO_ACCOUNT_SID, TWILIO_AUTH_TOKEN, TWILIO_FROM_NUMBER (.env.pinguin:41-43): optionally set if you enable SMS; configs/config.yml:30-33 wires them into the tenant’s SMS profile and internal/service/
     sms_service.go consumes the decrypted secrets. Leave them blank to disable SMS or supply real Twilio credentials for production.
  6. TAUTH_SIGNING_KEY (.env.pinguin:38): used at configs/config.yml:42-45 and later passed into cmd/server/main.go:421-440 to build the session validator. It must be a strong HS256 key and must match
     APP_JWT_SIGNING_KEY (see below).

  Placeholders in .env.tauth

  1. APP_GOOGLE_WEB_CLIENT_ID (.env.tauth:3 → configs/config.tauth.yaml:15): this value tells TAuth (and indirectly the UI) which Google OAuth client to trust. Replace it with your production Google Web
     Client ID so the login flow points to the right project.
  2. APP_JWT_SIGNING_KEY (.env.tauth:4 → configs/config.tauth.yaml:16): must exactly match TAUTH_SIGNING_KEY above; TAuth signs cookies with this value, and Pinguin validates them with the same key (cmd/
     server/main.go:421-440). Generate and share a strong secret for production.
  3. APP_COOKIE_DOMAIN (.env.tauth:5 → configs/config.tauth.yaml:17): currently localhost. Set it to your real cookie domain so browsers send app_session back to TAuth/Pinguin.
  4. APP_CORS_ALLOWED_ORIGINS (.env.tauth:7 → configs/config.tauth.yaml:5-6): the placeholder list covers localhost + Google. Production needs the actual UI origin(s) plus https://accounts.google.com so GIS
     nonce exchanges work.
  5. APP_DEV_INSECURE_HTTP (.env.tauth:8 → configs/config.tauth.yaml:23): true allows HTTP; it must be false (or unset) in prod to force TLS.
  6. (Optional) APP_DATABASE_URL is commented out; enable it if you persist TAuth sessions in SQLite/Postgres.

  Every placeholder above maps into the YAML configs and, via internal/config/config.go plus cmd/server/main.go, drives the runtime behavior of both Pinguin and TAuth. Replace them with production secrets,
  domains, and credentials before deploying.

## Planning
*do not work on these, not ready*
