# Managed Tenant Configuration

## Status

This document defines the implemented managed tenant contract and its production recovery record.

P001 verified this plan against the current product and deployment contracts on 2026-08-11.

[P002](../.mprlab/ISSUES.archive.md) completed the product decisions on 2026-08-11.

F001 retires the obsolete converter after the recorded authorized recovery.

The source implementation and automated acceptance suite are completed.

## Purpose

Pingwin stores tenant definitions in SQLite.
The former YAML tenant contract is obsolete.

The managed product lets an authenticated user create and manage tenant configuration in Pingwin.

The managed product does not require an operator to edit tenant YAML or restart the service.

## Confirmed Requirements

- Keep the current TAuth and `mpr-ui` authentication integration.
- Use the validated TAuth user ID as the Pingwin owner identity.
- Let each authenticated user establish their Pingwin tenants.
- Store tenant configuration in the Pingwin database.
- Remove tenant definitions from the runtime configuration file.
- Require one complete external email delivery profile during tenant creation.
- Permit an optional complete SMS delivery profile during tenant creation.
- Give each tenant exactly one API credential for programmatic notification requests.
- Store only a one-way API secret digest.
- Keep raw API secrets out of stored data and operator responses.
- Provide permanent tenant deletion without suspension.
- Make SMTP sender domains, identities, forwarding routes, and credentials tenant-owned.
- Keep shared SMTP listeners and upstream relays in service configuration.
- Keep one forward-only runtime contract.
- Keep only the current managed schema in the runtime.
- Keep obsolete conversion code outside the application source.

## Scope Boundary

TAuth remains the authentication service.

TAuth already provides login, logout, session restoration, and session validation.

Pingwin consumes the validated session and owns tenant authorization.

This plan does not add Pingwin login code, TAuth account code, or browser token code.

This plan removes tenant data from YAML.

Pingwin still uses service configuration for listeners, the database, logging, TAuth validation, and the master encryption key.

A separate change can replace the service configuration transport if that outcome becomes necessary.

## Former State Before F001

### Tenant source

`internal/config/config.go` read `TenantConfigPath` or inline `TenantBootstrap` data.

`cmd/server/main.go` required one source and ran tenant bootstrap during each service start.

`internal/tenant/bootstrap.go` treated the configured tenant list as the source of truth.

The bootstrap operation reset profiles and removed tenants that were absent from the file.

### Browser authentication

`web/config-ui.yaml` and `mpr-ui-config.js` configure the shared browser shell.

`web/js/core/sessionBridge.js` consumes the existing `mpr-ui` authentication events and profile snapshot.

`internal/httpapi/server.go` validates the TAuth session cookie with `sessionvalidator`.

This completed integration remains the authentication boundary.

The validated TAuth claims include the user ID through `Claims.GetUserID()`.

### Browser authorization

The HTTP API authorized tenant access through configured administrator email addresses, user roles, and email domains.

The browser supplied a tenant ID for notification operations.

The server compared that ID with the tenants that the session could use.

### Programmatic authorization

The gRPC server compared one deployment-wide bearer token.

The caller also supplied a tenant ID in request fields or `x-tenant-id` metadata.

A valid deployment token could select any configured tenant.

### Stored data and secrets

The database stored tenants, notifications, email profiles, and SMS profiles.

`internal/db/db.go` ran GORM `AutoMigrate` during each service start.

Pingwin encrypts provider credentials with the service master key.

Pingwin must decrypt provider credentials to deliver email and SMS messages.

SMTP sender domains and identities used an owner email address instead of a tenant ID.

### Deployment inputs

`configs/config.production.yml` contained tenant definitions and the global gRPC token reference.

The current `.mprlab/deploy/resources.yml` file declared each tenant value and the global token as private inputs.

As a result, tenant changes required operator input and a new service configuration.

## Ownership Model

The HTTP session gives Pingwin a validated TAuth user ID.

Pingwin uses that value as `owner_user_id`.

The browser does not submit `owner_user_id`.

Each tenant has one immutable owner value.

One owner can have more than one tenant.

Every tenant query includes both `owner_user_id` and `tenant_id`.

Pingwin returns `404` when the tenant does not exist or belongs to a different owner.

This response does not disclose the existence of another owner's tenant.

## Managed Data Model

### Tenant

Add these current fields to the tenant record:

- `id`: A server-generated UUID.
- `owner_user_id`: The immutable validated TAuth user ID.
- `display_name`: The tenant name for the user interface.
- `support_email`: An optional validated support address.
- `version`: The value for update preconditions.
- `created_at`: The creation time.
- `updated_at`: The last change time.

Remove `TenantDomain` and `TenantAdmin` from the managed schema.

Remove host and email-domain authorization from the managed contract.

### Delivery profiles

Keep `EmailProfile` and `SMSProfile` as tenant-owned records.

The email delivery profile contains the external SMTP connection that Pingwin uses to send notifications.

Require one complete email delivery profile in each tenant creation request.

Permit one complete SMS profile when the tenant uses SMS delivery.

Store SMTP usernames and passwords as encrypted values.

Store Twilio account identifiers and tokens as encrypted values.

Return only safe profile metadata through the management API.

Require a replacement secret when the user changes a stored provider secret.

Do not accept a masked value as a replacement secret.

### API credential

Add an `APICredential` record with these fields:

- `id`: The public credential UUID.
- `tenant_id`: The unique tenant foreign key.
- `secret_digest`: The 32-byte SHA-256 digest.
- `display_prefix`: A safe key prefix for identification.
- `created_at`: The creation time.
- `last_used_at`: The last successful authentication time.
- `updated_at`: The last rotation time.
- `version`: The value for rotation preconditions.

Use the exact credential format `pgn_1_<credential-id>_<secret>`.

Use a UUID for `<credential-id>`.

Use 32 random bytes and base64url without padding for `<secret>`.

The browser creates the secret with Web Crypto.

The browser calculates SHA-256 from the decoded secret bytes.

The browser submits the credential ID and digest with the tenant creation request.

Pingwin derives the display prefix from the validated credential ID.

Each tenant always has one API credential.

Credential rotation atomically replaces the credential ID and digest.

Pingwin stores the digest and compares it in constant time.

Pingwin never stores, returns, or logs the raw API secret.

The gRPC server receives the bearer value during request authentication.

A bearer protocol cannot hide the presented value from the authentication process.

This contract makes the secret unavailable in stored data and later operator responses.

P001 selects bearer API keys with digest-only storage.

P001 does not require process-level secret invisibility.

Asymmetric request signatures are outside F001.

### Idempotency record

Add an owner-scoped idempotency record for tenant creation.

Store the operation, idempotency key, request digest, tenant ID, response status, and creation time.

An exact repeated request returns the stored result.

A repeated key with a different request digest returns `409`.

## HTTP Contract

All management routes use the existing TAuth session middleware.

The HTTP boundary creates validated owner, tenant, profile, and credential domain values.

### Tenant routes

- `GET /api/tenants`
- `POST /api/tenants`
- `GET /api/tenants/:tenant_id`
- `PUT /api/tenants/:tenant_id`
- `DELETE /api/tenants/:tenant_id`

`POST` derives the owner from the session and creates a server-owned tenant ID.

`POST` requires an `Idempotency-Key` header.

The create request contains tenant metadata, one complete email profile, one API credential ID, and one API credential digest.

The create request can contain one complete SMS profile.

The create transaction stores all required tenant resources before it returns `201`.

`PUT` keeps the owner and tenant ID unchanged and requires a matching `If-Match` header.

`DELETE` requires a matching `If-Match` header.

`DELETE` permanently removes the tenant and all tenant-owned records.

The delete transaction removes notifications, attachments, profiles, API credentials, SMTP resources, and forwarding routes.

Responses return safe profile state and do not return provider secrets.

Tenant representations use an `ETag` that contains the tenant version.

### Delivery profile routes

- `GET /api/tenants/:tenant_id/email-profile`
- `PUT /api/tenants/:tenant_id/email-profile`
- `PATCH /api/tenants/:tenant_id/email-profile`
- `GET /api/tenants/:tenant_id/sms-profile`
- `PUT /api/tenants/:tenant_id/sms-profile`
- `PATCH /api/tenants/:tenant_id/sms-profile`
- `DELETE /api/tenants/:tenant_id/sms-profile`

`PUT` completely replaces a profile and requires all required provider secrets.

`PATCH` changes only documented fields and keeps each omitted secret unchanged.

A secret field must contain a new secret or be absent.

`DELETE` removes the optional SMS profile. It requires the current profile version in `If-Match`. A repeated request with the same version succeeds when the profile is absent.

The profile routes reject masked secret values.

Profile updates require the current `ETag` value in `If-Match`.

### Credential route

- `GET /api/tenants/:tenant_id/api-credential`
- `PUT /api/tenants/:tenant_id/api-credential`

The read response contains safe metadata only.

`PUT` accepts a new client-generated credential ID and digest.

`PUT` requires a matching credential `If-Match` header.

An exact repeated rotation is idempotent.

A successful rotation invalidates the prior API key immediately.

The raw key remains in browser memory until the user closes the one-time dialog.

Authenticated management responses use `Cache-Control: private, no-store`.

### SMTP routes

- `GET /api/tenants/:tenant_id/smtp-domains`
- `POST /api/tenants/:tenant_id/smtp-domains`
- `POST /api/tenants/:tenant_id/smtp-domains/:domain_id/dns-checks`
- `GET /api/tenants/:tenant_id/smtp-identities`
- `POST /api/tenants/:tenant_id/smtp-identities`
- `PATCH /api/tenants/:tenant_id/smtp-identities/:identity_id`
- `GET /api/tenants/:tenant_id/smtp-identities/:identity_id/credential`
- `PUT /api/tenants/:tenant_id/smtp-identities/:identity_id/credential`
- `DELETE /api/tenants/:tenant_id/smtp-identities/:identity_id`

These routes manage the optional Pingwin SMTP submission and forwarding capability.

These resources are separate from the required external email delivery profile and do not block tenant creation.

Each SMTP query includes the tenant ID and the validated owner user ID.

A sender domain belongs to one tenant.

The DNS check route creates one DNS check result.

The SMTP identity patch changes forwarding recipients.

The credential put operation rotates the SMTP identity credential.

Service configuration continues to own SMTP listeners and upstream relay profiles.

### Tenant resource routes

Move notification routes under the authorized tenant:

- `GET /api/tenants/:tenant_id/notifications`
- `PATCH /api/tenants/:tenant_id/notifications/:notification_id`

The notification patch schema permits one state change in each request.

The schema accepts a new `scheduled_time` or the `cancelled` status.

Delete replaced global routes in the release that starts the new contract.

Do not add route aliases or redirects.

### Error contract

- Return `400` for malformed JSON.
- Return `401` for an invalid TAuth session.
- Return `404` for an absent or foreign tenant resource.
- Return `409` for a unique-value or notification-state conflict.
- Return `412` for a failed `If-Match` precondition.
- Return `415` for an unsupported request media type.
- Return `422` for valid JSON that has invalid domain values.
- Return `500` for an internal boundary failure.

Return each expected error as `{ "error": { "code": "...", "message": "...", "request_id": "..." } }`.

Keep secret values out of errors and logs.

## gRPC Contract

Replace the global token interceptor and tenant interceptor with one credential interceptor.

The interceptor performs these operations:

1. Read exactly one bearer credential.
2. Parse the current credential format.
3. Load the active credential by its public ID.
4. Compare the supplied secret digest in constant time.
5. Load the active tenant configuration.
6. Add validated tenant runtime data to the request context.
7. Record the credential use time.

Remove the caller-owned tenant ID from gRPC request messages and metadata.

Remove `tenant_id` from responses when authentication already defines the tenant.

Reserve each removed protobuf field number and field name in its original message.

Regenerate the Go gRPC package with the repository Make target.

Change `pkg/client.Settings` to accept a server address and one API key.

Remove `TenantID()`, request tenant mutation, and `x-tenant-id` metadata.

Replace the CLI `--grpc-auth-token` and `--tenant-id` flags with `--api-key`.

Update the `grpcurl`, CLI, and Go client examples.

Return `Unauthenticated` for a missing, malformed, or invalid API key.
Return `Internal` when credential storage lookup fails.

## Browser Work

Keep the current shared header, footer, authentication events, and session snapshot.

Add a tenant management destination to the shared application navigation.

Load tenant data only after the existing authenticated event or snapshot.

Show a managed empty state when the authenticated user has no tenants.

Add semantic Alpine components for these tasks:

- List owned tenants.
- Create a tenant.
- Change tenant metadata and delivery profiles.
- Permanently delete a tenant after explicit confirmation.
- Create and copy the API key during tenant creation.
- Show safe API credential metadata.
- Rotate and copy the API key.
- Manage tenant-owned SMTP domains, identities, credentials, and forwarding routes.

Clear the raw key from component state when the one-time dialog closes.

When authentication ends, clear tenant data, forms, open dialogs, pending operations, selected tenant state, and one-time API keys before redirecting to the landing page.

Use a single-flight state for destructive requests.

Restore focus to a stable control after each dialog closes.

Move Event log API calls to the selected owner tenant path.

Move all SMTP relay API calls under the selected tenant path.

## Backend Work

### Domain and repository work

1. Add smart constructors for owner IDs, tenant IDs, names, profiles, credential IDs, and credential digests.
2. Add owner fields and unique indexes to the tenant model.
3. Add the API credential model and its unique tenant foreign key.
4. Add transactional tenant create, list, read, update, and delete operations.
5. Add credential read, rotate, and verify operations.
6. Add runtime profile cache invalidation after each tenant change.
7. Delete bootstrap, domain, and configured administrator queries.
8. Add tenant foreign keys to SMTP sender domains and identities.
9. Delete SMTP owner email and administrator access scopes.

### HTTP work

1. Create `OwnerUserID` from validated TAuth claims at the HTTP boundary.
2. Add the tenant and credential handlers.
3. Add owner checks to all nested resource handlers.
4. Make `/runtime-config` independent from a tenant host.
5. Delete email-domain, configured administrator, role, and host authorization.
6. Add idempotency and update-precondition handling.

### gRPC and client work

1. Add the tenant credential interceptor.
2. Delete the global bearer comparison.
3. Delete caller-selected tenant resolution.
4. Change the protobuf contract and regenerate code.
5. Change the Go client and CLI contract.
6. Audit logs for API key disclosure.

### Configuration work

1. Remove `GRPCAuthToken`, `TenantConfigPath`, and `TenantBootstrap` from `internal/config`.
2. Remove tenant bootstrap from `cmd/server/main.go`.
3. Let the service start with zero tenants.
4. Remove tenant and token validation from `pingwin-doctor`.
5. Remove tenant and token values from tracked configuration templates.
6. Remove tenant and token private inputs from `.mprlab/deploy/resources.yml`.
7. Keep only service-level runtime configuration in the files.
8. Create the exact current schema for an empty database.
9. Validate the exact current schema for a non-empty database.
10. Remove GORM data conversion from steady-state service startup.

## Test Work

Add black-box HTTP tests with two TAuth users and two tenant sets.

Prove that each user can use only their tenant resources.

Add gRPC tests with two credentials and two tenants.

Prove that a credential cannot select another tenant.

Prove that a rotated credential fails on the next request.

Prove that each tenant has exactly one API credential.

Prove that the database does not contain a raw API secret.

Add browser tests for these flows:

- An authenticated user has no tenants.
- The user creates the first tenant.
- The user changes a tenant configuration.
- The user creates and copies the API key during tenant creation.
- The user rotates and copies the API key.
- The user permanently deletes a tenant.
- Tenant deletion removes all tenant-owned records.
- The user manages only SMTP resources for the selected tenant.
- Direct access to another owner's tenant returns the expected result.
- Dialog focus and request state remain accessible.

Add HTTP tests for idempotent creates and failed update preconditions.

Add configuration tests that reject removed tenant and token keys.

Add a start test for an empty tenant database.

Add deployment tests that reject tenant-specific private inputs.

Run focused Make targets during implementation.

Run `make ci` after the final implementation change.

## Production Data Conversion

### Completed managed tenant recovery

The original four-tenant conversion was incomplete. On September 15, 2026, the user authorized restoration of the missing tenants.
Loopaware, NameSignal, and SummerCan were restored through the managed API. PoodleScanner retained its existing Amazon SES profile.
The saved production database query confirms all four tenants share owner `google:111357980452034959148`.
The restoration used the same authenticated browser session as the retained PoodleScanner tenant.

The archived chat `01a0a3de-9e9a-7521-9266-83908bef89e1`, turn `01a0a3de-aee1-7790-a56f-f4bd2fb017d7`, records the authorized recovery.
Command `exec-02b1223e-a4f8-48f6-99b6-d5d7db3114f4` records the production owner assignments.
The private receipt directory is `$HOME/.config/pinguin/tenant-recovery-2026-09-15-cB6TIM`.

| Receipt | Recorded result |
| --- | --- |
| `probe-receipt.json` | The three restored tenant API keys authenticated successfully. |
| `email-acceptance-receipt.json` | Loopaware delivery reached Gmail; initial NameSignal and SummerCan relay attempts failed. |
| `mailbox-verification-receipt.json` | Corrected NameSignal and SummerCan profiles returned `SENT`; both messages reached Gmail from their dedicated sender addresses. |
| `client-inputs-receipt.json` | Client API keys were updated in local private deployment inputs. Client redeployment was not performed. |

The user explicitly deferred SMS acceptance. Client application activation remains separate operational work.
These records establish authorized tenant restoration and managed-runtime acceptance.
They do not establish execution of the original converter, preservation of historical notification data, or matching conversion output counts.
The November 12 deployment record separately confirms the managed production schema.

F001 removes the obsolete conversion source, image executable, and former input procedure.
The service supports the current managed schema; a new empty database receives that schema at startup.

## Implementation Order

1. Add failing HTTP, gRPC, browser, configuration, and deployment contract tests.
2. Add managed tenant and credential domain types.
3. Add the exact database schema and owner-scoped repositories.
4. Add managed HTTP tenant, profile, credential, and SMTP APIs.
5. Replace gRPC authentication and update clients.
6. Add the managed tenant browser interface.
7. Complete the recorded tenant recovery and remove the obsolete converter.
8. Remove tenant YAML, global token input, bootstrap, old routes, and startup data conversion.
9. Update current documentation and run the final `make ci` gate.

## Acceptance Criteria

- An existing TAuth user can create a Pingwin tenant without an operator change.
- Tenant creation rejects an incomplete email profile or an absent API credential ID or digest.
- A new Pingwin database can start with zero tenants.
- An owner can list and change only their tenants.
- A foreign tenant ID does not disclose foreign tenant data.
- Repeated tenant creates and credential rotations do not create duplicate resources.
- A stale tenant update fails with `412`.
- Each tenant has exactly one API credential.
- Credential rotation immediately rejects the prior API key.
- A tenant API credential defines the gRPC tenant context.
- A gRPC caller cannot override the credential tenant.
- Pingwin storage and operator responses contain no raw API secret.
- Each SMTP sender domain, identity, credential, and forwarding route belongs to one tenant.
- Tenant deletion removes all tenant-owned records in one transaction.
- A tenant configuration change takes effect without a service restart.
- Runtime configuration contains no tenant definitions or global gRPC token.
- Deployment inputs contain no tenant provider values or global gRPC token.
- Managed tenants use UUID identifiers and current schema references.
- The recorded recovery assigns all four tenants to the retained owner.
- The managed database contains no obsolete tenant domain or administrator data.
- The steady-state release contains no conversion or compatibility path.
- Current TAuth and `mpr-ui` authentication tests pass without a new Pingwin authentication flow.
- The complete `make ci` target passes.
