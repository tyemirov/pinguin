# Managed Tenant Configuration

## Status

This document defines the technical plan for managed tenant configuration.

The plan is under review in [P001](../.mprlab/ISSUES.md).

The document records confirmed requirements, proposed interfaces, implementation work, and open decisions.

## Purpose

Pinguin currently reads tenant definitions from YAML during service start.

The target product lets an authenticated user create and manage tenant configuration in Pinguin.

The target product does not require an operator to edit tenant YAML or restart the service.

## Confirmed Requirements

- Keep the current TAuth and `mpr-ui` authentication integration.
- Use the validated TAuth user ID as the Pinguin owner identity.
- Let each authenticated user establish their Pinguin tenants.
- Store tenant configuration in the Pinguin database.
- Remove tenant definitions from the runtime configuration file.
- Give each tenant an API credential for programmatic notification requests.
- Keep the API secret unavailable in stored data and normal operator interfaces.
- Keep one forward-only runtime contract.
- Use one bounded production data conversion before the managed contract starts.
- Remove the conversion code after the production conversion.

## Scope Boundary

TAuth remains the authentication service.

TAuth already provides login, logout, session restoration, and session validation.

Pinguin consumes the validated session and owns tenant authorization.

This plan does not add Pinguin login code, TAuth account code, or browser token code.

This plan removes tenant data from YAML.

Pinguin still uses service configuration for listeners, the database, logging, TAuth validation, and the master encryption key.

A separate change can replace the service configuration transport if that outcome becomes necessary.

## Current State

### Tenant source

`internal/config/config.go` reads `TenantConfigPath` or inline `TenantBootstrap` data.

`cmd/server/main.go` requires one source and runs tenant bootstrap during each service start.

`internal/tenant/bootstrap.go` treats the configured tenant list as the source of truth.

The bootstrap operation resets profiles and removes tenants that are absent from the file.

### Browser authentication

`web/config-ui.yaml` and `mpr-ui-config.js` configure the shared browser shell.

`web/js/core/sessionBridge.js` consumes the existing `mpr-ui` authentication events and profile snapshot.

`internal/httpapi/server.go` validates the TAuth session cookie with `sessionvalidator`.

This completed integration remains the authentication boundary.

### Browser authorization

The HTTP API currently authorizes tenant access through configured administrator email addresses, user roles, and email domains.

The browser supplies a tenant ID for notification operations.

The server compares that ID with the tenants that the session can use.

### Programmatic authorization

The gRPC server currently compares one deployment-wide bearer token.

The caller also supplies a tenant ID in request fields or `x-tenant-id` metadata.

A valid deployment token can select any configured tenant.

### Stored data and secrets

The database already stores tenants, notifications, email profiles, and SMS profiles.

Pinguin encrypts provider credentials with the service master key.

Pinguin must decrypt provider credentials to deliver email and SMS messages.

SMTP sender domains and identities currently use an owner email address instead of a tenant ID.

### Deployment inputs

`configs/config.production.yml` contains tenant definitions and the global gRPC token reference.

`.mprlab/deploy/resources.yml` declares private inputs for each configured tenant and the global token.

As a result, tenant changes require operator input and a new service configuration.

## Target Ownership Model

The HTTP session gives Pinguin a validated TAuth user ID.

Pinguin uses that value as `owner_user_id`.

The browser does not submit `owner_user_id`.

Each tenant has one immutable owner value.

One owner can have more than one tenant.

Every tenant query includes both `owner_user_id` and `tenant_id`.

Pinguin returns `404` when the tenant does not exist or belongs to a different owner.

This response does not disclose the existence of another owner's tenant.

## Target Data Model

### Tenant

Add these current fields to the tenant record:

- `id`: A server-generated UUID.
- `owner_user_id`: The immutable validated TAuth user ID.
- `display_name`: The tenant name for the user interface.
- `display_name_key`: The normalized name for an owner-scoped unique index.
- `support_email`: An optional validated support address.
- `status`: The current tenant state.
- `created_at`: The creation time.
- `updated_at`: The last change time.

Remove `TenantDomain` and `TenantAdmin` from the managed schema.

Remove host and email-domain authorization from the managed contract.

### Delivery profiles

Keep `EmailProfile` and `SMSProfile` as tenant-owned records.

Store SMTP usernames and passwords as encrypted values.

Store Twilio account identifiers and tokens as encrypted values.

Return only safe profile metadata through the management API.

Require a replacement secret when the user changes a stored provider secret.

Do not accept a masked value as a replacement secret.

### API credential

Add an `APICredential` record with these fields:

- `id`: The public credential UUID.
- `tenant_id`: The tenant foreign key.
- `name`: The owner-facing credential name.
- `secret_digest`: The 32-byte SHA-256 digest.
- `display_prefix`: A safe key prefix for identification.
- `created_at`: The creation time.
- `last_used_at`: The last successful authentication time.
- `revoked_at`: The optional revocation time.

Use a versioned credential format such as `pgn_1_<credential-id>_<secret>`.

Use at least 256 random bits for the secret.

The browser creates the secret with Web Crypto.

The browser calculates the digest and submits only the digest and safe metadata.

Pinguin stores the digest and compares it in constant time.

Pinguin never stores, returns, or logs the raw API secret.

The gRPC server receives the bearer value during request authentication.

A bearer protocol cannot hide the presented value from the authentication process.

This contract makes the secret unavailable in stored data and normal operator interfaces.

## Proposed HTTP Contract

All management routes use the existing TAuth session middleware.

The HTTP boundary creates validated owner, tenant, profile, and credential domain values.

### Tenant routes

- `GET /api/tenants`
- `POST /api/tenants`
- `GET /api/tenants/:tenant_id`
- `PUT /api/tenants/:tenant_id`
- `DELETE /api/tenants/:tenant_id`

`POST` derives the owner from the session and creates a server-owned tenant ID.

`PUT` keeps the owner and tenant ID unchanged.

Responses return safe profile state and do not return provider secrets.

### Credential routes

- `GET /api/tenants/:tenant_id/api-credentials`
- `POST /api/tenants/:tenant_id/api-credentials`
- `DELETE /api/tenants/:tenant_id/api-credentials/:credential_id`

The list response contains safe metadata only.

The create request contains a credential ID, name, prefix, and digest.

The raw key remains in browser memory until the user closes the one-time dialog.

Secret-related responses use `Cache-Control: no-store`.

### Tenant resource routes

Move notification routes under the authorized tenant:

- `GET /api/tenants/:tenant_id/notifications`
- `PATCH /api/tenants/:tenant_id/notifications/:notification_id/schedule`
- `POST /api/tenants/:tenant_id/notifications/:notification_id/cancel`

If SMTP resources become tenant resources, move their routes under the same tenant path.

Delete replaced global routes after the new contract starts.

Do not add route aliases or redirects.

### Error contract

- Return `400` for invalid JSON or invalid domain input.
- Return `401` for an invalid TAuth session.
- Return `404` for an absent or foreign tenant resource.
- Return `409` for a unique-value or notification-state conflict.
- Return `422` for an invalid provider configuration combination.
- Return `500` for an internal boundary failure.

Return a stable error code with each expected error.

Keep secret values out of errors and logs.

## Proposed gRPC Contract

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

Regenerate the Go gRPC package with the repository Make target.

Change `pkg/client.Settings` to accept a server address and one API key.

Remove `TenantID()`, request tenant mutation, and `x-tenant-id` metadata.

Replace the CLI `--grpc-auth-token` and `--tenant-id` flags with `--api-key`.

Update the `grpcurl`, CLI, and Go client examples.

## Browser Work

Keep the current shared header, footer, authentication events, and session snapshot.

Add a tenant management destination to the shared application navigation.

Load tenant data only after the existing authenticated event or snapshot.

Show a managed empty state when the authenticated user has no tenants.

Add semantic Alpine components for these tasks:

- List owned tenants.
- Create a tenant.
- Change tenant metadata and delivery profiles.
- Delete or suspend a tenant.
- Create and copy an API key one time.
- List safe credential metadata.
- Revoke an API credential.

Clear the raw key from component state when the one-time dialog closes.

Use a single-flight state for destructive requests.

Restore focus to a stable control after each dialog closes.

Move Event log API calls to the selected owner tenant path.

Move SMTP relay API calls when the SMTP ownership decision includes those resources.

## Backend Work

### Domain and repository work

1. Add smart constructors for owner IDs, tenant IDs, names, statuses, profiles, credential IDs, and credential digests.
2. Add owner fields and unique indexes to the tenant model.
3. Add the API credential model and tenant foreign key.
4. Add transactional tenant create, list, read, update, and delete operations.
5. Add credential create, list, revoke, and verify operations.
6. Add runtime profile cache invalidation after each tenant change.
7. Delete bootstrap, domain, and configured administrator queries.

### HTTP work

1. Create `OwnerUserID` from validated TAuth claims at the HTTP boundary.
2. Add the tenant and credential handlers.
3. Add owner checks to all nested resource handlers.
4. Make `/runtime-config` independent from a tenant host.
5. Delete email-domain, configured administrator, role, and host authorization.

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
4. Remove tenant and token validation from `pinguin-doctor`.
5. Remove tenant and token values from tracked configuration templates.
6. Remove tenant and token private inputs from `.mprlab/deploy/resources.yml`.
7. Keep only service-level runtime configuration in the files.

## Test Work

Add black-box HTTP tests with two TAuth users and two tenant sets.

Prove that each user can use only their tenant resources.

Add gRPC tests with two credentials and two tenants.

Prove that a credential cannot select another tenant.

Prove that revoked credentials fail on the next request.

Prove that the database does not contain a raw API secret.

Add browser tests for these flows:

- An authenticated user has no tenants.
- The user creates the first tenant.
- The user changes a tenant configuration.
- The user creates and copies an API key one time.
- The user revokes an API credential.
- The user deletes or suspends a tenant.
- Direct access to another owner's tenant returns the expected result.
- Dialog focus and request state remain accessible.

Add configuration tests that reject removed tenant and token keys.

Add a start test for an empty tenant database.

Add deployment tests that reject tenant-specific private inputs.

Run focused Make targets during implementation.

Run `make ci` after the final implementation change.

## Production Data Conversion

Use one operator command for the production conversion.

The command reads the current tenant YAML and an explicit TAuth owner mapping.

The mapping gives each current tenant ID one TAuth user ID.

The command also maps each SMTP resource if the final ownership model includes those resources.

The command validates the complete mapping before it changes the database.

The command completes these operations in one transaction:

1. Add managed owner values to current tenants.
2. Keep tenant IDs that current notifications reference.
3. Store current provider secrets with the current encryption contract.
4. Convert each included SMTP resource to tenant ownership.
5. Examine row counts, foreign keys, and encrypted profile access.

The global gRPC token cannot become a tenant credential.

Each owner creates a new tenant API credential for the managed contract.

Use a write-stopped maintenance window for the conversion and deployment.

Do not run old and new authorization contracts at the same time.

Delete the conversion command and mapping input after production acceptance.

## Implementation Order

1. Add failing HTTP, gRPC, browser, configuration, and deployment contract tests.
2. Add managed tenant and credential domain types.
3. Add the database schema and owner-scoped repositories.
4. Add managed HTTP tenant and credential APIs.
5. Replace gRPC authentication and update clients.
6. Add the managed tenant browser interface.
7. Apply the final SMTP resource ownership decision.
8. Add and run the bounded production data conversion.
9. Remove tenant YAML, global token input, bootstrap, and old routes.
10. Update current documentation and run the final `make ci` gate.

## Open Decisions

### Tenant creation state

Decide whether tenant creation requires a complete email profile.

The alternative is a valid draft tenant that cannot send notifications.

### Tenant lifecycle

Decide whether an owner can suspend, delete, or use both operations.

Define notification retention and resource deletion for the selected operation.

### API credential count

Decide whether a tenant has one credential or multiple named credentials.

Multiple credentials give separate revocation for each client.

### API secret privacy

Confirm whether service-owner invisibility means storage and operator interface protection.

Strict cryptographic invisibility requires asymmetric request signatures instead of a bearer API key.

### SMTP ownership

Decide whether sender domains, identities, and forwarding routes belong to a tenant or directly to a TAuth user.

The selected owner model must control all SMTP management routes and database foreign keys.

### Existing owner mapping

Identify the TAuth user ID that will own each current production tenant.

Identify the target tenant for each current SMTP sender domain and identity.

## Acceptance Criteria

- An existing TAuth user can create a Pinguin tenant without an operator change.
- A new Pinguin database can start with zero tenants.
- An owner can list and change only their tenants.
- A foreign tenant ID does not disclose foreign tenant data.
- A tenant API credential defines the gRPC tenant context.
- A gRPC caller cannot override the credential tenant.
- Pinguin storage and operator responses contain no raw API secret.
- A tenant configuration change takes effect without a service restart.
- Runtime configuration contains no tenant definitions or global gRPC token.
- Deployment inputs contain no tenant provider values or global gRPC token.
- The production data conversion keeps required notification references.
- The steady-state release contains no conversion or compatibility path.
- Current TAuth and `mpr-ui` authentication tests pass without a new Pinguin authentication flow.
- The complete `make ci` target passes.
