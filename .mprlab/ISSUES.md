# ISSUES

Entries record newly discovered requests or changes.
Resolved non-recurring issues are in [ISSUES.archive.md](ISSUES.archive.md).

## BugFixes

## Improvements

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
