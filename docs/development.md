# Development

## Toolchain

The Go module requires Go 1.25.4 or later.
The backend uses SQLite through GORM.
The browser uses ES modules, Alpine.js, and `mpr-ui`.
The repository uses its Makefile for local automation.
GitHub Actions are disabled.

Install the locked browser dependencies and Playwright browsers:

```bash
npm ci
npx playwright install --with-deps
```

Node.js and npm are required for these commands.
Build the server with `make build`.
The output is `bin/pingwin`.
The CLI and doctor build commands are in the [API](api.md) and [configuration](configuration.md) guides.

## Validation targets

| Target | Scope |
| --- | --- |
| `make check-format` | Verify Go formatting. |
| `make format` | Format Go source files. |
| `make lint` | Run go vet, staticcheck, and ineffassign. |
| `make test` | Run fast Go packages and integration packages. |
| `make test-coverage` | Require 100% total Go statement coverage. |
| `make test-frontend` | Run the Playwright browser suite. |
| `make test-shared-ui` | Run focused shared-library candidate scenarios. |
| `make ci` | Run format checks, analysis, Go tests, coverage, and browser acceptance. |

Browser tests cover authentication, tenant lifecycle, responsive layout, Event log, SMTP relay, accessibility, and visual snapshots.
The harness starts a local server with the managed tenant API and TAuth test boundary.
Google responses are controlled inputs.
Browser acceptance does not establish live Google sign-in or provider delivery.
The [shared UI migration](mpr-ui-migration.md) explains pinned candidate qualification and hosted activation.

The reusable notification worker comes from `github.com/tyemirov/utils/scheduler`.
Its repository and dispatcher interfaces permit reuse in other binaries.
The [delivery guide](smtp_delivery_plan.md) describes the Pingwin adapter and protocol tests.

## Visual baselines

Snapshots use one host-independent path, locale `en-US`, and timezone `America/Los_Angeles`.
For an accepted visual change, refresh Pingwin-owned baselines:

```bash
make test-frontend-update
```

Then run the normal browser suite:

```bash
make test-frontend
```

## Protobuf generation

Edit the canonical [`pkg/proto/pingwin.proto`](../pkg/proto/pingwin.proto) definition for gRPC contract changes.
Install `protoc` and the Go protobuf generators for regeneration.
Then run:

```bash
make generate-proto
```

The generated Go package remains under the `github.com/tyemirov/pinguin` module.
The RPC namespace is `pingwin.NotificationService`.

## Browser implementation

The [architecture document](../ARCHITECTURE.md#browser-structure) assigns modules and shared-shell responsibilities.
The component references are [`web/js/core/README.md`](../web/js/core/README.md) and [`web/js/ui/README.md`](../web/js/ui/README.md).
