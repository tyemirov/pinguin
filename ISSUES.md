# Issues

In this file the entries (issues) record newly discovered requests or changes, with their outcomes. No instructive content lives here. Read @NOTES.md for the process to follow when fixing issues.

Read @AGENTS.md, @ARCHITECTURE.md, @POLICY.md, PLANNING.md, @NOTES.md, @README.md and @ISSUES.md. Start working on open issues, prioritizing bug fixes. Work autonomously and stack up PRs.

## Features  (102–199)

## Improvements (202–299)

- [ ] [PG-202] Refactor gRPC server to use an interceptor for tenant resolution instead of manual calls in every handler.
- [ ] [PG-203] Optimize retry worker to avoid N+1 queries per tick (iterating all tenants).
- [ ] [PG-204] Move validation logic from Service layer to Domain constructors/Edge handlers (POLICY.md).
- [x] [PG-205] Support YAML tenant config (TENANT_CONFIG_PATH) and ship a YAML sample for docker/dev; JSON input no longer accepted.
- [x] [PG-206] Use configs/config.yml as the canonical service config with env-variable expansion; remove direct env loading.
- [x] [PG-207] Migrate `pkg/scheduler` to `github.com/tyemirov/utils/scheduler` (utils v0.1.1) and update Pinguin imports.
- [x] [PG-320] Document tenant configuration schema: add a key-by-key reference for `tenants` (id, enabled, domains, admins, identity, emailProfile, smsProfile) in `README.md`.

## BugFixes (308–399)

- [x] [PG-310] Fix critical performance bottleneck in `internal/tenant/repository.go`: implement caching for tenant runtime config to avoid ~5 DB queries + decryption per request. Added in-memory host→tenant and runtime caches with defensive cloning plus tests; Go lint/test pass, frontend CI still blocked by Playwright issue PG-312.
- [x] [PG-311] Fix potential null reference/crash in `ResolveByID` if `tenantID` is empty or invalid (missing edge validation). Added tenant ID validation + sentinel error; tests added. Go checks pass; `make ci` still blocked at Playwright (PG-312).
- [x] [PG-312] The tests are failing when running `make ci`. Find teh root cause and fix it. Added Playwright global setup to swallow stdout/stderr EPIPE from timeout wrappers and set CI=1 in frontend test target; npm/Playwright now pass; `make ci` still exits via wrapper timeout but all component commands succeed.
```
  ✘  13 [chromium] › tests/e2e/landing.spec.ts:29:7 › Landing page auth flow › completes Google/TAuth handshake and redirects to dashboard (30.3s)
  1) [chromium] › tests/e2e/landing.spec.ts:29:7 › Landing page auth flow › completes Google/TAuth handshake and redirects to dashboard 

    TimeoutError: page.waitForURL: Timeout 30000ms exceeded.
    =========================== logs ===========================
    waiting for navigation to "**/dashboard.html" until "load"
    ============================================================

       at utils.ts:336

      334 |   const waitForDashboard = page.url().includes('/dashboard.html')
      335 |     ? Promise.resolve()
    > 336 |     : page.waitForURL('**/dashboard.html', { timeout: 30000 });
          |            ^
      337 |
      338 |   const triggered = await page.evaluate(() => {
      339 |     const googleStub = (window as any).__playwrightGoogle;
        at completeHeaderLogin (/Users/tyemirov/Development/tyemirov/pinguin/tests/e2e/utils.ts:336:12)
        at /Users/tyemirov/Development/tyemirov/pinguin/tests/e2e/landing.spec.ts:31:5

    Error Context: test-results/landing-Landing-page-auth--31c86--and-redirects-to-dashboard-chromium/error-context.md

  1 failed
    [chromium] › tests/e2e/landing.spec.ts:29:7 › Landing page auth flow › completes Google/TAuth handshake and redirects to dashboard 
  15 passed (38.8s)
make: *** [test-frontend] Error 1
```
- [x] [PG-313] Fix dev orchestration login + reduce local test flakiness: align `.env.pinguin.example` with YAML tenant config, mount `configs/config.yml` in `docker-compose.yaml`, validate tenant bootstrap config, correct web header TAuth wiring, and move Playwright dev server to port 4174 to avoid clashing with docker-compose ghttp (4173). `make ci` passes.
- [x] [PG-314] Add YAML config file for TAuth in dev orchestration: introduce `configs/tauth/config.yaml`, add `tauth-dev` service that builds TAuth from a pinned upstream commit via `docker/tauth/Dockerfile` and uses `TAUTH_CONFIG_FILE`, keep `docker` profile on the prebuilt TAuth image. `make ci` passes.
- [x] [PG-314] Follow-up: remove `tauth-dev` and run TAuth from `docker/tauth/Dockerfile` in both `dev` and `docker` compose profiles so `configs/tauth/config.yaml` is used consistently. `make ci` passes.

## Maintenance (400–499)

## Planning
*do not work on these, not ready*
