# AGENTS.md

## Forward-Only Contract Discipline

This repository follows a forward-only, confident programming paradigm. This is a binding agent contract: no fallbacks, no backward compatibility, no legacy support, and no compatibility shims. Do not spend design or implementation effort on backward compatibility considerations except for explicit one-off data migrations into the current canonical contract.

Repeat for emphasis because this rule is binding: no fallbacks, no backward compatibility, no legacy compatibility. Delete or reject obsolete code paths, stale schemas, deprecated config, and old persisted shapes instead of preserving them through compatibility layers, dual reads/writes, aliases, or best-effort recovery.

One-off data migrations are allowed only when they move existing persisted data into the current schema in a bounded operation. After migration, remove the bridge and keep only the current contract.

## Pinguin Notification Service

Pinguin is a production‑quality notification service written in Go. It exposes a gRPC interface for sending **email** and **SMS** notifications on a schedule. See README.md for details

## Document Roles

- NOTES.md: Read-only process playbook maintained by leads. Agents never edit it during implementation cycles.
- ISSUES.md: Append-only log of newly discovered requests and changes. No instructive sections live here; each entry records what changed or what was discovered.
- PLAN.md: Working plan for one concrete change/issue; ephemeral and replaced per change.

### Document Precedence

- `AGENTS.md` (this file) defines repo-wide workflow, testing philosophy, and agent behavior; stack-specific AGENTS.* guides refine these rules for each technology.
- `issues.md/AGENTS.*.md` files never contradict `AGENTS.md` or `POLICY.md`; if guidance appears inconsistent, defer to `POLICY.md` first, then `AGENTS.md`, and treat the stack guide as a refinement.
- `issues.md/NOTES.md` is process-only and must not introduce rules that conflict with `POLICY.md` or any `AGENTS*.md` files.
- `issues.md/PLANNING.md` for planning stage.
- `issues.md/POLICY.md` defines binding validation, error-handling, and “confident programming” rules.

### Issue Status Terms

- Resolved: Completed and verified; no further action.
- Unresolved: Needs decision and/or implementation.
- Blocked: Requires an external dependency or policy decision.

### Validation & Confidence Policy

All rules for validation, error handling, invariants, and “confident programming” (no defensive checks, edge-only validation, smart constructors, CI gates) are defined in POLICY.md. Treat that document as binding; this file does not restate them.

### Legacy & Compatibility Policy

Pinguin does not support backward compatibility. Legacy data, schemas, config keys, endpoints, users, and historical behavior are invalid state. Delete or reject them rather than preserving, migrating, translating, or routing around them. The application supports one current code path only; do not add compatibility branches, fallbacks, or migration shims.

### Build & Test Commands

- Use the repository `Makefile` for local automation. Invoke `make test`, `make lint`, `make ci`, or other documented targets instead of running ad-hoc tool commands.
- `make test` runs the canonical test suite for the active stack.
- `make lint` enforces linting rules before code review.
- `make ci` is the local CI contract and should pass locally before opening a PR.

### Tooling Workflow (Tests, Lint, Format)

- For any change intended to land, agents MUST ensure that all required tooling for the relevant stack (tests, linters, and formatters as defined in `AGENTS*` and `POLICY.md`) passes cleanly on the branch before code is merged or released.
- `NOTES.md` defines the concrete workflow for humans (when and how to invoke specific commands such as `make test`, `make lint`, `make ci`, and formatter targets); agents should treat those steps as given but do not need to restate or modify them.

### Testing Philosophy

- Testing follows an **inverted test pyramid**: most coverage comes from high-value black-box integration and end-to-end tests; unit tests are optional and exist only when they add clear implementation guardrails.
- We **strive for 100% test coverage**, achieved primarily through integration/black-box suites whose scenarios are exhaustive enough to exercise all meaningful branches and error paths.
- For CLI and backend services, tests compile or run the real program/CLI entrypoints, capture exit codes and output (stdout/stderr, files, side effects), and assert against those observable results—not internal functions.
- For web/UI, tests run the app and backing web server, drive flows through the browser, and assert against the rendered page, DOM state, events, and other user-visible behavior.
- Unit tests are acceptable as **implementation guardrails**, but they are not product-level acceptance criteria, must not be the primary mechanism for achieving coverage, and may be removed when equivalent or stronger integration coverage exists.

## Tech Stack Guides

Stack-specific instructions now live in dedicated files. Apply the relevant guide alongside the shared policies above.

- Front-End (Browser ES Modules with Alpine.js): `issues.md/AGENTS.FRONTEND.md`
- Backend (Go): `issues.md/AGENTS.GO.md`
- Docker and containerization: `issues.md/AGENTS.DOCKER.md`
- Git and version control workflow: `issues.md/AGENTS.GIT.md`
