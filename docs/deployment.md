# Deployment

## Lifecycle ownership

GitHub Actions are disabled for Pingwin.
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
Production lifecycle operations use `make release`, `make publish`, and `make deploy`.

## Production name change

The tracked configuration declares these current production names:

| Role | Hostname |
| --- | --- |
| Browser | `pingwin.mprlab.com` |
| HTTP API and SMTPS | `pingwin-api.mprlab.com` |
| Inbound MX | `mx.pingwin.mprlab.com` |

Native `make deploy` activated `v1.1.8` on November 11, 2026.
The source repository and Go module remain `github.com/tyemirov/pinguin`.
The current runtime uses the Pingwin service names, browser keys, SMTP headers, and gRPC namespace.

The permanent deployment owner remains `pinguin`.
This identity selects the current Gateway application graph during the public service rename.
Native convergence replaces that owner's routes and listeners with the current Pingwin declarations.
The operator authorized removal of Pingwin deployment records on November 11, 2026.

For the complete name change:

1. Configure DNS for all three hostnames.
2. Configure the TAuth tenant `pingwin` and its browser origin.
3. Authorize the new origin in the Google OAuth client.
4. Update private origins, SMTP hostnames, cookies, and `TAUTH_TENANT_GOOGLE_WEB_CLIENT_ID_PINGWIN`.
5. Publish new `_pingwin-challenge` TXT records and relay SPF records for each sender domain.
6. Preserve deployment owner `pinguin` and retire `mprlab-pinguin-runtime/pinguin-api` before the replacement starts.
7. Preserve volume `mprlab-nginx-gateway_pinguin-data`, the database path, and the encryption key.
8. Stop the previous runtime before the new runtime opens the same SQLite database.
9. Complete release, publication, deployment, and public acceptance in that order.

The local Compose volume declaration keeps its existing `pinguin-data` identity.

## Current operational status

The November 11, 2026 deployment completed for `v1.1.8`.
Native state revision `774` contains six verified resources for owner `pinguin`, generation `2`.
The API and website health endpoints returned `200`.
The public `/.mprlab-release.json` marker reported `v1.1.8`.
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
Authenticated browser access and message delivery retain their separate acceptance procedures below.

## Managed tenant conversion

The first managed release requires the bounded [production data conversion](multitenancy-plan.md#production-data-conversion) before deployment.
The conversion assigns current production tenants to the TAuth account for `temirov@gmail.com`.
The operator must confirm that account's TAuth user ID.
The conversion requires a write-stopped maintenance window and the runbook's acceptance procedure.
Server startup never runs the conversion.

F001 remains active until the operator accepts the managed runtime and the bounded conversion command is removed.
The [managed tenant contract](multitenancy-plan.md) contains the complete conversion inputs, commands, and acceptance criteria.
The [shared UI migration](mpr-ui-migration.md#activation) retains separate publication, cache, and hosted Google acceptance requirements.

## Release, publish, and deploy

A clean application checkout must be synchronized with the remote default branch.
The Gateway runtime and required operator credentials must be available.

Use `MPRLAB_GATEWAY_EXECUTABLE` to select the installed Gateway command.
Each Makefile wrapper selects this repository's Git root through `--app-root`.
The wrappers set `CI=1` and close standard input.
Gateway reads operator files from `MPRLAB_GATEWAY_OPERATOR_ROOT`.
Its default operator root is `$HOME/.config/mprlab-gateway`.

Authenticate to the image registry when required:

```bash
docker login ghcr.io
```

Prepare the declared artifacts locally:

```bash
make release
```

Publish that sealed release:

```bash
make publish
```

After the conversion and other production prerequisites, activate the declared runtime:

```bash
make deploy
```

Release prepares artifacts.
Publication exposes the sealed artifacts.
Deployment converges the declared runtime resources.
Each operation has a separate result.

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

Complete the conversion acceptance procedure against the deployed managed runtime.
Verify hosted Google sign-in, session restoration, and sign-out against the published shared UI contract.
Verify browser publication through the public website and `/.mprlab-release.json`.
Verify the API health route and the protocol-native gRPC and SMTP routes.
Verify Send-As and forwarding with the configured domains and destination inboxes.
Record the active release, lifecycle receipts, completed actions, and remaining acceptance items.

Source changes and local test results do not establish production activation.
The [issue tracker](../.mprlab/ISSUES.md) records the remaining F001 and I004 work.
