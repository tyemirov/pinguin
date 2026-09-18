# ISSUES Archive

This file contains resolved non-recurring issue history.

The active [ISSUES.md](ISSUES.md) file contains current work.

## BugFixes

- [x] [B008] (P1) Restore protected requests after session expiry.
  Goal: Restore the tenant workspace after a recoverable authentication failure.
  Requirements: Use the shared authenticated transport. Preserve each mutation body and request identity during its retry.
  Validation: Both browser recovery scenarios failed after the first tenant read returned HTTP 401. Eight public candidate checks passed.
  Validation: The correction passed all ten candidate scenarios and final `make ci`, with 65 browser checks and 100 percent Go coverage.

- [x] [B007] (P1) The Pages source owns Gateway metadata.
  Goal:
  The Gateway adds the Pages metadata during release assembly.
  Validation:
  - Verify that the Pages source does not contain a reserved metadata path.
  - Run `make ci` after the last change.
  Resolution:
  The Pages source no longer contains Gateway-owned metadata.
  The deployment contract test rejects each reserved metadata path.
  The final CI passed all Go checks, 100 percent coverage, and 55 browser tests.

- [x] [B006] (P1) The browser pages use an obsolete LoopAware site identifier.
  Goal:
  Each browser page sends traffic data to the current Pinguin site.
  Validation:
  - Verify the LoopAware pixel URL on each browser page.
  - Run `make ci` after the last change.
  Resolution:
  Each browser page now uses the current Pinguin site identifier.
  The final CI passed all Go checks, 100 percent coverage, and 55 browser tests.

- [x] [B001] (P1) Package the production tenant conversion command
  Goal:
  The published image contains the command for the production data conversion.
  Validation:
  - Verify the image build contract.
  - Verify the production volume command in the conversion runbook.

- [x] [B002] (P1) Keep a credential for an uncertain write result
  Goal:
  The tenant workspace can retry the same credential write after an uncertain response.
  Validation:
  - Verify that a create retry uses the same credential and idempotency key.
  - Verify that a rotation retry uses the same credential and version.

- [x] [B003] (P1) Clear tenant workspace data after logout
  Goal:
  The tenant workspace contains no tenant data or credential data after logout.
  Validation:
  - Verify the forms, dialogs, tenant list, and temporary credential state after logout.

- [x] [B004] (P1) Remove a disabled SMS profile
  Goal:
  The tenant update removes the SMS profile when the user disables SMS delivery.
  Validation:
  - Verify the SMS profile deletion API.
  - Verify the tenant update in the browser.

- [x] [B005] (P1) Report credential storage errors as internal errors
  Goal:
  The gRPC API reports a storage fault separately from an invalid credential.
  Validation:
  - Verify an `Internal` result for a credential storage fault.
  - Verify an `Unauthenticated` result for an invalid credential.

## Improvements

- [x] [I003] (P1) Standardize HTTP health at `/healthz`.


  Goal:
  Make `/healthz` the canonical health endpoint for the Pinguin API and
  static web origins. Use the endpoint for readiness without application requests.

  Requirements:
  - Keep unauthenticated `GET /healthz` on the API origin.
  - Publish a static `/healthz` resource for the GitHub Pages origin.
  - Return `200` only when each origin can serve its current application contract.
  - Return a non-success status when a required runtime dependency prevents API service.
  - Send `Cache-Control: no-store` on API and local health responses.
  - Use the GitHub Pages cache policy for production static health responses.
  - Keep each response free from credentials and internal state.
  - Do not mutate application state during a probe.
  - Do not record a probe as application usage or an audit event.
  - Do not emit routine information-level request events for successful probes.
  - Keep failed probe evidence in container and deployment diagnostics.
  - Use `/healthz` for local Compose, runtime capability, and public health checks.
  - Set `start_interval: 1s` and `interval: 30s` for Docker probes.
  - Set a bounded `start_period` for the API startup contract.
  - Keep protocol-native readiness for gRPC and SMTP services.
  - Do not add HTTP mirrors for non-HTTP services.
  - Keep the selected manifest contract unchanged.

  Deliverables:
  - Update the API, static artifact, request logging, orchestration, manifest, documentation, and black-box tests.

  Validation:
  - Verify unauthenticated `GET /healthz` returns `200` on each origin.
  - Verify API and local health responses use `Cache-Control: no-store`.
  - Verify a required dependency failure returns a non-success API status.
  - Verify the static publication artifact contains `/healthz`.
  - Verify gRPC and SMTP readiness remain protocol-native.
  - Verify Docker probes use the required startup and steady intervals.
  - Verify successful probes create no routine request events.
  - Verify failed probes retain diagnostic evidence.
  - Run `make ci`.

  Cache policy:
  The operator approved the GitHub Pages cache-policy exception on 2026-09-04.
  This exception applies only to production static health responses.
  API and local health responses still require `Cache-Control: no-store`.

  Resolution:
  Implemented and verified API health, the static artifact, and readiness probes.
  Full local `make ci` passed. The approved cache exception removes the remaining blocker.

- [x] [I001] (P0) Use the permanent versionless selected application manifest
  Goal:
  Use one selected application manifest contract without a schema number.
  Requirements:
  - Remove `schema_version` from `.mprlab/deploy/resources.yml`.
  - Require only `owner`, `release`, and `resources` at the manifest root.
  - Reject each numbered selected application manifest form.
  - Preserve independent schema contracts.
  Validation:
  - Run `make ci` after the last repository change.
  - Plan release through gateway commit `753c727` without production contact.
  Resolution:
  - The manifest preserves the SemVer release scheme without a schema number.
  - The compiled deployment contract rejects a `schema_version` field.

## Planning

- [x] [P001] (P1) Define managed tenant configuration
  Goal:
  Define the current contract that lets a TAuth user manage Pinguin tenants without tenant YAML.
  Requirements:
  - Keep the current TAuth and `mpr-ui` authentication integration.
  - Define owner-scoped tenant management and tenant-bound programmatic access.
  - Define API secret storage that prevents secret recovery from Pinguin data.
  - Define one forward-only production data conversion.
  - Record unsupported product choices as open decisions.
  Deliverables:
  - Maintain the durable [managed tenant configuration plan](../docs/multitenancy-plan.md).
  - Record the confirmed architecture, implementation order, acceptance criteria, and open decisions.
  Validation:
  - Examine the plan against current configuration, HTTP, gRPC, database, browser, and deployment contracts.
  - Confirm that the plan adds no Pinguin authentication flow.
  - Confirm that the plan contains no compatibility runtime path.
  Resolved 2026-08-11:
  - Verified the plan against the current configuration, HTTP, gRPC, database, browser, and schema version 4 deployment contracts.
  - Replaced the action route with one notification resource patch contract.
  - Added protobuf reservation, exact schema validation, and UUID data conversion requirements.
  - Recorded P002 for product decisions and F001 for implementation.

- [x] [P002] (P1) Select managed tenant product contracts
  Goal:
  Select the remaining product contracts for managed tenant implementation.
  Requirements:
  - Select the required tenant state at creation.
  - Select suspension, deletion, or both tenant lifecycle operations.
  - Select one credential or multiple named credentials for each tenant.
  - Select tenant ownership or TAuth user ownership for SMTP resources.
  - Define retention and deletion behavior for the selected lifecycle.
  Deliverables:
  - Record one canonical selection for each open decision in the [managed tenant configuration plan](../docs/multitenancy-plan.md).
  - Remove the corresponding open-decision text after each selection.
  - Complete all selections.
  - Change F001 from blocked to open.
  Validation:
  - Confirm that each selection has one implementation path.
  - Confirm that the selections keep TAuth as the authentication service.
  - Confirm that the selections add no compatibility path.
  Resolved 2026-08-11:
  - Tenant creation requires one complete external email delivery profile and one API credential ID and digest.
  - The tenant lifecycle has permanent deletion and no suspension.
  - Each tenant has exactly one API credential with atomic rotation.
  - Each SMTP sender domain, identity, credential, and forwarding route belongs to one tenant.
  - Shared SMTP listeners and upstream relays remain service configuration.
  - F001 changed from blocked to open.
