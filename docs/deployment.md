# Deployment

## Lifecycle ownership

Run `make ci` for local validation.
The permanent production declaration is the versionless [resources manifest](../.mprlab/deploy/resources.yml).
It declares the binary, container image, retained data, runtime capabilities, listeners, health probe, and Pages site.
It declares private TAuth inputs.
The operator configures the TAuth tenant separately.
The tracked production config template is [`configs/config.production.yml`](../configs/config.production.yml).

The manifest selects the ignored `.mprlab/deploy/.env` through `mprlab_resources.environment.source`.
The source field is optional when the process supplies every declared input.
The manifest also declares the names of its private inputs.
A process value takes precedence over the same input from the selected file.
The Docker build context excludes the private file.

The installed Gateway runtime owns lifecycle validation, sealed releases, declared publications, and runtime convergence.
It also owns shared networks, listeners, Caddy reconciliation, and lifecycle receipts.
The static browser frontend uses GitHub Pages.
Pingwin has no separate Pages activation command.

`make up` and `make down` operate the local Compose stack.
The production entry point is `make deploy`.

## Production name change

The tracked configuration declares these current production names:

| Role | Hostname |
| --- | --- |
| Browser | `pingwin.mprlab.com` |
| HTTP API and SMTPS | `pingwin-api.mprlab.com` |
| Inbound MX | `mx.pingwin.mprlab.com` |

Native `make deploy` activated `v1.1.11` on November 12, 2026.
The current runtime uses the Pingwin service names, browser keys, SMTP headers, and gRPC namespace.

The permanent deployment owner remains `pinguin`.
This identity selects the current Gateway application graph during the public service rename.
Native convergence replaces that owner's routes and listeners with the current Pingwin declarations.
The operator authorized removal of Pingwin deployment records on November 11, 2026.

The November 12 activation is recorded below. Use this checklist when changing production hostnames:

1. Configure DNS for all three hostnames.
2. Keep the permanent TAuth tenant `pinguin`, its user IDs, signing keys, and cookie names. Set its browser origin to `https://pingwin.mprlab.com`.
3. Authorize the new origin in the Google OAuth client.
4. Update private origins, SMTP hostnames, and `TAUTH_TENANT_GOOGLE_WEB_CLIENT_ID_PINGWIN`.
5. Publish new `_pingwin-challenge` TXT records and relay SPF records for each sender domain.
6. Preserve deployment owner `pinguin` and retire `mprlab-pinguin-runtime/pinguin-api` before the replacement starts.
7. Preserve volume `mprlab-nginx-gateway_pinguin-data`, the database path, and the encryption key.
8. Stop the previous runtime before the new runtime opens the same SQLite database.
9. Run `make deploy`. Then complete public acceptance.

The local Compose volume declaration keeps its existing `pinguin-data` identity.

## Recorded operational status

The November 12, 2026 deployment completed for `v1.1.11`.
Native state revision `790` contains six verified resources for owner `pinguin`, generation `5`.
The API and website health endpoints returned `200`.
The public `/.mprlab-release.json` marker reported `v1.1.11`.
The private capability is `pingwin.grpc.private-lan`.
The current gRPC interface returned `Unauthenticated` when the request contained no credentials.

The existing production database has the current managed schema.
The volume name, database path, encryption key, and TAuth key match the previous runtime.
SMTP checks through the public hostnames returned `250` on ports `25` and `465`.
TLS certificate verification passed on port `465`.

The previous refusal came from omitted host-port declarations.
The router targets host ports `8025` and `8465`, but Caddy published host ports `25` and `465`.
B010 set the manifest's `host_port` fields to match the existing router rules.
Public SMTP and SMTPS checks now pass from the macOS CLI.
All ten router rules and both HTTPS host bindings remained unchanged.

TAuth activated configuration revision `2` for the current browser origin.
Hosted Google sign-in succeeded, and all six existing tenants remained available.
The message delivery results below complete the recorded I009 acceptance.

DNS for `pingwin.mprlab.com` uses four GitHub Pages A records and MX priority `10` to `mx.pingwin.mprlab.com`.
The apex `mprlab.com` mail records remain unchanged.

The dedicated Pingwin tenant owns verified sender domain `pingwin.mprlab.com` and SMTP identity `support@pingwin.mprlab.com`.
The identity forwards to `marcopoloresearchlab@gmail.com`.
Pingwin verified the challenge, SPF, and DMARC records.

The PoodleScanner tenant moved to MPR Lab account `299335570791` on September 30, 2026, with verified message delivery.
The previous shared forwarding and submission relays retained credentials from account `297841131854`.
SES now verifies `pingwin.mprlab.com` and DKIM in account `299335570791`.

The dedicated `pingwin-runtime-smtp` principal permits SMTP delivery from the six configured sender addresses.
Its policy permits only `ses:SendRawEmail` for the three verified domains and the exact sender addresses.
Future SMTP identities require an explicit policy update.

On November 18, 2026, SES delivery-event logging was enabled for all three SES domains with one-day CloudWatch Logs retention.
The [delivery logging guide](ses-delivery-logging.md) records the configuration, generic recipient and forwarding queries, retention semantics, and live acceptance results.

The validated replacement credential is in the canonical private inputs and the Pingwin tenant profile.
Native deployment verified both shared relay credentials against those inputs.
That credential replacement preserved the other six tenant profiles and the PoodleScanner SMTP principal.
The November 18 logging activation added the configuration-set ARN to both SMTP principals' policies while preserving their credentials and sender restrictions.

The public port `25` test reached the destination Gmail inbox through the configured forwarding route.
The authenticated public port `465` test also reached that inbox.
Both messages passed SPF, DKIM, and DMARC checks with DKIM domain `pingwin.mprlab.com`.

## Managed tenant recovery

The [production recovery record](multitenancy-plan.md#production-data-conversion) documents the authorized September 15 tenant restoration, owner assignments, and acceptance receipts.
The obsolete converter and its image executable have been removed from the source.
SMS acceptance was deferred by the user. Client keys were updated locally; client application redeployment was not recorded.
These operational items remain separate from F001 source completion.
The original converter execution and historical notification preservation remain unverified.
The [shared UI activation record](mpr-ui-migration.md#activation) retains separate cache and hosted session checks.

## Production deployment

A clean application checkout must be synchronized with the remote default branch.
The Gateway runtime and required operator credentials must be available.

Use `MPRLAB_GATEWAY_EXECUTABLE` to select the installed Gateway command.
The Makefile calls the installed `mprlab-gateway deploy` command.
It selects this repository's Git root through `--app-root`.
It sets `CI=1` and closes standard input.
Gateway reads operator files from `MPRLAB_GATEWAY_OPERATOR_ROOT`.
Its default operator root is `$HOME/.config/mprlab-gateway`.

After the applicable production prerequisites, run the complete pipeline:

```bash
make deploy
```

Gateway runs the required CI, artifact preparation, publication, deployment, and health checks.
It owns phase selection and resumes incomplete work through the same command.
Each phase has a separate result.
The manifest retains `app-release`, `app-publish`, and `app-deploy` as internal policy identifiers.
These identifiers are separate from the public CLI command.

The manifest declares public SMTPS at `pingwin-api.mprlab.com:465` and MX traffic at `mx.pingwin.mprlab.com:25`.
The manifest's `host_port` fields declare the production host bindings:

| Listener | Public port | Host port | Caddy port |
| --- | --- | --- | --- |
| SMTP forwarding | `25` | `8025` | `25` |
| SMTPS | `465` | `8465` | `465` |

The public gateway forwards each public port to its declared host port.
Caddy terminates SMTPS TLS and forwards plaintext SMTP to private port `587`.
The shared HTTP request limit does not control this SMTP listener.
The [SMTP setup guide](smtp-setup.md) preserves listener limits and DNS procedures.

## Public acceptance

PG-386 requested a public TLS SMTP dialogue from a disposable deployment host.
The recorded B010 checks ran from the macOS CLI. I009 records destination delivery.
The disposable-host qualification has no recorded result.
Run and record that dialogue on a disposable software host to complete the historical environment-specific qualification.
Keep this operational check separate from implementation issue acceptance.

Verify the managed tenant inventory and owner access against the deployed runtime.
Verify hosted Google sign-in, session restoration, and sign-out against the published shared UI contract.
Verify browser publication through the public website and `/.mprlab-release.json`.
Verify the API health route and the protocol-native gRPC and SMTP routes.
Verify Send-As and forwarding with the configured domains and destination inboxes.
Record the active release, lifecycle receipts, completed actions, and remaining acceptance items.

Source changes and local test results do not establish production activation.
The [issue archive](../.mprlab/ISSUES.archive.md) records F001 and I004 source acceptance.
