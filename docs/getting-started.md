# Getting started

## Requirements

- Docker with the Compose plugin for the local stack.
- Go 1.25.4 or later for native commands.
- A Google OAuth web client for browser sign-in.
- An external SMTP account for tenant creation and notification email.
- Twilio credentials when SMS is required.

SQLite is embedded through the Go driver.
A separate database service is not required.
SMTP submission additionally requires a verified sender domain and public TLS for an external client.
The [SMTP setup guide](smtp-setup.md) covers DNS and listener configuration.

## Prepare the local stack

1. Clone the repository and enter its root:

   ```bash
   git clone https://github.com/tyemirov/pinguin.git pingwin
   cd pingwin
   ```

2. Create `configs/.env.pingwin` and `configs/.env.tauth` with the complete [local private inputs](configuration.md#local-private-inputs).
   Generate real keys and replace the Google client ID placeholder.
   The tracked example values are documentation only.
3. In `web/config-ui.yaml`, set the **Local Docker** Google `clientId` to your OAuth web client ID.
   Use that same client ID in `TAUTH_TENANT_GOOGLE_WEB_CLIENT_ID_PINGWIN`.
4. Authorize `http://localhost:8080` as a JavaScript origin in that Google OAuth client.
5. Keep the browser `tenantId` and `TAUTH_TENANT_ID_PINGWIN` equal to `pingwin`.
6. Match `TAUTH_SIGNING_KEY` with `TAUTH_TENANT_JWT_SIGNING_KEY_PINGWIN`.
7. Match `TAUTH_COOKIE_NAME` with `TAUTH_TENANT_SESSION_COOKIE_NAME_PINGWIN`.
8. Keep the local origin in Pingwin `HTTP_ALLOWED_ORIGIN1` and both TAuth origin inputs.
   The local TAuth URL is `http://localhost:8082`.
   Compose mounts `configs/config.tauth.yml` at `/config/config.yml`.
9. Create the local override for host gRPC access:

   ```bash
   cat > /tmp/pingwin-local.yml <<'YAML'
   services:
     pingwin-dev:
       ports:
         - "127.0.0.1:50051:50051"
   YAML
   ```

10. Start the stack:

    ```bash
    make up DOCKER_COMPOSE='docker compose -f docker-compose.yaml -f /tmp/pingwin-local.yml'
    ```

The `dev` profile builds Pingwin from the local Dockerfile.
TAuth and ghttp use their published images.
The browser loads shared `mpr-ui` assets from their configured network URLs.

| Service | Local address |
| --- | --- |
| Browser workspace | `http://localhost:8080` |
| HTTP API | `http://localhost:8081` |
| TAuth | `http://localhost:8082` |
| gRPC with the override | `127.0.0.1:50051` |
| SMTP forwarding, when enabled | Host `8025` to container `25` |
| SMTP submission, when enabled | Host `1587` to container `587`, host `8465` to container `465` |

The base Compose file does not publish gRPC to the host.
An SMTP port mapping does not enable its listener.
The local private inputs disable both SMTP listeners until the SMTP setup procedure is completed.

Compose retains SQLite in the `pinguin-data` volume declaration.
Docker can prefix the actual volume name with the Compose project name.
The image declares `/web` as a separate UI volume.
Compose mounts the `pingwin-web` volume at `/web` for Pingwin and ghttp.

## Send the first notification

1. Open [the local workspace](http://localhost:8080).
2. Sign in through the shared header.
3. Open **Tenants** and create a tenant with a display name and complete external SMTP profile.
4. Save the API key from the creation dialog.
   Pingwin displays the raw key once.
5. Build the native CLI:

   ```bash
   go build -o bin/pingwin-cli ./cmd/client
   ```

6. Read the key into the current shell without shell-history text:

   ```bash
   read -r PINGWIN_API_KEY
   ```

7. Send an immediate email to an address that you can inspect:

   ```bash
   ./bin/pingwin-cli send \
  --grpc-server-addr localhost:50051 \
  --api-key "$PINGWIN_API_KEY" \
  --type email \
  --recipient you@example.com \
  --subject "Pingwin first send" \
  --message "Hello from Pingwin"
   ```

8. Open **Event log** and select the same tenant.
   Find the returned notification ID and inspect its status.
9. Confirm receipt in the destination inbox.
   Status `sent` alone establishes provider handoff.

An immediate provider failure can produce an `errored` record for later retry.
The [API guide](api.md) provides status retrieval, schedules, attachment examples, and queued actions.

## Stop and inspect the stack

Stop the stack with the same profile and override:

```bash
make down DOCKER_COMPOSE='docker compose -f docker-compose.yaml -f /tmp/pingwin-local.yml'
```

For a published Pingwin image, use the `docker` profile:

```bash
COMPOSE_PROFILE=docker make up
COMPOSE_PROFILE=docker make down
```

That profile still pulls TAuth and ghttp.
Its Pingwin service is named `pingwin`.
A host gRPC override for that profile must target `services.pingwin` instead of `services.pingwin-dev`.

Inspect Compose volumes with:

```bash
docker compose volumes
```

For a known actual volume name, inspect its details with `docker volume inspect <actual-volume-name>`.
The Compose declaration retains its existing `pinguin-data` identity.

## Native server startup

Native startup uses `configs/config.yml`, rather than the template filename.
Prepare that file from the tracked template:

```bash
cp configs/config.pingwin.yml configs/config.yml
```

Supply every referenced variable through the process environment.
For a private file with shell-compatible assignments, load it in a subshell:

```bash
(
  set -a
  . ./configs/.env.pingwin
  set +a
  ./bin/pingwin
)
```

For native use, set `DATABASE_PATH` to a host path before startup.
The Compose path `/app/data/pingwin.db` is container-specific.
Build the server first with `make build`.
For development, `go run ./cmd/server` is the direct source command.
The native server listens on `:50051`.
An empty database receives the managed schema.

Run one server for each SQLite database.
Native use with HTTP still requires the configured TAuth service and browser assets.
A separate empty database starts with zero tenants.
Tenant creation and an API key must precede its first gRPC request.

## Failure diagnosis

- For a missing-variable or schema error, correct the named input before restart.
- For HTTP authentication failure, compare signing keys, cookie names, tenant IDs, origins, and Google client IDs.
- For a refused gRPC connection, verify the local override and the active Compose profile.
- For email failure, inspect the tenant SMTP profile and notification status.
- For configuration errors, run the [doctor procedure](configuration.md#configuration-validation).
