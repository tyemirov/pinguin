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

- [x] [B009] (P1) {I007} Preserve the deployment owner during the Pingwin rename
  Goal: Deploy Pingwin with one permanent owner.
  Requirements: Keep the permanent owner `pinguin`, retained volume, database path, and encryption key.
  Requirements: Keep the current Pingwin service names and public interfaces.
  Deliverables: Correct the manifest, retirement declaration, contract test, and deployment guide.
  Validation: The v1.1.4 deployment reproduced a shared SMTP listener conflict at state revision 765.
  Validation: State inspection found owner `pinguin` generation 5 and runtime service `pinguin-api` in project `mprlab-pinguin-runtime`.
  Validation: The contract test failed before the manifest correction and passed after it.
  Validation: `make ci` passed with 100 percent Go coverage and 70 browser tests.
  Validation: The user authorized removal of Pingwin deployment records. The cleanup preserved application data and all other application records.
  Validation: The manifest uses route `pingwin-public-api` and the deployed Caddy execution policy.
  Validation: Native `make deploy` deployed `v1.1.7` at state revision `770`. API and website health checks returned `200`.
  Validation: The volume name, `DATABASE_PATH`, `MASTER_ENCRYPTION_KEY`, and `TAUTH_SIGNING_KEY` matched the previous runtime.
  Validation: B010 records the later public SMTP host-port correction.

- [x] [B010] (P1) Correct public SMTP host ports
  Goal: Expose the current SMTP listeners.
  Requirements: Keep public ports `25` and `465`. Publish host ports `8025` and `8465` through the manifest.
  Requirements: Preserve the router rules, HTTPS bindings, application data, and keys.
  Deliverables: Correct the listener declarations, contract test, and operational record.
  Validation: Public SMTP connections failed. Direct host connections returned `250` on ports `25` and `465`.
  Validation: The router forwards to host ports `8025` and `8465`. The manifest omitted both host ports.
  Validation: The contract test failed before the correction and passed after it. The installed Gateway plan accepted both host ports.
  Validation: `make ci` passed with 100 percent Go coverage and 70 browser tests.
  Validation: Native `make deploy` deployed `v1.1.8` at state revision `774`.
  Validation: Public SMTP and SMTPS `NOOP` commands returned `250`. TLS certificate verification passed on port `465`.
  Validation: API and website health returned `200`. All ten router rules and HTTPS bindings remained unchanged.
  Validation: The database volume, database path, and protected inputs were preserved.

- [x] [B011] (P1) Keep the production TAuth tenant identity
  Goal: Keep the production browser on its permanent TAuth tenant.
  Requirements: Keep tenant `pinguin`, its user IDs, signing keys, and cookie names.
  Requirements: Use `https://pingwin.mprlab.com` as the production browser origin.
  Deliverables: Correct the production browser config, contract test, and deployment instructions.
  Validation: The production browser used tenant `pingwin`, but the permanent TAuth tenant is `pinguin`.
  Validation: The contract test failed before the config change and passed after it.
  Validation: `make ci` passed with 100 percent Go coverage and 70 browser tests.
  Validation: TAuth activated revision `2` with the current browser origin. The new-origin preflight returned `204`.
  Validation: Google saved the current JavaScript origin. Hosted sign-in succeeded, and all six existing tenants remained available.
  Validation: Native `make deploy` deployed `v1.1.10` at state revision `788`, generation `4`, with six verified resources.
  Validation: Public SMTP checks returned `250`. API and website health checks returned `200`. Protected runtime inputs were preserved.

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

- [x] [I009] (P1) Configure the Pingwin mail subdomain
  Goal: Receive mail for `support@pingwin.mprlab.com` and forward it to `marcopoloresearchlab@gmail.com`.
  Requirements: Preserve the apex domain mail records and all unrelated tenant records.
  Deliverables: Configure DNS, a tenant, a verified sender domain, and the forwarding identity.
  Validation: The local Squarespace TOTP helper passed provider verification.
  Validation: All four authoritative nameservers returned the website A records and the new MX record. All unrelated registrar records were unchanged.
  Validation: The dedicated Pingwin tenant and SMTP identity were created after user approval. The three sender domain checks passed.
  Validation: The saved identity forwards to `marcopoloresearchlab@gmail.com`. Public SMTP accepted the recipient with `250` but returned `451` after message data.
  Validation: The September 30 receipt confirms PoodleScanner delivery through MPR Lab account `299335570791`.
  Validation: The previous shared SMTP credentials belonged to account `297841131854`.
  Validation: SES verified `pingwin.mprlab.com` and its DKIM records in account `299335570791`. All twelve authoritative DNS checks passed.
  Validation: The approved SMTP principal permits six exact sender addresses. TLS verification, SMTP authentication, and all six sender-policy checks passed.
  Validation: The canonical relay inputs and the Pingwin tenant profile contain the validated credential. The other six tenant profiles remain unchanged.
  Validation: Native `make deploy` completed for `v1.1.11` at state revision `790`, generation `5`, with six verified resources.
  Validation: Public port `25` forwarded the message. Public port `465` authenticated the SMTP identity and accepted the outbound message.
  Validation: Both test messages reached the destination Gmail inbox. SPF, DKIM, and DMARC passed for both messages.
  Validation: All seven tenants and six SMTP identities remained available. The existing PoodleScanner SMTP principal and all unrelated DNS records remained unchanged.

- [x] [I008] (P1) Use one production deployment command
  Goal: Use `make deploy` for the complete Gateway pipeline.
  Requirements: Call the installed `deploy` command. Remove the separate release and publication targets.
  Requirements: Preserve the current manifest, internal policy identifiers, private inputs, and runtime resources.
  Deliverables: Update the Makefile, command test, and deployment instructions.
  Validation: Gateway v9.2.1 exposes `deploy` and retains the internal `app-*` policy identifiers.
  Validation: The command test failed because `make release` remained available.
  Validation: The corrected command test passed. It verifies Gateway selection, closed standard input, CI mode, and failure propagation.
  Validation: The installed Gateway planner accepted the current manifest. The manifest required no changes.
  Validation: `make ci` passed with 100 percent Go coverage and 70 browser tests.

- [x] [I007] (P1) Change the service name to Pingwin
  Goal: Use `pingwin` for the current service contract.
  Requirements: Preserve source repository paths, retained data identities, and historical records.
  Deliverables: Update executable names, gRPC definitions, browser assets, SMTP names, configuration, and deployment declarations.
  Validation: `make ci` passed with 100 percent Go coverage and 70 browser tests. CLI, gRPC, container health, doctor, and installed Gateway checks passed.
  Validation: The Governor check reported the same 16 findings before and after this change. Native `make deploy` deployed `v1.1.7`.

- [x] [I006] (P1) Use the `theme-control` contract
  Goal: Use the `mpr-ui` contract on each page.
  Requirements: Keep each mode, position, label, and initial selection.
  Validation: The browser suite and `make ci` passed with `mpr-ui` assets from `v4.3.5`. All 70 browser scenarios passed.

- [x] [I004] (P1) {I009@https://github.com/MarcoPoloResearchLab/mpr-ui} Adopt the shared authentication and footer contracts.
  Goal: Preserve the four browser pages with the current shared library.
  Requirements: Declare each provider. Preserve the session endpoint and identifiers. Replace the obsolete footer menu attribute.
  Deliverables: Prepare source changes, browser checks, integration instructions, and a pull request.
  Validation: Final `make ci` passed with 100% Go coverage and 63 browser checks. Eight candidate checks failed before migration.
  Validation: Final B069 qualification passed `make ci`, with 65 browser checks and 100 percent Go coverage. Ten candidate scenarios verify all four pages and protected recovery. B008 preserves mutation payloads and request identities after authentication recovery.
  Validation: Source acceptance is completed. Publication, cache checks, and hosted Google acceptance remain in the [activation record](../docs/mpr-ui-migration.md#activation).
  Validation: On 2026-11-18, all 70 browser tests passed. The configuration and integration tests also passed.

- [x] [I005] (P1) Standardize HTTP health at `/healthz`.
  Goal:
  Make `/healthz` the canonical health endpoint for the Pinguin API and
  static web origins. Use the endpoint for readiness without application requests.

  Requirements:
  - Keep unauthenticated `GET /healthz` on the API origin.
  - Publish a static `/healthz` resource for the GitHub Pages origin.
  - Return `200` only when each origin can serve its current application contract.
  - Return a non-success status when a required runtime dependency prevents API service.
  - Send `Cache-Control: no-store` on every health response.
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
  - Verify unauthenticated `GET /healthz` returns `200` and `Cache-Control: no-store` on each HTTP origin.
  - Verify a required dependency failure returns a non-success API status.
  - Verify the static publication artifact contains `/healthz`.
  - Verify gRPC and SMTP readiness remain protocol-native.
  - Verify Docker probes use the required startup and steady intervals.
  - Verify successful probes create no routine request events.
  - Verify failed probes retain diagnostic evidence.
  - Run `make ci`.
  Validation: This request duplicates I003. I003 defines the current health contract and the approved GitHub Pages cache exception.
  Validation: The original request above does not replace that exception. See the [health guide](../docs/health.md).
  Validation: On 2026-11-18, the API, static health, and configuration tests passed.

- [x] [I002] (P2) Normalize the managed governance sections.
  Goal:
  The managed governance sections match the current Governor templates.
  Validation:
  - Run the Governor check.
  - Run `git diff --check`.
  Validation: The 2026-11-18 check found drift in the Docker guide, Git guide, planning guide, and core policy.
  Validation: Preserve the existing local changes when I002 reconciles these four files.
  Validation: On 2026-11-18, the Governor check passed with no managed differences.
  Validation: The repository policy source retains the four local validation sections without changes.
  Validation: The core policy controls validation when the Git or planning guide differs.

## Features

- [x] [F001] (P1) {P002} Implement managed tenant configuration
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
  Validation: Historical notes from the review before source cleanup follow.
  Validation: The managed tenant source and automated acceptance suite are completed.
  Validation: The remaining source change must remove the bounded conversion command, its package, tests, and image entry.
  Validation: The [conversion record](../docs/multitenancy-plan.md#production-data-conversion) retains owner confirmation, conversion, and production acceptance requirements.
  Validation: Review that record before the source cleanup. Run the complete code validation after the cleanup.
  Validation: Retirement results follow. The September 15 records confirm authorized recovery of all four tenants under the retained owner.
  Validation: The original conversion execution and historical notification preservation remain unverified.
  Validation: The obsolete command, package, tests, and image executable are removed.
  Validation: The image contract failed before removal and passed after removal.
  Validation: The final code gate is `make ci`. SMS acceptance remains deferred by the user.

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
