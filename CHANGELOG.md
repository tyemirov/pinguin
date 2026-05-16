# Changelog

## Unreleased

### Features
- Add authenticated sender-domain DNS setup for SMTP relay, including exact DNS records, manual DNS checks, verified-domain identity creation, and owner-scoped relay management for non-admin users.
- Allow admins to reopen existing SMTP relay credentials in the Gmail SMTP settings modal, with passwords stored encrypted at rest and rotation available inside the modal.
- Add UI/API-driven inbound SMTP forwarding for shared SMTP identities, with required forwarding owners, no mailbox storage, and immediate fanout through a configured relay.
- Decouple authenticated SMTP submission from notification tenants by giving it its own sender-domain allowlist, identity credentials, and upstream relay profile.
- Add an authenticated tenant switcher so Pinguin admins can load active tenants and view each tenant's notification events from the dashboard.
- Add backend-backed search and infinite scroll for dashboard notification events, including cursor pagination and a single top-level refresh control.

### Bug Fixes
- Prune dead sender-domain config cleanup, SMTP identity credential migration, the obsolete `failed` notification status path, and the old `dashboard.html` redirect so current runtime behavior has no compatibility shims.
- Make SMTP sender domains fully API/DB-owned by rejecting unknown static sender-domain config, removing static config/env references, and surfacing sender-domain API validation errors in the SMTP relay UI.
- Move the gateway-executed GitHub Pages deployment resource into `deploy/app.yml` so Pinguin owns the deployable frontend contract while gateway Ansible owns execution.
- Align production workflow contract tests and docs with the current release branch defaults and `deploy-pinguin-backend` gateway target.
- Require `make release`, `make publish`, and `make deploy` to run only from clean local `master` matching `origin/master` with zero open PRs, and print the verified branch/commit before production work starts.
- Accept SMTP forwarding `MAIL FROM:<>` null reverse-path traffic so DSNs and auto-generated loop-safe messages can reach configured shared-address routes.
- Replace the generated placeholder logo with the canonical Pinguin turquoise envelope mark.
- Enforce tenant authorization before honoring `tenant_id` on notification list, reschedule, and cancel endpoints.
- Require the TAuth `admin` role before listing, creating, rotating, or deleting global SMTP identities.
- Honor configured tenant admin emails for dashboard tenant access and global SMTP identity management.
- Restore the deploy script's gateway handoff to the generic `deploy TARGET=pinguin` dispatcher before legacy Pages publication.
- Remove stale tenant bootstrap records so deleted tenants no longer leave active admin access behind.
- Keep the landing and dashboard header branded as `[logo] Pinguin` even when runtime tenant metadata belongs to a notification consumer, and serve the Pinguin favicon from `/favicon.svg`.
- Restore production login rendering by moving the frontend onto the `mpr-ui` `/config-ui.yaml` orchestration contract and removing the direct `tauth.js` loader.
- Align the local Docker browser origin with the configured Google OAuth client by moving the UI to `http://localhost:8080`, the API to `http://localhost:8081`, and TAuth to `http://localhost:8082`.
- Remove the duplicate landing-page auth controller so local login initializes Google Identity once.
- Remove Pinguin's duplicate account chip so the authenticated header uses the single shared `mpr-ui` user menu.
- Publish `pinguin-doctor` in the production image and make the server the default command so gateway Compose preflight can run the doctor binary.

### Improvements
- Replace the old landing page with a focused Pinguin sign-in screen and notification queue preview.
- Add a dashboard horizontal menu using `mpr-ui` header links for Event log and SMTP relay.
- Split the authenticated browser workspace into dedicated `event-log.html` and `smtp-relay.html` pages, removing the combined dashboard header copy.
- Rename the SMTP identity dashboard surface to SMTP relay while keeping exact sender identity management in that view.
- Add `make up` and `make down` wrappers for the local Docker Compose orchestration.
- Add split `configs/.env.pinguin.example` and `configs/.env.tauth.example` files for the current Compose topology.
- Align Pinguin SMTP setup with gateway high-port publishing so MX and SMTPS use `8025` and `8465` on the host.

### Testing
- Add production workflow contract coverage for the shared release/publish/deploy Git guard.
- Add backend and browser coverage for sender-domain DNS setup, manual checks, and verified-domain SMTP identity creation.
- Add backend and browser coverage for retrieving existing SMTP relay credentials and rotating them from inside the modal.
- Add black-box SMTP forwarding coverage for accept/forward, unknown-recipient rejection, size limits, relay failures, and startup wiring.
- Add backend and browser coverage for notification search, cursor pagination, infinite scroll, and the GORM-only query contract.
- Add browser coverage for the dashboard horizontal Event log / SMTP relay menu.
- Add backend and browser coverage for the separate Event log and SMTP relay page contract.
- Add backend coverage for admin-wide notification tenant access and non-admin email-domain tenant restrictions.
- Add backend coverage proving non-admin SMTP identity routes return 403 before touching identity storage.
- Add backend coverage for configured tenant admin authorization.
- Add backend coverage for pruning tenant bootstrap records that disappear from config.
- Add backend and browser coverage for explicit tenant notification listing and dashboard switching between tenant event views.
- Add browser coverage for the Pinguin logo/favicon header contract, including a regression where runtime config returns `PoodleScanner` tenant metadata.
- Add browser coverage for the landing header login path and for the `mpr-ui@latest` config contract.
- Update profile-menu browser coverage to assert the shared `mpr-user` header contract instead of the removed local settings menu.
- Restore Go statement coverage to 100.0% across all covered packages.
- Add a `make ci` coverage gate that fails unless total Go statement coverage remains at 100.0%.
- Add deployment contract coverage for the sibling gateway SMTP host-port configuration.

### Docs
- Document the production Git-state requirements for release, publish, and deploy commands.
- Document that Pinguin does not support backward compatibility, legacy schemas, legacy data, or fallback code paths.
- Update SMTP relay docs to describe reusable credential viewing instead of one-time password display.
- Document shared-address forwarding DNS setup and verification using `mx.pinguin.mprlab.com`.
- Update README and architecture notes to describe `config-ui.yaml` as the browser auth source of truth.
- Update the dashboard docs to describe the authenticated event log and SMTP relay surfaces.
- Update README and architecture notes to describe the split Event log and SMTP relay page destinations.
- Document dashboard tenant authorization roles and non-admin domain scoping.
- Document the remaining edge mappings after gateway deployment: `25 -> tutosh:8025` and `465 -> tutosh:8465`.

## [v0.4.17] - 2026-05-16

### Features ✨
- _No changes._

### Improvements ⚙️
- Prune dead sender-domain config cleanup, SMTP identity credential migration, obsolete notification status path, and legacy dashboard redirect to remove compatibility shims.
- Make SMTP sender domains fully API/DB-owned by rejecting unknown static config and removing static references; surface validation errors in SMTP relay UI.
- Update internal doctor logic, related tests, and clean startup by deleting legacy NULL-owner SMTP sender-domain rows.
- Align production deploy resources and workflows with current release branch and contract defaults.
- Enforce tenant authorization on notification list, reschedule, and cancel endpoints.
- Accept SMTP forwarding with `MAIL FROM:<>` null reverse-path for DSNs and loop-safe messages.
- Brand landing and dashboard headers with Pinguin logo and favicon consistently.
- Move frontend loading to `mpr-ui` orchestration and update local Docker origins to align with Google OAuth client.

### Bug Fixes 🐛
- Fix startup crash caused by legacy NULL-owner configured SMTP sender-domain rows.
- Correct status mapping by removing deprecated `failed` notification handling.
- Restore production login rendering and dashboard tenant admin email handling.
- Remove stale tenant bootstrap records to prevent lingering admin access.

### Testing 🧪
- Add backend and browser coverage for sender-domain DNS setup, SMTP identity creation and rotation, forwarding scenarios, tenant authorization, and notification listing.
- Add production workflow contract coverage for release, publish, deploy guards.
- Add browser coverage for Pinguin logo, landing header login, and frontend config contracts.

### Docs 📚
- Update README to replace references from "failed" to "errored" notification status.
- Clarify background worker processes queued and errored notifications.
- Revise environment variable descriptions to reflect updated error terminology.

## [v0.4.16] - 2026-05-15

### Features ✨
- Users can now add and verify sender domains through the SMTP relay page with DNS verification (TXT, SPF, DMARC) gating domain ownership.
- All sender domains are managed via API and database instead of static YAML configuration.

### Improvements ⚙️
- Removed legacy `smtpSubmission.senderDomains` YAML configuration; sender domains are fully API/DB owned.
- SMTP sender domain validation errors are surfaced in the SMTP relay UI for better visibility.
- Cleaned up legacy sender domain data automatically during startup.
- Documentation updated to reflect the new sender domain ownership and verification flow.
- Production SMTP relay setup simplified by shifting TLS termination to gateway and removing direct TLS config from Pinguin.

### Bug Fixes 🐛
- Fixed handling of dynamic SMTP sender domains to prevent incorrect legacy configurations.
- Rejected legacy sender domain YAML entries to avoid startup failures and inconsistent states.

### Testing 🧪
- Updated server tests to cover legacy sender domain cleanup instead of sender domain replacement.
- Enhanced test coverage for SMTP identity repository initialization and sender domain cleanup error paths.

### Docs 📚
- Updated README and SMTP delivery plan documentation to remove deprecated YAML sender domain config and describe new API-backed sender domain verification process.
- Adjusted architecture documentation to clarify sender domain management changes.

## [v0.4.15] - 2026-05-15

### Features ✨
- Add initial deployment configuration file `deploy/app.yml` to manage GitHub Pages deployment within the Pinguin app.

### Improvements ⚙️
- Migrate stored SMTP identity credential passwords during server startup to improve security handling.
- Refactor deployment process: move GitHub Pages deployment resource into `deploy/app.yml` so Pinguin owns the deployable frontend contract while gateway Ansible owns execution.
- Enhance deployment documentation detailing environment variables and verification steps for production deploys.

### Bug Fixes 🐛
- Remove legacy unresolved password error response from SMTP identity handling for more consistent error mappings.
- Fix SMTP sender domain replacement logic and tenant bootstrap integration with credential migration.
- Prevent server startup failure by deleting legacy NULL-owner SMTP sender-domain rows before seeding.

### Testing 🧪
- Add tests covering SMTP credential migration during server run.
- Adjust test suite to expect SMTP credential migration alongside other bootstrapping and TLS setup steps.

### Docs 📚
- Update README deployment section to describe new app manifest usage and gateway forwarding configuration.
- Synchronize documentation to reflect the centralized deployment control and updated release preparation steps.

## [v0.4.14] - 2026-05-15

### Features ✨
- GitHub Pages publishing is now managed under gateway Ansible ownership via a new `pages-deploy` target.
- Introduced a `pages-deploy` Makefile target and deployment script to integrate Pages publishing into Ansible workflow.

### Improvements ⚙️
- Updated deployment and release scripts to reflect the new deployment flow and targets.
- Deployment now verifies operator is on clean `master` branch matching `origin/master` with zero open PRs before proceeding.
- Gateway Ansible deploy includes both backend and Pages publishing, replacing legacy post-Ansible publishing steps.
- README and contract tests aligned with new deployment and GitHub Pages ownership changes.

### Bug Fixes 🐛
- Fixed production guard contract tests that failed due to outdated environment defaults and deploy targets.

### Testing 🧪
- Contract and production workflow tests updated to cover new deployment targets and workflows.

### Docs 📚
- Updated README describing new deployment process and integration with gateway Ansible for Pages publishing.
- Enhanced documentation and contract test notes for deployment guard requirements and workflow.

## [v0.4.13] - 2026-05-14

### Features ✨
- Added a shared production Git guard to ensure release, publish, and deploy commands run only from a clean local `master` branch matching `origin/master` with zero open PRs.
- Introduced Playwright tests for SMTP relay identity modal interaction, improving UI security controls.

### Improvements ⚙️
- Updated production workflows to verify Git state and zero open PRs before running release, publish, and deploy tasks.
- Enhanced deployment script to enforce production Git guard and verify gateway SMTP listener ports.
- Refined publishing process to include Git state verification and improved documentation.
- Added comprehensive contract testing coverage for production Git guard usage.
- Improved CSS styling for base styles.
- Updated SMTP relay UI to move password and rotation controls inside modals for better security and UX.
- Backend alignment with gateway high-port publishing for MX and SMTPS on ports 8025 and 8465.
- Replaced placeholder logo with canonical Pinguin turquoise envelope mark.

### Bug Fixes 🐛
- Fixed production deploy completing without validating clean Git state.
- Resolved startup crashes caused by legacy NULL-owner sender-domain rows by deleting them before seeding the allowlist.
- Allowed null reverse-path SMTP forwarding for DSNs and loop-safe messages.
- Corrected issue exposing SMTP relay passwords and rotate actions by confining them to modals only.

### Testing 🧪
- Added contract coverage for the production Git guard in release, publish, and deploy scripts.
- Expanded backend and frontend test coverage for sender-domain handling and SMTP relay credential management.
- Included production workflow contract coverage verifying release and publish processes.
- Improved end-to-end dashboard and UI test stability with updated artifacts.

### Docs 📚
- Documented new production Git-state requirements for release, publish, and deploy commands.
- Updated deployment and publishing procedures with stricter validation details.
- Clarified SMTP relay identity and DNS setup documentation.
- Strengthened guidance on backwards compatibility limitations and shared-address forwarding verification.

## [v0.4.12] - 2026-05-14

### Features ✨
- _No changes._

### Improvements ⚙️
- Documented that Pinguin does not support backward compatibility, legacy schemas, legacy data, or fallback code paths.
- Added a Compatibility Policy section explaining the removal of legacy data and single runtime path enforcement.
- Updated SMTP relay documentation to describe reusable credential viewing instead of one-time display.

### Bug Fixes 🐛
- Fixed startup crash caused by legacy sender-domain rows with NULL owner_email by deleting these legacy rows before re-seeding the configured allowlist.

### Testing 🧪
- Added sender-domain storage tests covering deletion of legacy NULL-owner configured rows without compatibility migration.
- Expanded black-box SMTP forwarding and deployment contract coverage.

### Docs 📚
- Added Compatibility Policy documentation emphasizing no backward compatibility and legacy data removal.
- Updated README with Compatibility Policy details.

## [v0.4.11] - 2026-05-14

### Features ✨
- Add sender domain setup workspace for SMTP relay with domain verification and credential APIs.
- Enable authenticated users to add, verify, and manage sender domains and SMTP identities with exact DNS record checks.
- Allow admins to view and rotate SMTP relay credentials inside the Gmail SMTP settings modal with passwords stored encrypted at rest.

### Improvements ⚙️
- Bypass tenant lookup for SMTP domains to simplify domain access.
- Remove validation requirement for static sender domains when using SMTP forwarding.
- Align UI and API workflows with verified domain ownership and reusable SMTP credentials.
- Enhance HTTP API with new endpoints for listing, creating, checking DNS, and retrieving credentials of sender domains.

### Bug Fixes 🐛
- Fix access control to enforce tenant authorization on notification endpoints.
- Reject one-time password leaks in SMTP identity list responses.
- Correct validation logic for SMTP forwarding configuration.

### Testing 🧪
- Add backend and browser coverage for sender-domain DNS setup, manual DNS checks, and verified-domain SMTP identity creation.
- Add coverage for retrieving and rotating existing SMTP relay credentials from the modal.
- Extend black-box testing for SMTP forwarding, notification search, and the dashboard event log.

### Docs 📚
- Update SMTP relay documentation to describe reusable credential viewing and rotation instead of one-time password display.
- Clarify shared-address forwarding DNS setup and verification process.
- Revise README and architecture notes regarding sender-domain allowlist and SMTP relay workflows.

## [v0.4.10] - 2026-05-14

### Features ✨
- Introduce rewriting of forwarded SMTP messages to preserve original sender identity and remove stale headers.
- Rewrite From, Reply-To, and add X-Pinguin-Original-From headers during message forwarding.
- Maintain header order, including trace and resent headers, while stripping authentication-related headers.

### Improvements ⚙️
- Refactor smtpforwarding relay and types tests for better coverage and clarity.
- Add comprehensive message header parsing with support for folded headers and validation.
- Improve logging on forwarding rewrite skips and relay failures.

### Bug Fixes 🐛
- Fix smtpforwarding logic to correctly rewrite messages and update tests accordingly.
- Correct handling of messages lacking Reply-To to fallback correctly to original From.
- Ensure temporary forward errors are wrapped and propagated properly.

### Testing 🧪
- Add tests verifying header rewrite correctness including From, Reply-To, X-Pinguin-Original-From fields.
- Test preservation of original message body and header order after rewriting.
- Include negative tests for malformed headers, body read errors, and unparseable messages.
- Test forwarding keeps original Reply-To if present.
- Test error scenarios for message parsing and body read failures.

### Docs 📚
- _No changes._

## [v0.4.9] - 2026-05-12

### Features ✨
- _No changes._

### Improvements ⚙️
- Publish `pinguin-doctor` in the production Docker image and make the server the default command to support gateway Compose preflight overrides.
- Add a Dockerfile contract test to validate the presence of the doctor preflight command in production builds.

### Bug Fixes 🐛
- Fix gateway deploy preflight to run `pinguin-doctor` instead of the normal server by updating the Docker image and command configuration.
- Align doctor TAuth parsing and validation with runtime schema and add regression coverage.
- Correct SMTP forwarding for null reverse-path traffic and enforce sender-domain allowlist for forwarding-only deployments.

### Testing 🧪
- Add contract test ensuring that the production Dockerfile publishes both `pinguin` and `pinguin-doctor` binaries correctly.
- Validate gateway Pinguin config with environment expansion in regression tests.

### Docs 📚
- Document DSN forwarding behavior and forwarding identity dependencies.

## [v0.4.8] - 2026-05-12

### Features ✨
- Added legacy `dashboard.html` redirect to new `event-log.html`.
- Introduced runtime configuration and theme management for the Pinguin UI.
- Split the authenticated browser workspace into dedicated `event-log.html` and `smtp-relay.html` pages.

### Improvements ⚙️
- Replaced the old landing page with a focused Pinguin sign-in screen and notification queue preview.
- Added a horizontal dashboard menu with `mpr-ui` header links for Event log and SMTP relay.
- Aligned SMTP host-port publishing with the edge gateway high-port setup (MX 8025, SMTPS 8465).
- Added `make up` and `make down` commands for Docker Compose orchestration.
- Updated configuration and documentation for the split Event log and SMTP relay pages and shared-shell auth.

### Bug Fixes 🐛
- Fixed doctor module logic and updated related tests.

### Testing 🧪
- Added black-box SMTP forwarding coverage including accept/forward, unknown-recipient rejection, and relay failure cases.
- Added backend and browser test coverage for notification search, cursor pagination, infinite scroll, and GORM query contracts.
- Added browser tests for dashboard horizontal menu navigation between Event log and SMTP relay.
- Added backend and browser tests for separate Event log and SMTP relay page contracts.
- Enhanced backend tests for tenant admin authorization and email-domain scope restrictions.
- Added coverage for deployment contract regarding gateway SMTP host-port configuration.
- Restored Go statement coverage to 100% and introduced `make ci` coverage gate.

### Docs 📚
- Documented shared-address forwarding DNS setup and verification using `mx.pinguin.mprlab.com`.
- Updated README and architecture docs to describe `config-ui.yaml` as the browser auth source of truth without embedded provider metadata.
- Updated docs to describe split Event log and SMTP relay page destinations.
- Clarified dashboard tenant authorization roles and domain scoping.
- Documented gateway edge mappings for SMTP ports (`25 -> tutosh:8025`, `465 -> tutosh:8465`).

## [v0.4.7] - 2026-05-08

### Features ✨
- Add UI and API-driven inbound SMTP forwarding for shared SMTP identities with forwarding owners and immediate fanout via configured relay.
- Implement core inbound SMTP forwarding service accepting unauthenticated MX delivery and forwarding mail without mailbox storage.
- Overhaul dashboard with horizontal menu and new SMTP relay management surface.
- Add forwarding management endpoints and support forwarding owners to SMTP identities.

### Improvements ⚙️
- Rename SMTP identity dashboard surface to SMTP relay, maintaining exact sender identity management.
- Accept `MAIL FROM:<>` null reverse-path traffic in SMTP forwarding for DSNs and auto-generated loop-safe messages.
- Require `smtpSubmission.senderDomains` when inbound SMTP forwarding is enabled to ensure proper allowlist configuration.

### Bug Fixes 🐛
- Fix null reverse path forwarding handling in SMTP forwarding.
- Fix forwarding sender domain validation to enforce correct domain restrictions.

### Testing 🧪
- Add black-box SMTP forwarding coverage for acceptance, forwarding, unknown-recipient rejection, size limits, relay failures, and startup wiring.
- Improve and update smtpidentity handler and repository tests.
- Add browser coverage for dashboard horizontal Event log and SMTP relay menu.

### Docs 📚
- Document shared-address forwarding DNS setup and verification using `mx-forward.pinguin.mprlab.com`.
- Update README and architecture notes to describe inbound SMTP forwarding listener and new dashboard SMTP relay surface.
- Update dashboard docs for authenticated event log and SMTP relay management.

## [v0.4.6] - 2026-05-06

### Features ✨
- _No changes._

### Improvements ⚙️
- Redesigned the one-time Gmail SMTP settings modal with a top-right close control and inline clipboard copy icons inside non-editable fields.
- Added SMTP submission throttling and direct relay options for Gmail Send-As domains without upstream SMTP accounts.
- Updated frontend tests to verify copy controls and readonly assertions for SMTP credentials.

### Bug Fixes 🐛
- _No changes._

### Testing 🧪
- Extended end-to-end dashboard tests to cover modal closing, copy button functionality, and readonly fields in SMTP settings.

### Docs 📚
- Updated issues documentation to reflect the changes in Gmail SMTP settings features and controls.

## [v0.4.5] - 2026-05-06

### Features ✨
- Trigger GitHub Pages build during deploy and verify live artifact source commit to ensure deployment consistency.
- Add a `pinguin-pages-build.json` marker file to the Pages artifact including source commit metadata.

### Improvements ⚙️
- Enhance deploy script to publish static assets, trigger Pages build, and poll for source commit verification.
- Update README with detailed publish and deploy instructions clarifying parameterless production deployment and step sequencing.
- Refine legacy GitHub Pages publishing to handle backend verification, artifact staging, and source commit tracking.
- Improve testing by adding contract tests for deployment sequence and Pages artifact content.

### Bug Fixes 🐛
- Fix Pages deploy source verification so deployment reports success only when the live Pages artifact matches the expected commit.
- Resolve issue where `make deploy` could report success while GitHub Pages still served stale content, causing UI cache duplication.

### Testing 🧪
- Add tests to verify deployment script includes required steps for backend and Pages deployment and source commit verification.
- Add tests to confirm `pinguin-pages-build.json` is generated correctly with commit metadata during artifact build.

### Docs 📚
- Update README.md and ISSUES.md with deployment workflow and issue PG-351 resolution details.
- Clarify documentation for using `make deploy` after `make publish` to properly deploy backend and Pages with verification.

## [v0.4.4] - 2026-05-05

### Features ✨
- Add SMTP submission throttle mechanism limiting sessions, auth attempts, and messages.
- Support new SMTP submission delivery mode `direct` for sending to recipient MX servers.
- Add SMTP credential copy controls to enhance security.

### Improvements ⚙️
- Count message quota only after relay acceptance to improve accuracy.
- Apply SMTP timeouts to reads only for better handling.
- Update direct relay logic and tests to support direct MX delivery mode.
- Enhance SMTP throttling tests for robustness.
- Expose public SMTP port and security mode for Gmail-facing settings.
- Accept insecure auth when running behind TLS-terminating proxy (Caddy).

### Bug Fixes 🐛
- Fix duplicate auth probes causing unnecessary failures.
- Correct issues in direct relay implementation and add related tests.

### Testing 🧪
- Strengthen and expand tests for SMTP submission throttling and direct relay.
- Add CI coverage for SMTP public settings and server startup scenarios.

### Docs 📚
- Update README and docs with instructions for `direct` delivery mode and SMTP throttling.
- Clarify configuration options for SMTP submission public port, security mode, and delivery mode.
- Document architecture changes supporting direct MX delivery and precise throttling controls.

## [v0.4.3] - 2026-05-04

### Features ✨
- Added new environment boundary tests centralizing environment variable access.
- Added environment boundary tests for configuration.

### Improvements ⚙️
- Updated configuration files and documentation to use `web.enabled: false` in `config.yml` instead of `DISABLE_WEB_INTERFACE` for disabling the web interface.
- Refined CLI settings: replaced environment variable config with explicit flags for server address, auth token, tenant ID, timeouts, and log level.
- Simplified CLI config loading to ignore environment variable fallbacks.
- Enhanced README with clearer configuration, CLI usage, and logging instructions.
- Removed deprecated environment fallback bindings in client config.
- Improved test coverage and refactored tests to match new CLI flag usage.
- Streamlined server config loading API and removed deprecated flags.

### Bug Fixes 🐛
- Fixed incorrect usage of environment variables in CLI config loading that could cause unexpected behavior.

### Testing 🧪
- Added comprehensive tests for environment variable boundaries in config loading.
- Added examples and tests verifying environment and config interactions.
- Expanded integration and E2E tests with updated CLI usage patterns.

### Docs 📚
- Revised all documentation references to web interface configuration and CLI usage.
- Clarified environment variable usage and config file structure in README.
- Updated instructions for loading environment variables prior to running the server.

## [v0.4.2] - 2026-05-04

### Features ✨
- Add deploy script to enable backend deployment through mprlab-gateway with verification and GitHub Pages publishing.
- Improve release and publish workflows with enhanced scripts and Makefile targets.

### Improvements ⚙️
- Restore deploy script's gateway handoff to generic `deploy TARGET=pinguin` dispatcher before legacy GitHub Pages publication.
- Update Makefile with new deploy target and improved publish target supporting branch and remote customization.
- Enhance publish script by decoupling image publishing from GitHub Pages publishing, supporting dry-run and no-latest modes.
- Update documentation including CHANGELOG and ISSUES with latest resolved issues and deployment improvements.

### Bug Fixes 🐛
- Fix release `make ci` failure by routing backend deployment through the gateway dispatch model to preserve backend-before-pages sequencing.

### Testing 🧪
- _No changes._

### Docs 📚
- Update CHANGELOG and ISSUES documents with details on deployment and release process improvements.

## [v0.4.1] - 2026-05-04

### Features ✨
- Support tenant admin emails configured in `tenants[].admins` to grant dashboard admin access.
- Remove tenant bootstrap records when tenants are removed from configuration to clean up stale data.

### Improvements ⚙️
- Update authorization to consider both TAuth `admin` role and configured tenant admin emails.
- Enhance SMTP identity management to allow access for configured tenant admins.
- Add backend and browser test coverage for tenant admin authorization and bootstrap record pruning.
- Update documentation to reflect tenant admin email configuration and behavior.
- Inject tenant repository into SMTP identity handler for admin checks.
- Migrate database schema to include TenantAdmin entity.

### Bug Fixes 🐛
- Fix admin authorization logic for tenant and SMTP identity access to include configured tenant admin emails.

### Testing 🧪
- Add comprehensive tests for tenant admin email authorization in notifications and SMTP identity routes.
- Add tests for error conditions during admin email lookups.
- Enhance integration tests for multi-tenancy with configured tenant admins.

### Docs 📚
- Update README and ARCHITECTURE.md to document tenant admin email configuration and authorization behavior.
- Add examples for tenant admin email entries in configuration files.

## [v0.4.0] - 2026-05-03

### Features ✨
- Decouple authenticated SMTP submission from notification tenants with an independent sender-domain allowlist, identity credentials, and upstream relay profile.
- Add tenant switcher for admins to view and manage notifications across tenants from the dashboard.
- Implement backend-supported search and infinite scroll for notifications with cursor pagination and a global refresh control.

### Improvements ⚙️
- Enforce tenant authorization on notification list, reschedule, and cancel endpoints based on session roles.
- Keep Pinguin branding consistent across tenant UIs and serve the official favicon.
- Publish multi-platform Docker images for amd64 and arm64 architectures.
- Update Makefile to require 100% Go statement coverage and gate it in CI.
- Align local Docker browser origins and services for improved OAuth integration and development experience.

### Bug Fixes 🐛
- Fix SMTP identity admin authorization to require the admin role.
- Match legacy errored alias only on exact search to avoid query broadening.
- Fix tenant query authorization enforcing session role checks.
- Correct Docker build context variable usage during publishing.

### Testing 🧪
- Add comprehensive backend and frontend tests covering notification search, pagination, tenant authorization, and SMTP identity access control.
- Restore and enforce 100% Go statement coverage across all packages.
- Add CI coverage gate to fail if coverage falls below 100%.

### Docs 📚
- Update architecture and README docs to clarify SMTP submission independence from tenants and dashboard tenant role authorization.
- Document the use of `smtpSubmission.relay` and global sender domain allowlist.
- Clarify TAuth role semantics for tenant and SMTP identity authorization in UI and API.

## [v0.3.1] - 2026-05-03

### Features ✨
- _No changes._

### Improvements ⚙️
- Replace the old landing page with a focused Pinguin sign-in screen and notification queue preview.
- Add `make up` and `make down` wrappers for local Docker Compose orchestration.
- Add split `configs/.env.pinguin.example` and `configs/.env.tauth.example` files to reflect current Compose topology.
- Update README and architecture to use `config-ui.yaml` as the browser auth source of truth.

### Bug Fixes 🐛
- Restore production login by migrating frontend auth to `mpr-ui` and removing direct `tauth.js` loading.
- Align local Docker origins: UI on `http://localhost:8080`, API on `http://localhost:8081`, and TAuth on `http://localhost:8082`.
- Remove duplicate landing-page auth controller to avoid multiple Google Identity initializations.
- Remove Pinguin's duplicate account chip in the authenticated header to use the shared `mpr-ui` user menu.

### Testing 🧪
- Add browser tests covering landing header login path and `mpr-ui@latest` config contract.
- Update profile-menu tests to assert shared `mpr-user` header contract instead of removed local settings menu.

### Docs 📚
- Document updated architecture and README for new auth flow using `config-ui.yaml` and updated Docker Compose setup.

## [v0.3.0] - 2026-05-02

### Features ✨
- Add authenticated SMTP submission for Gmail-compatible Send-As identities.
- Dashboard users can create, rotate, and delete exact SMTP sender credentials with secure one-time passwords.
- Expose SMTP submission listeners that validate and relay SMTP AUTH submissions through tenant's upstream SMTP provider.

### Improvements ⚙️
- Refactor SMTP submission listener handling and improve error management.
- Normalize sender domain and improve error handling in SMTP identity management.
- Differentiate between not found and storage errors in SMTP identity lookups.
- Avoid restoring deleted identities during concurrent authentication updates.
- Disable GitHub Actions workflows and enable local CI and publish scripts.
- Update configuration and documentation to support SMTP submission and sender domains.
- Simplify Makefile and introduce local publish and pages-build targets.

### Bug Fixes 🐛
- Fix sender domain error handling and concurrent identity restoration issues in SMTP identity module.

### Testing 🧪
- Add extensive tests for SMTP identity repository, SMTP submission server, and integration tests for multitenancy.
- Enhance HTTP API tests for SMTP identity management and related components.
- Remove CI workflows to enforce local CI run for validation.

### Docs 📚
- Add detailed SMTP submission and Send-As identities usage and configuration documentation.
- Update architecture and README with SMTP submission features and tenant sender domain configuration.
- Document new build and publish process replacing GitHub Actions workflows.
- Improve AGENTS.md and other docs for local CI usage and SMTP identity workflow guidance.

## [v0.2.0] - 2026-04-01

### Features ✨
- Add `make publish` target to build and publish multi-arch Docker images with platforms linux/amd64 and linux/arm64.

### Improvements ⚙️
- Update Docker workflow to pull latest base images and support multi-arch build and push.
- Include Makefile in CI tests to ensure build targets are verified.
- Enhance README with instructions for publishing multi-arch Docker images to GHCR.

### Bug Fixes 🐛
- _No changes._

### Testing 🧪
- Add Makefile to CI test paths for improved test coverage.

### Docs 📚
- Add documentation for `make publish` Docker image publishing process and configuration options.

## [v0.1.1] - 2026-03-29

### Features ✨
- Trigger Docker build workflow on push tags matching `v*` for improved release automation.

### Improvements ⚙️
- Enhanced CI workflow condition to include push events for Docker build triggering.
- Docker build metadata step now emits image tags for Git release tags.
- Added push trigger with tag filters to Docker build workflow specification.

### Bug Fixes 🐛
- _No changes._

### Testing 🧪
- Expanded tests to cover push tag triggers in Docker build workflow.
- Added assertions for Docker metadata step producing correct tag information.
- Improved test helpers to verify workflow steps and conditions.

### Docs 📚
- _No changes._

## [v0.1.0] - 2026-03-29

### Features ✨
- Add `pinguin-doctor` command for configuration validation
- Introduce multi-platform Docker build support in CI
- Implement tenant and authentication domain configurations with strict validation
- Add tenant runtime interceptor and tenant-scoped delivery notifications
- Introduce dynamic TAuth client loader for web
- Support tenant repository and HTTP wiring for multitenancy
- Add backend and HTTP integration tests for multitenancy

### Improvements ⚙️
- Improve theme persistence and profile menu styling
- Refactor GORM queries and use portable domain reset queries
- Upgrade Go package dependencies and document dependencies
- Optimize retry worker pending job queries and tenant repository caching
- Enhance autonomous coding flow and CI workflow path triggers
- Add Docker Compose profiles and gate Docker build pushes for PRs
- Add volume mounts and configuration bridges for web UI

### Bug Fixes 🐛
- Fix avatar menu layout and dashboard profile menu avatar styling
- Resolve redirect loop by validating auth state cache profile
- Fix missed auth events on bootstrap and tenant domain upsert conflicts
- Stabilize Playwright runs and fix Google Identity stub timeouts in tests
- Restore header login button and fix mpr login button trigger

### Testing 🧪
- Add e2e check for avatar menu after login
- Cover CLI with integration tests and extend login automation coverage
- Add test coverage for server helpers and attachments
- Improve integration tests with healthz endpoint bypass and tenant lookup validation
- Stabilize multitenancy HTTP and UI flow tests with optimized timeouts

### Docs 📚
- Document tenant configuration and multitenancy plans
- Clarify API-only serving by Gin and update issues tracking
- Add .env example and sample snippet with environment parametrics
- Document real end-to-end UI tests and Playwright dependency installation steps

### Detailed Changes
- Cached early mpr-ui auth events in `tauth-helper` and seed the session bridge from the cached state to avoid missed auth transitions when the UI loads before the app bootstrap (PG-332 follow-up).
- Fixed auth bootstrap loops by waiting on tauth.js/mpr-ui readiness, removing fallback redirects, and broadening the TAuth CORS allowlist defaults for UI + GIS origins (PG-332).
- Added a declarative `MPRUI.init` configuration bridge so runtime TAuth settings are applied as mpr-ui DSL attributes (PG-331).
- Enforced the `/api/me` session endpoint in the tauth.js bootstrap and validated required helper globals before mpr-ui loads (PG-330).
- Migrated TAuth config to `server.tauth`, removed Pinguin-side access allowlists and tenant admins, defaulted the UI to global view, and aligned the UI/auth tests with `tauth.js` + mpr-ui DSL (PG-329).
- Simplified the TAuth client integration to load `tauth.js` before `mpr-ui` and rely on declarative auth events for UI state (PG-328).
- Aligned mprlab-gateway production orchestration with `pinguin-api.mprlab.com` routing, GitHub Pages UI hosting, and updated CORS/env templates (PG-325).
- Added GitHub Pages deployment for the `/web` bundle with `pinguin.mprlab.com` CNAME and production API/runtime defaults for `pinguin-api.mprlab.com` (PG-324).
- Swapped SQLite to a pure-Go GORM driver so CGO-disabled builds can open SQLite databases (PG-317).
- Reworked tenant and notification GORM queries to use struct/clause builders so SQL is fully generated by GORM (PG-316).
- Made tenant bootstrap domains authoritative by resetting domain mappings and validating duplicates, plus stabilized the Playwright dev server logging for CI (PG-315).
- Switched docker-compose to mount `/web` via the named `pinguin-web` volume bound to `./web` (PS-402).
- Declared `/web` as a Docker volume for the UI bundle and documented the mount expectations (PS-401).
- Moved notification request validation into model constructors and edge handlers, leaving service logic edge-validated (PG-204).
- Optimized retry worker to query pending notifications across active tenants in a single join (PG-203).
- Added a gRPC tenant-resolution interceptor and removed per-handler tenant lookups (PG-202).
- Added the `--disable-web-interface` flag (and matching `DISABLE_WEB_INTERFACE` env var) so operators can run gRPC-only deployments without configuring TAuth/Google web settings (PG-103).
- Documented the multitenancy technical plan (`docs/multitenancy-plan.md`) covering schema, config, auth, and rollout steps for serving multiple domains from one deployment (PG-104).
- Added a regression test that asserts the `third_party` directory stays absent so we continue relying solely on upstream modules for TAuth and google protos (PG-405).
- Relocated `pinguin.proto` to `pkg/proto/pinguin.proto` so consumer-facing definitions live under the exported packages and documented the new path (PG-406).
- Moved the Cobra CLI into `cmd/client`, removed the extra module/go.work entry, and updated build/docs/tests to reference the unified binary (PG-407).
- Removed the Go workspace files (`go.work`/`go.work.sum`) now that the repository relies solely on the root module (PG-408).
- Relocated integration tests to `tests/integration`, renamed the package to `integrationtest`, and updated build tooling accordingly (PG-404).
- Removed `tests/clientcli`; `cmd/client` is the single CLI for manual usage and automated test harnesses (PG-321).
- Removed the `third_party` directory and rely entirely on module-managed dependencies, simplifying Go workspace setup and proto regeneration (PG-402).
- Gated the docker-build GitHub Actions workflow so it only runs after the Go Tests workflow completes successfully, while preserving manual dispatch for emergencies (PG-401).
- Added `dev` and `docker` docker-compose profiles plus a regression test and README guidance so operators can choose between local builds and GHCR-hosted images (PG-400).
- Documented that the Gin HTTP stack no longer serves `/web` and relies on ghttp for the static bundle, keeping the Go server focused on `/api`/`/runtime-config` (BF-306).
- Disabled Playwright’s per-test parallelism so the shared mock dev server state is not mutated concurrently, stabilizing dashboard smoke tests (BF-305).
- Hardened HTTP CORS defaults by disabling credentialed responses whenever `HTTP_ALLOWED_ORIGINS` is empty, preventing cross-site requests from reusing TAuth cookies (BF-304).
- Added a “Docker quickstart” section to README so operators can boot the full orchestration (Pinguin + TAuth + ghttp) with timed commands on the documented ports (IM-200).
- Corrected docker-compose so the ghttp static host binds to `http://localhost:4173` (matching docs/CORS defaults) and updated the TAuth sample `.env`/README instructions accordingly (BF-303).
- Stabilized the scheduled email integration test by waiting for the persisted `sent` status so CI no longer flakes on BF-302.
- Fixed the sample `HTTP_ALLOWED_ORIGINS` value and README docker-compose instructions so compose users open the UI on `http://localhost:4173` and the HTTP API accepts requests from that origin (BF-301).
- Added GoDoc coverage for all client-facing packages (client, attachments, grpcapi, grpcutil, logging) so integrators can rely on `go doc` to understand how to embed the SDK.
- Added a scheduling integration test backed by injectable senders to ensure emails queued with future timestamps are dispatched only after the background worker releases them.
- Extracted the scheduling/retry worker into `github.com/tyemirov/utils/scheduler`, wired the server through a repository/dispatcher bridge, and added unit tests so other binaries can reuse the persistence-agnostic scheduler.
- Removed the `generate-secret` CLI command and `pkg/secret` helper in favor of documenting `openssl rand -base64 32` for token generation.
- Split the CLI and client-test utilities into standalone Go modules so `go run ./...` targets only the server binary.
- Documented all required environment variables in README/.env so the server starts without configuration surprises.
- Segregated server-only config, db, model, and service code under `internal/` while keeping shared clients in `pkg/`.
- Added a Cobra/Viper-based CLI for submitting immediate or scheduled notifications to the Pinguin gRPC service.
- Disabled SMS delivery when Twilio credentials are absent and emit a startup warning to document the configuration gap.
- Renamed the project to Pinguin, including module path, build targets, and user-facing documentation.
- Added comprehensive unit and integration tests across configuration, persistence, and gRPC layers.
- Added GitHub Actions workflow to enforce gofmt, go vet, and go test on pushes and pull requests.
- Added multi-stage Dockerfile and automated GHCR build workflow for the Pinguin gRPC server.
- Added optional `scheduled_time` to the gRPC Notification API and persisted model to support delayed dispatch.
- Updated retry worker to respect scheduled timestamps before attempting delivery.
- Introduced regression tests ensuring scheduled notifications remain queued until due.
- Migrated email delivery configuration to provider-agnostic SMTP naming, eliminating legacy third-party terminology from code and docs.
- Documented the SMTP delivery pipeline and added a unit test verifying the service wires the SMTP sender with configured credentials.
