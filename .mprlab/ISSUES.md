# ISSUES

Entries record newly discovered requests or changes.
Resolved non-recurring issues are in [ISSUES.archive.md](ISSUES.archive.md).

## BugFixes

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

- [-] [I009] (P1) Configure the Pingwin mail subdomain
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
  Validation: The canonical relay inputs and the Pingwin tenant profile contain the validated credential. Existing tenant profiles remain unchanged.
  Validation: Complete native deployment, public SMTP submission, and Gmail delivery acceptance.

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

- [!] [I004] (P1) {I009@https://github.com/MarcoPoloResearchLab/mpr-ui} Adopt the shared authentication and footer contracts.
  Goal: Preserve the four browser pages with the current shared library.
  Requirements: Declare each provider. Preserve the session endpoint and identifiers. Replace the obsolete footer menu attribute.
  Deliverables: Prepare source changes, browser checks, integration instructions, and a pull request.
  Validation: Final `make ci` passed with 100% Go coverage and 63 browser checks. Eight candidate checks failed before migration.
  Validation: Final B069 qualification passed `make ci`, with 65 browser checks and 100 percent Go coverage. Ten candidate scenarios verify all four pages and protected recovery. B008 preserves mutation payloads and request identities after authentication recovery.
  Blocked: mpr-ui I009 must complete coordinated publication and cache qualification. The owner must complete real Google acceptance.

- [ ] [I005] (P1) Standardize HTTP health at `/healthz`.
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

- [ ] [I002] (P2) Normalize the managed governance sections.
  Goal:
  The managed governance sections match the current Governor templates.
  Validation:
  - Run the Governor check.
  - Run `git diff --check`.

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

- [ ] [F002] (P1) {F001} Add provider-backed tenant mailboxes
  Goal:
  Let a tenant provision user mailboxes for send and receive operations on a verified domain.
  Requirements:
  - Keep Pinguin focused on tenant email identity and delivery infrastructure.
  - Use one selected mailbox provider as the canonical storage and protocol boundary.
  - Require a verified sender domain before mailbox creation.
  - Create each mailbox for one exact address and one mailbox user.
  - Provision inbound delivery, persistent storage, and the provider access interface.
  - Provision authenticated SMTP submission for the mailbox address.
  - Keep message bodies and mailbox retention in the selected provider.
  - Do not add a Pinguin webmail client in the first release.
  - Keep mailbox users separate from shared forwarding addresses.
  - Keep notification delivery independent from mailbox delivery.
  - Store provider identifiers and credentials under the current tenant encryption and secret-response rules.
  - Define one-time activation and credential recovery without logging raw credentials.
  - Remove each provider mailbox during permanent tenant deletion.
  - Reject unsupported provider capabilities at the API boundary.
  Deliverables:
  - Define the canonical mailbox domain types, persistence schema, HTTP routes, and browser workflow.
  - Implement one mailbox provider adapter and its tenant-scoped lifecycle operations.
  - Add provider sandbox or local protocol integration tests for provisioning, sending, receiving, isolation, failure, and deletion.
  - Define quota, retention, abuse, bounce, and provider-outage operational rules.
  - Update `README.md` and `ARCHITECTURE.md` with the mailbox contract and scope boundary.
  - Add live provider qualification steps after local validation passes.
  Validation:
  - Verify that an unverified domain cannot create a mailbox.
  - Verify that one tenant can create, use, and delete one mailbox through public API entry points.
  - Verify that inbound mail reaches persistent mailbox storage.
  - Verify that a mailbox user can send mail through authenticated submission.
  - Verify tenant isolation and mailbox authorization.
  - Verify that provider failures return contextual errors and create no partial mailbox state.
  - Verify that secrets do not appear in API responses or logs.
  - Verify that shared forwarding and notification delivery remain unchanged.
  - Run `make ci`.

## Planning
