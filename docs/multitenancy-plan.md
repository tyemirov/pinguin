# Multitenancy Technical Plan

## Goals
- Allow a single Pinguin deployment to serve multiple logical customers (“tenants”) who access the system through their own domains / subdomains.
- Isolate data, delivery credentials, and admin access between tenants while reusing the same process, scheduler, and queues.
- Support both programmatic gRPC clients and the existing web dashboard/HTTP API without breaking current single-tenant users.

## High-Level Tenancy Model
1. **Tenant definition**  
   - Introduce a `tenants` table seeded via admin CLI with fields: `id (UUID)`, `slug`, `display_name`, `primary_domain`, `support_email`, `status`, `created_at`, `updated_at`.
   - Associate at least one `domain` (host name) per tenant for HTTP routing (`tenant_domains` table). Each entry stores `tenant_id`, `host`, `is_default`.
2. **Credential profiles**  
   - Move SMTP/Twilio settings into `delivery_profiles` tables:
     - `email_profiles (id, tenant_id, host, port, username, password, from_address, max_per_hour, is_default)`
     - `sms_profiles (id, tenant_id, account_sid, auth_token, from_number, is_default)`
   - Allow multiple profiles per tenant to support future per-channel overrides. Store encrypted credentials (using env-supplied AES key).
3. **Tenant membership**  
   - Replace the global `ADMINS` list with `tenant_members (tenant_id, email, role)`.  
     Roles: `owner`, `admin`, `viewer`, `integration` (for API tokens if needed).

## Context Propagation
1. **HTTP / Web**  
   - Map `Host` header to tenant via `tenant_domains`. `httpapi.Server` will inject `tenant_id` into request context before hitting handlers.
   - `<mpr-header>` stays tenant-specific by loading `/runtime-config`, which now returns `{ apiBaseUrl, tenantSlug }`. UI uses slug to show tenant name and to inform `mpr-header` which Google client ID to use.
   - Store per-tenant Google Identity client IDs and TAuth base URLs in `tenant_identity` table. Runtime config endpoint will emit the tenant-specific values.
2. **gRPC**  
   - Add `tenant_id` (string) to each gRPC request message (proto + generated code). Clients must set either:
     - Metadata header `x-tenant-id`, or
     - Field `tenant_id` in the request body (preferred once proto regenerated).
   - `buildAuthInterceptor` extracts tenant ID from metadata, verifies membership (if using bearer tokens per tenant), and injects it into context.
3. **Internal context**  
   - Define `tenant.ContextKey` and `TenantContext` domain type. All service methods accept `context.Context` carrying `tenant_id`.
   - Model functions gain explicit `tenantID` filter parameters to prevent accidental cross-tenant access.

## Data Model Changes
1. **Notifications**  
   - Add `tenant_id` (UUID FK) to `notifications` and `notification_attachments`.  
   - Unique constraint switches from `(notification_id)` to `(tenant_id, notification_id)` so each tenant can reuse IDs without collisions.
   - Update indexes: `(tenant_id, status, scheduled_for)` for worker scans, `(tenant_id, created_at)` for list pagination.
2. **Scheduler tables**  
   - Existing retry logic uses `notifications` only. Ensure scheduler queries include `tenant_id = ?`.
   - Add `tenant_dispatch_state` table to track per-tenant retry cursors / throttles (optional but recommended for fairness).

## Configuration & Secrets
1. **Bootstrap config**  
   - Keep global env vars for database, logging, etc.  
   - Store tenant seed data alongside service config in a single YAML file (default `configs/config.yml`). Provide CLI: `pinguin tenants import --file configs/config.yml`.
2. **Secrets management**  
   - Add `MASTER_ENCRYPTION_KEY` env for credential encryption-at-rest. Use AES-256-GCM to store SMTP/Twilio secrets in DB.
   - For migrations from current env-based config, supply `pinguin tenants migrate-single-tenant --from-env` CLI to create a default tenant using existing vars.

## HTTP / API Surface
1. **Runtime Config Endpoint (`/runtime-config`)**  
   - Response signature:  
     ```json
     {
       "apiBaseUrl": "...",
       "tenant": {
         "id": "tenant-uuid",
         "slug": "acme",
         "displayName": "Acme Corp",
         "identity": {
           "googleClientId": "xxx.apps.googleusercontent.com",
           "tauthBaseUrl": "https://auth.acme.example",
           "tauthTenantId": "acme-auth"
         }
       }
     }
     ```
2. **HTTP API**  
   - Embed tenant slug/ID in URL (`/api/:tenantSlug/notifications`) or infer from host header.  
   - Session middleware verifies:
     - Cookie from TAuth belongs to the host’s tenant (TAuth instance issues tokens scoped to tenant).
     - Claim email exists in `tenant_members`.
   - Handlers call service layer with `tenant_id`.
3. **gRPC Proto Updates**  
   - Add `string tenant_id = 99;` (chosen high field number) to every request message.
   - Provide `TenantService` RPC for managing tenant metadata (future enhancement).

## Service Layer & Scheduler
1. **NotificationService**  
   - Accepts a `TenantScopedConfig` resolved from DB each time based on context. Contains SMTP/Twilio senders built per tenant (with caching).  
   - Caches senders keyed by `(tenant_id, profile_id)` with TTL + eviction hooks.
   - Validates attachments and quotas per tenant (allow per-tenant size/volume limits).
2. **Retry Worker**  
   - Modify `StartRetryWorker` to iterate per tenant:  
     ```go
     for _, tenant := range tenantRepo.ListActiveTenants() {
         scheduler.ProcessTenant(ctx, tenant.ID)
     }
     ```
   - Optionally run workers per tenant goroutine to avoid long tail.
   - Metrics/logging include `tenant_id`.
3. **Repositories**  
   - Introduce `TenantRepository`, `DeliveryProfileRepository`, `TenantMemberRepository`.  
   - Existing `model` functions become tenant-aware or move under repository layer to keep domain logic cohesive.

## Web Assets
1. **Static bundle**  
   - Serve a single build where Alpine stores `tenant.displayName`, `tenant.identity` in a store.  
   - `<mpr-header>` attributes bound to tenant-specific values.
2. **Playwright tests**  
   - Extend tests to spin two tenants with different domains using the new Docker profile.  
   - Validate isolation: logging into tenant A cannot access tenant B data even with direct API calls.

## Observability & Operations
1. **Logging**: add `tenant_id`, `tenant_slug` to every structured log entry (zap/slog fields).  
2. **Metrics**: expose per-tenant counters (`notifications_sent_total{tenant="acme"}`) to detect noisy neighbors.  
3. **Rate limiting**: optional future extension – use token bucket per tenant.
4. **Admin tooling**: CLI commands for listing tenants, rotating credentials, and exporting audit trails.

## Migration Strategy
1. **Schema migrations**  
   - Use GORM auto-migrations or SQL migrations to add new tables/columns.  
   - Backfill existing notifications with default tenant ID (created via migration CLI) and update indexes.
2. **Config transition**  
   - Ship `pinguin migrate single-tenant --tenant-slug=default --domain=notifications.example` command that:
     - Creates tenant row.
     - Moves env SMTP/Twilio settings into delivery profile rows.
     - Imports `ADMINS` values as tenant members.
3. **Rollout**  
   - Deploy feature flags: `MULTITENANCY_ENABLED`.  
   - Stage 1: support tenant ID but default to single tenant when unspecified (backward compatibility).  
   - Stage 2: mandate tenant ID, deprecate legacy env vars.

## Testing Plan
1. **Unit tests** for tenant resolver, config loader, repositories, and service-level guardrails (e.g., cannot fetch other tenant’s notification).  
2. **Integration tests** using SQLite: create two tenants, enqueue notifications, ensure list APIs scoped per tenant.  
3. **Playwright e2e**: run via Docker stack with two ghttp hostnames. Validate host-based tenant resolution and admin allowlists.  
4. **Load tests**: verify scheduler fairness under mixed tenant workloads.

## Open Questions & Next Steps
1. **Per-tenant authentication source**: do all tenants reuse one TAuth instance with multiple client IDs, or separate TAuth deployments per tenant?  
   - Proposed: support both. `tenant_identity` stores base URL + client ID so we can mix-and-match.
2. **Billing/quota**: not part of this plan but data model allows future per-tenant quota enforcement.  
3. **Backups**: ensure backups include tenant metadata; consider per-tenant export CLI.

## Deliverables
1. Schema migrations + repositories + tenant resolver.  
2. Config/CLI for managing tenants and secrets.  
3. Updated proto/SDKs/README/docs describing multi-tenant usage.  
4. Test harness updates (Go + Playwright).  
5. Operational runbook for adding tenants and migrating existing deployments.
