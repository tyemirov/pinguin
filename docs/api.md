# API and CLI

## gRPC contract

The canonical definition is [`pkg/proto/pingwin.proto`](../pkg/proto/pingwin.proto).
The service is `pingwin.NotificationService`.
Its methods send, retrieve, list, reschedule, and cancel notifications.

Each request requires `Authorization: Bearer pgn_1_<credential-uuid>_<secret>`.
The API key resolves the tenant.
The caller does not select a tenant in the protobuf request or gRPC metadata.
The server and CLI permit gRPC messages up to 32 MiB.

The local server uses plaintext gRPC.
The CLI also uses plaintext transport.
The deployment declaration exposes gRPC through a private LAN route.
The [getting-started guide](getting-started.md) supplies a loopback host mapping for local Compose clients.

## CLI

The Cobra CLI at `cmd/client` accepts explicit flags and submits one request.
Build it from the repository root:

```bash
go build -o bin/pingwin-cli ./cmd/client
./bin/pingwin-cli send --help
```

| Flag | Purpose | Default |
| --- | --- | --- |
| `--grpc-server-addr` | Target gRPC endpoint | `localhost:50051` |
| `--api-key` | Tenant API key | Required |
| `--connection-timeout-sec` | Dial timeout in seconds | `5` |
| `--operation-timeout-sec` | Command timeout in seconds | `30` |
| `--log-level` | `DEBUG`, `INFO`, `WARN`, or `ERROR` | `INFO` |

Send an immediate email:

```bash
./bin/pingwin-cli send   --api-key "$PINGWIN_API_KEY"   --type email   --recipient someone@example.com   --subject "Meeting reminder"   --message "See you at 10:00"
```

`--to` is also accepted for the recipient.
For SMS, use `--type sms` and the recipient phone number.
The tenant must have a complete Twilio profile.

For scheduled delivery, add `--scheduled-time` with a future RFC3339 timestamp.
A future request remains queued until the worker selects it at or after that time.
A past send timestamp triggers immediate dispatch.
Reschedule operations require a future timestamp.

Add attachments with repeatable `--attachment` flags:

```bash
./bin/pingwin-cli send   --api-key "$PINGWIN_API_KEY"   --type email   --recipient someone@example.com   --subject "Weekly report"   --message "See the attached report."   --attachment /tmp/report.pdf   --attachment /tmp/notes.txt::text/plain
```

An attachment value is a path or `path::content-type`.
The CLI infers a missing MIME type from the extension or content.
Its final default is `application/octet-stream`.
Email permits 10 files, 5 MiB per file, and 25 MiB total attachment bytes.
Attachments are persisted with scheduled and retry records.

## grpcurl

Run these commands from the repository root with [grpcurl](https://github.com/fullstorydev/grpcurl) installed.
The server does not register reflection.
Each example supplies the protobuf source and plaintext transport explicitly.

Send an immediate email:

```bash
grpcurl -plaintext -import-path . -proto pkg/proto/pingwin.proto   -H "Authorization: Bearer $PINGWIN_API_KEY"   -d '{"notification_type":"EMAIL","recipient":"someone@example.com","subject":"Test email","message":"Hello from Pingwin"}'   localhost:50051 pingwin.NotificationService/SendNotification
```

For scheduling, add a future RFC3339 `scheduled_time` to the JSON object.
Protobuf encodes attachment `bytes` as base64 in JSON.
This example attaches a text file with the content `Hello` and a newline:

```bash
grpcurl -plaintext -import-path . -proto pkg/proto/pingwin.proto   -H "Authorization: Bearer $PINGWIN_API_KEY"   -d '{"notification_type":"EMAIL","recipient":"someone@example.com","subject":"Attachment","message":"See attached text.","attachments":[{"filename":"hello.txt","content_type":"text/plain","data":"SGVsbG8K"}]}'   localhost:50051 pingwin.NotificationService/SendNotification
```

Retrieve a returned notification ID:

```bash
grpcurl -plaintext -import-path . -proto pkg/proto/pingwin.proto   -H "Authorization: Bearer $PINGWIN_API_KEY"   -d '{"notification_id":"<notification-id>"}'   localhost:50051 pingwin.NotificationService/GetNotificationStatus
```

For large requests, supply `-max-msg-sz 33554432` to align grpcurl with the service message limit.

## Notification lifecycle

A future scheduled request is stored with status `queued`.
An immediate request attempts provider delivery before database insertion.
A successful attempt produces `sent`.
A provider error produces `errored` when the request reaches persistence.
An initial database-write failure can occur after a provider accepts the message.

The worker selects due `queued` or `errored` records below `MAX_RETRIES`.
It uses exponential backoff between retry attempts.
Status `errored` alone does not identify retry exhaustion.
The retry count and configured limit also matter.
Status `sent` establishes provider handoff, rather than inbox arrival or handset receipt.

Only queued notifications can be rescheduled or cancelled.
Cancellation changes status to `cancelled` and removes the schedule.
The [delivery guide](smtp_delivery_plan.md) explains the separate SMTP paths and duplicate-delivery risks.

## HTTP management API

The Gin HTTP API exposes:

- `GET /runtime-config` for tenant-independent workspace URLs.
- Unauthenticated `GET /healthz` for readiness.
- `/api/tenants` for owner-scoped list and create operations.
- `/api/tenants/:tenant_id` for one tenant and its nested resources.

The nested resources include profiles, credentials, notifications, SMTP domains, and SMTP identities.
Notification submission uses gRPC.
The HTTP notification routes list records and change queued notifications.

Each `/api` request requires a validated TAuth session cookie.
Tenant creation requires `Idempotency-Key`.
Versioned updates and deletion require `If-Match`.
Management representations exclude provider secrets and raw API secrets.
SMTP identity passwords have a separate authenticated retrieval operation.

Expected failures use this envelope:

```json
{"error":{"code":"...","message":"...","request_id":"..."}}
```

| Status | Meaning |
| --- | --- |
| `401` | Authentication failure |
| `404` | Absent or foreign resource |
| `409` | Conflict |
| `412` | Stale version |
| `415` | Unsupported media type |
| `422` | Invalid domain value |

CORS permits configured origins and credentials.
HTTP logs honor forwarding headers only from configured trusted proxies.
The [managed tenant contract](multitenancy-plan.md#http-contract) contains the resource schemas and route details.
