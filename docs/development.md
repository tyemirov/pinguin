# Development

## Toolchain

The Go module requires Go 1.25.4 or later.
The backend uses SQLite through GORM.
The browser uses ES modules, Alpine.js, and `mpr-ui`.
The repository uses its Makefile for local automation.

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
| `make lint` | Analyze Go code and tests with go vet, staticcheck, and ineffassign. |
| `make test` or `make test-unit` | Run the in-process Go suite with controlled dependencies. |
| `make test-coverage` | Run the unit suite and require 100% Go statement coverage. |
| `make test-integration` | Run the separately selected CLI, service, SQLite contention, and Gateway integration checks. |
| `make test-image` | Build the production image and verify the server, doctor, and current executable set. |
| `make test-frontend` | Run the Playwright browser suite. |
| `make test-shared-ui` | Run focused shared-library candidate scenarios. |
| `make test-delivery` | Run focused delivery-history Go, HTTP, configuration, and startup scenarios. |
| `make test-delivery-contract` | Check affected GORM, environment, and production declaration contracts. |
| `make test-delivery-coverage` | Collect focused delivery-history coverage through `COVERAGE_PROFILE`. |
| `make test-delivery-frontend` | Run delivery-history and affected navigation browser scenarios. |
| `make ci` | Run Go format checks, analysis, and the unit coverage gate. |

The unit suite uses in-process protocol peers, HTTP recorders, and isolated in-memory SQLite fixtures.
Put Go integration cases in files with the `integration` build tag and use the `TestIntegration` prefix for their test functions.
Select those checks when a change needs evidence from real component interactions.

Browser tests cover authentication, tenant lifecycle, responsive layout, Event log, SMTP relay, accessibility, and visual snapshots.
The harness starts a local server with the managed tenant API and TAuth test boundary.
Google responses are controlled inputs.
Browser acceptance does not establish live Google sign-in or provider delivery.
The [shared UI migration](mpr-ui-migration.md) explains pinned candidate qualification and hosted activation.

The reusable notification worker comes from `github.com/tyemirov/utils/scheduler`.
Its repository and dispatcher interfaces permit reuse in other binaries.
The [delivery guide](smtp_delivery_plan.md) describes the Pingwin adapter and protocol tests.

## Issue maintenance

The [active tracker](../.mprlab/ISSUES.md) contains current work and recurring maintenance.
The [resolved archive](../.mprlab/ISSUES.archive.md) preserves complete issue entries.
The [historical snapshot](../issues.md/ISSUES.archive.md) preserves earlier identifiers and contracts.
Review completed issues for documentation changes before moving them to the archive.
Update the document that owns each changed product, architecture, operator, or testing contract.
Keep production operations in the relevant runbook and use code validation for implementation closure.

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

Go clients import packages from the `github.com/tyemirov/pingwin` module.
The RPC namespace is `pingwin.NotificationService`.

Install the Go client from the latest source version.

```bash
go get github.com/tyemirov/pingwin/pkg/client@latest
```

## Browser implementation

The [architecture document](../ARCHITECTURE.md#browser-structure) assigns modules and shared-shell responsibilities.
The component references are [`web/js/core/README.md`](../web/js/core/README.md) and [`web/js/ui/README.md`](../web/js/ui/README.md).
