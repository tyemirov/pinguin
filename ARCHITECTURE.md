# Architecture

## System Overview

- Pinguin runs a gRPC notification service and an optional Gin HTTP API in one Go process.
- The gRPC service uses port `50051`. The HTTP API uses port `8080`.
- Static browser assets run separately. Production uses GitHub Pages at `https://pinguin.mprlab.com`. Local development uses the ghttp container on port `8080` and the Pinguin HTTP API on port `8081`.
- SQLite stores tenants, delivery profiles, API credential digests, notifications, attachments, SMTP sender domains, SMTP identities, and forwarding routes.
- A new database can start with zero tenants. The server creates the exact managed schema for an empty database.
- The server rejects a non-empty database that does not have the exact current schema.
- Tenant configuration changes take effect from the database without a service restart.

## Ownership and Authentication

- TAuth owns browser authentication. The `<mpr-header>` component starts sign-in, TAuth issues the HttpOnly session cookie, and Pinguin validates that cookie for each `/api` request.
- Pinguin uses the validated TAuth user ID as `owner_user_id`. Each browser tenant query contains both the owner user ID and tenant ID.
- A user can own more than one tenant. A foreign tenant ID returns `404` and does not disclose that tenant.
- Each tenant has exactly one API credential. The API key format is `pgn_1_<credential-uuid>_<secret>`.
- Pinguin stores only the SHA-256 digest of the 32-byte API secret. The browser shows the raw key once during tenant creation or rotation and clears it when the dialog closes.
- The gRPC interceptor validates one bearer API key and resolves its tenant.
- The interceptor adds the tenant runtime data to the request context. gRPC messages and metadata contain no caller-selected tenant ID.

## Managed Tenant Data

- A tenant contains a server-generated UUID, immutable owner user ID, display name, optional support email, version, and timestamps.
- Tenant creation stores one tenant, one email profile, and one API credential in one transaction.
- The same transaction stores an optional complete SMS profile.
- Provider usernames, passwords, Twilio account identifiers, and Twilio tokens are encrypted with `MASTER_ENCRYPTION_KEY`.
- Management responses contain safe metadata and do not contain provider secrets or raw API secrets.
- Tenant metadata, delivery profiles, and the API credential use version preconditions through `ETag` and `If-Match`.
- Tenant deletion is permanent. It removes notifications, attachments, profiles, the API credential, SMTP resources, forwarding routes, and idempotency records in one transaction.

## HTTP API

- `GET /runtime-config` returns `apiBaseUrl`, `tenantUrl`, `eventLogUrl`, and `smtpRelayUrl`. It contains no tenant or authentication-provider data.
- `GET /healthz` is the unauthenticated readiness route.
- `/api/tenants` lists and creates owner-scoped tenants.
- `/api/tenants/:tenant_id` reads, replaces tenant metadata, and permanently deletes one tenant.
- Nested `email-profile`, `sms-profile`, and `api-credential` routes manage delivery data and the single programmatic credential.
- Nested `notifications` routes list, reschedule, and cancel tenant notifications.
- Nested `smtp-domains` and `smtp-identities` routes manage tenant-owned SMTP resources.
- Tenant creation requires `Idempotency-Key`. Mutating an existing versioned resource requires `If-Match`.
- Authenticated responses use `Cache-Control: private, no-store`.
- CORS permits configured browser origins and credentials. Forwarding headers affect `source_ip` only when the direct peer is in `web.trustedProxies`.

## Notification Runtime

- The gRPC API credential defines the tenant for each programmatic notification request.
- The retry worker lists current tenant IDs and resolves fresh tenant provider data before dispatch.
- Email uses the tenant external SMTP profile. SMS uses the optional tenant Twilio profile.
- Scheduled and errored notifications remain in SQLite for retry processing.

## SMTP Submission and Forwarding

- SMTP sender domains, identities, identity credentials, and forwarding routes belong to one tenant.
- Sender-domain names and identity email addresses are globally unique.
- The SMTP relay page requires a tenant selection before it loads or changes SMTP resources.
- `smtpSubmission` and `smtpForwarding` remain service configuration because their listeners and upstream relays are shared infrastructure.
- The optional submission listener authenticates an exact SMTP identity. It relays through `smtpSubmission.relay` or delivers to recipient MX hosts when `deliveryMode` is `direct`.
- The optional forwarding listener accepts mail only for active identities that have forwarding recipients. It stores no mailbox or message body.
- The schema version 4 manifest publishes SMTPS on `pinguin-api.mprlab.com:465` and MX delivery on `mx.pinguin.mprlab.com:25` through gateway-owned listeners.

## Browser Structure

- `index.html` is the public landing page. `tenants.html`, `event-log.html`, and `smtp-relay.html` are authenticated workspaces.
- `web/js/runtime-config.js` buffers the latest public `mpr-ui` authentication event before the application module loads.
- `web/js/core/sessionBridge.js` consumes the buffered event and the optional public profile snapshot.
- `web/js/core/apiClient.js` owns the nested tenant HTTP routes.
- `web/js/ui/tenantManagement.js` owns tenant creation, update, credential rotation, and permanent deletion.
- Event log and SMTP relay require a selected owner tenant.
- Each page uses the shared MPR header, three workspace destinations, and the seven-service MPR footer.

## Configuration and Deployment

- `configs/config.pinguin.yml` contains only service settings: database, logging, encryption, TAuth validation, HTTP, SMTP submission, and SMTP forwarding.
- Runtime configuration and `.mprlab/deploy/resources.yml` contain no tenant definitions, tenant provider values, or global gRPC token.
- `.mprlab/deploy/resources.yml` is the schema version 4 production declaration.
- The bounded `cmd/convert-managed-tenants` command converts the former schema during one write-stopped maintenance window.
- The conversion assigns all current production tenants to the TAuth account for `temirov@gmail.com`.
- Server startup never runs this conversion.
- The production operator owns conversion and deployment. After production acceptance, a steady-state change removes the conversion command.

## Validation

- `make test-coverage` requires 100% Go statement coverage.
- `make test-frontend` runs the Playwright browser suite against `tests/support/devServer.js`.
- `make ci` runs format checks, Go analysis, all Go tests, the coverage gate, and browser acceptance tests.
