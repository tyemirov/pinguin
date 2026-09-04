# ISSUES

Entries record newly discovered requests or changes.

## BugFixes

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

- [ ] [I002] (P2) Normalize the managed governance sections.
  Goal:
  The managed governance sections match the current Governor templates.
  Validation:
  - Run the Governor check.
  - Run `git diff --check`.

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

## Maintenance

## Features

- [!] [F001] (P1) {P002} Implement managed tenant configuration
  Goal:
  Let a TAuth user manage Pinguin tenants and tenant API access without tenant YAML.
  Requirements:
  - Keep the current TAuth and `mpr-ui` authentication integration.
  - Derive each tenant owner from the validated TAuth user ID.
  - Store tenant configuration in the Pinguin database.
  - Require one complete external email delivery profile and one API credential during tenant creation.
  - Store only the API credential digest.
  - Provide permanent tenant deletion that removes all tenant-owned records.
  - Make SMTP sender domains, identities, credentials, and forwarding routes tenant-owned.
  - Remove tenant YAML, the global gRPC token, and gRPC caller-selected tenant IDs.
  - Keep one forward-only runtime contract.
  Deliverables:
  - Implement the [managed tenant configuration plan](../docs/multitenancy-plan.md).
  - Add the managed HTTP API, browser interface, gRPC authentication, and client contract.
  - Add tenant-owned SMTP management under tenant resource routes.
  - Add the exact managed schema and the bounded production data conversion command.
  - Assign all current production tenants to the TAuth account for `temirov@gmail.com`.
  - Remove the conversion command after user-owned production acceptance.
  Validation:
  - Pass all acceptance criteria in the managed tenant configuration plan.
  - Validate the conversion with production-shape fixtures.
  - Pass the complete `make ci` target.
  Blocked:
  - The production operator must confirm the TAuth user ID for `temirov@gmail.com`.
  - The production operator must run the conversion and accept the managed runtime.
  - After acceptance, remove the bounded conversion command and archive F001.

## Planning
