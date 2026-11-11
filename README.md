# Pingwin

Pingwin is a self-hosted service for application notifications and domain email routing.
It sends email and SMS through gRPC, accepts authenticated SMTP submissions, and forwards shared addresses to existing inboxes.
A browser workspace manages tenants, credentials, notification records, and sender domains.

Use Pingwin to:

- Centralize scheduled application email and SMS with tenant-specific provider credentials.
- Create exact sender identities for clients such as Gmail Send-As.
- Forward a shared address to one or more existing inboxes.

The service name is `pingwin`.
The source repository and Go module remain `github.com/tyemirov/pinguin`.

## Capabilities and limits

| Capability | Current behavior |
| --- | --- |
| Notifications | Email through a tenant SMTP profile. SMS through an optional Twilio profile. |
| Schedules and retries | SQLite retains notification records and attachments. A worker dispatches due requests and retries eligible failures with exponential backoff. |
| Attachments | Email supports 10 files, 5 MiB per file, and 25 MiB combined. The gRPC message limit is 32 MiB. |
| Tenant management | TAuth owners manage their tenants through the browser or HTTP API. Each tenant has one rotatable API key. |
| SMTP submission | Verified domains and exact sender credentials. Delivery uses an upstream relay or direct recipient MX hosts. |
| Direct SMTP | Each message can address one recipient domain. Pingwin provides no DKIM signer or bounce processor. |
| Shared addresses | Synchronous forwarding through a shared upstream relay. Pingwin stores no forwarded message body or mailbox state. |

Notification status `sent` means successful provider handoff, rather than confirmed receipt.
Status `errored` remains eligible for retries below the configured limit.
SMTP submission and forwarding use synchronous delivery, separate from the notification worker.
The [delivery guide](docs/smtp_delivery_plan.md) explains persistence, transport behavior, and duplicate-delivery risks.

Each tenant requires a complete external SMTP profile, even when its intended use is SMS or direct SMTP submission.
Provider secrets and SMTP identity passwords are encrypted at rest.
Notification content and attachment bytes remain in SQLite.
API secrets are stored as SHA-256 digests and displayed once.

## Project status

The current source implements managed tenants, notification operations, sender-domain verification, SMTP submission, and shared-address forwarding.
Provider-backed mailboxes remain planned work.

Production acceptance for managed tenants and the shared UI remains pending in the project records.
The Pingwin hostname change is declared for the next deployment.
These source changes do not establish production availability.
The [deployment guide](docs/deployment.md) preserves the conversion, rename, and activation prerequisites.
The [issue tracker](.mprlab/ISSUES.md) records F001, F002, I004, and I007.

## Quickstart

The local stack requires Docker Compose, Go 1.25.4 or later, and a Google OAuth web client.
Tenant creation also requires an external SMTP account.
SMS requires Twilio credentials.

1. Clone the repository:

   ```bash
   git clone https://github.com/tyemirov/pinguin.git pingwin
   cd pingwin
   ```

2. Complete [local configuration](docs/getting-started.md#prepare-the-local-stack).
   This step creates private environment files and aligns the browser with TAuth.
3. Start the stack with the documented local gRPC override:

   ```bash
   make up DOCKER_COMPOSE='docker compose -f docker-compose.yaml -f /tmp/pingwin-local.yml'
   ```

4. Open [the local workspace](http://localhost:8080).
   Sign in and create a tenant with its SMTP profile.
   Save the API key when Pingwin displays it.
5. Complete [the first-send procedure](docs/getting-started.md#send-the-first-notification).
   Send an immediate email and inspect its returned status.
6. Stop the stack with the same override:

   ```bash
   make down DOCKER_COMPOSE='docker compose -f docker-compose.yaml -f /tmp/pingwin-local.yml'
   ```

The [getting-started guide](docs/getting-started.md) includes ports, native startup, expected results, and failure diagnosis.

## Interfaces

| Interface | Purpose | Authentication |
| --- | --- | --- |
| gRPC | Send, list, inspect, reschedule, and cancel notifications. | Tenant bearer API key. |
| HTTP API | Manage tenants, profiles, credentials, notifications, and SMTP resources. | TAuth session cookie. |
| Browser | Tenants, Event log, and SMTP relay workspaces. | TAuth through the shared MPR UI. |
| SMTP submission | Send as one verified identity. | Identity-specific SMTP credentials. |
| SMTP forwarding | Receive mail for configured shared addresses. | Public listener with recipient restrictions. |

The gRPC namespace is `pingwin.NotificationService`.
The canonical schema is [pingwin.proto](pkg/proto/pingwin.proto).
The [API guide](docs/api.md) contains CLI examples, complete `grpcurl` commands, HTTP contracts, and notification status rules.

## Documentation

| Guide | Contents |
| --- | --- |
| [Configuration](docs/configuration.md) | Service settings, private inputs, tenant ownership, schema rules, doctor, and logs. |
| [SMTP setup](docs/smtp-setup.md) | DNS verification, Gmail Send-As, forwarding, listener limits, and gateway TLS. |
| [Delivery behavior](docs/smtp_delivery_plan.md) | Notification and SMTP delivery paths, storage, retries, and transport limits. |
| [Deployment](docs/deployment.md) | Release, publication, deployment, retained data, and production acceptance. |
| [Architecture](ARCHITECTURE.md) | Components, ownership, persistence, browser modules, and deployment boundaries. |
| [Managed tenant contract](docs/multitenancy-plan.md) | Resource schemas, API details, conversion procedure, and acceptance criteria. |
| [Shared UI migration](docs/mpr-ui-migration.md) | Shared-library qualification and production activation requirements. |

## Development

The backend uses Go, SQLite, and GORM.
The browser uses ES modules, Alpine.js, and `mpr-ui`.
Local CI runs through the repository Makefile.
GitHub Actions are disabled for this repository.

```bash
make build
make test
make lint
make test-frontend
make ci
```

`make ci` includes format checks, Go analysis, Go tests, the 100% Go statement coverage gate, and Playwright browser tests.
The browser harness uses a local API and controlled authentication responses.
Those tests do not establish live provider connectivity or hosted sign-in acceptance.
The [development guide](docs/development.md) covers dependencies, test targets, protobuf generation, and visual baselines.

## License and commercial use

The repository uses the [PolyForm Noncommercial License 1.0.0](LICENSE), subject to the scope in [NOTICE](NOTICE).
The [commercial licensing notice](COMMERCIAL_LICENSE.md) describes separate written agreements through Marco Polo Research Lab LLC.
The contact address is legal@mprlab.com.
Contributions are subject to [CONTRIBUTOR_LICENSE.md](CONTRIBUTOR_LICENSE.md).
