# Shared UI Migration

I004 prepares Pinguin for the shared authentication and footer contracts under mpr-ui I009.
The four pages retain their declarative bootstrap and literal `@latest` assets.
The application owns `web/config-ui.yaml`.
Each environment declares `auth.providers.google`, `auth.providers.apple`, and `auth.providers.password`.
Google is enabled. Apple and password are disabled.
The Google client identifiers, tenant identifiers, origins, and endpoint paths retain their configured values.
The explicit session endpoint remains `/auth/session`.

The shared library controls authentication. Pinguin consumes its documented events through `sessionBridge.js`.
The shared footer uses `menu` for its seven-service catalog.

## Validation

Run `make test-shared-ui` to verify each page at mobile and desktop widths.
Run `make test-frontend` for the complete browser suite.
Run `make ci` for Go analysis, Go tests, coverage, and browser acceptance.
The repository contract disables GitHub Actions. Local CI provides the application validation result.

The browser suite loads the real shared candidate at the production asset URLs.
`tests/e2e/shared-ui-candidate.json` records its immutable revision and SHA-256 digests.
The test boundary verifies each digest before it serves those bytes.
The suite uses the application pages, real shared components, and the local API test server.
Google responses remain controlled test inputs.
The tests exercise authentication, session restoration, tenant operations, notification operations, SMTP operations, layout, and themes.

## Activation

The owner controls production activation.
Complete consumer preparation under mpr-ui I009 before shared publication.
Use `mpr-ui/docs/config-migration-deployment-plan.md` for the coordinated interruption and cache procedure.
Verify the published HTML, YAML, loader, bundle, and CSS as one current contract.
Verify real Google sign-in, session restoration, and sign-out on the hosted frontend.
Keep the managed tenant conversion gates under F001 separate from this source migration.

The [public asset record](mpr-ui/public-assets-2026-09-09.json) contains eight HTTP observations from one network location.
The application pages and configuration declare `max-age=600`.
The shared assets declare `max-age=604800` and `s-maxage=43200`.
Browser cache qualification remains a separate acceptance requirement.
