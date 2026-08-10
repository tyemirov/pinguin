# Pinguin Compact MPR UX Refactoring Plan

## Status

This document is the completed durable plan for the Pinguin user interface refactor.

Implementation was authorized and completed on 2026-08-10. Publication, release, and deployment remain separate operator actions.

The current `PLAN.md` remains the implementation ledger for its active change.

## Objective

Change the Pinguin browser interface to use the compact MPR visual language.

Keep the current product capabilities, routes, API payloads, authorization rules, and authentication flow.

Improve information density, responsive behavior, task clarity, and operator feedback.

## Current Evidence

The current interface uses a light and spacious SaaS presentation.

The landing page uses a gradient canvas, a large marketing heading, and a floating preview card.

The application panels use large radii, large shadows, large gaps, and pill buttons.

The Event log uses a six-column table. At `390px`, the viewport does not show all record data and actions.

The populated SMTP relay page has a `599px` document width in a `390px` viewport.

The same SMTP page has a `1,952px` document height with two sender domains.

The SMTP page shows DNS requirements and DNS check results in separate components.

The fixed footer can cover SMTP content and controls during page movement.

The shared header can also cause horizontal page overflow on a small screen.

## Product Contract

Keep these current contracts:

- Use `mpr-ui` and TAuth as the only browser authentication authority.
- Route a restored or completed session to `event-log.html`.
- Route an anonymous protected request to `index.html`.
- Keep Event log and SMTP relay as the two protected product destinations.
- Use the current tenant list and tenant authorization rules.
- Use the current notification list, search, filter, page, reschedule, and cancel APIs.
- Use the current sender domain, SMTP identity, credential, rotation, forwarding, and deletion APIs.
- Keep the current JSON payloads and persisted schemas.
- Keep one current front-end path for each operation.

## Surface Classification

The landing page is a public site and application hybrid.

The Event log is an operator dashboard.

The SMTP relay is an administration and setup workflow.

The authenticated surfaces require the strongest MPR treatment.

The SMTP page uses progressive disclosure instead of a permanent side rail.

This adaptation keeps more width available for DNS values and identity records.

## Canonical User Flow

### Authentication And Navigation

1. Open the compact Pinguin overview.
2. Sign in with the single shared-header control.
3. Continue to Event log after successful authentication.
4. Use the workspace navigation to select Event log or SMTP relay.
5. Sign out with the shared profile menu.
6. Return to the landing page after sign-out.

The workspace navigation must show the current destination with `aria-current="page"`.

### Event Log

1. Select a tenant when the session can use more than one tenant.
2. Enter a search value when a smaller result set is necessary.
3. Select one status filter or all statuses.
4. Review the loaded record count in the toolbar.
5. Review each notification in a compact record row.
6. Reschedule a queued notification with an application dialog.
7. Cancel a queued notification with an application confirmation dialog.
8. Load the next page with the current scroll sentinel.

Each notification row must keep this information available:

- Subject.
- Recipient.
- Tenant.
- Status.
- Creation time.
- Scheduled time.
- Applicable actions.

Terminal notifications must not reserve space for unavailable actions.

### SMTP Relay

1. Add a sender domain from the Sender domains toolbar.
2. Expand the new sender domain.
3. Copy each required DNS host and value.
4. Publish the DNS records with the domain provider.
5. Select Check DNS in Pinguin.
6. Review each DNS result in its related DNS row.
7. Create an identity after one sender domain has the Verified status.
8. Enter the identity local part.
9. Select one verified sender domain.
10. Enter one or more forwarding owners.
11. Copy the Gmail SMTP settings from the credential dialog.
12. Open an existing identity to view its current credentials.
13. Edit forwarding owners in the selected identity row.
14. Rotate credentials in the credential dialog.
15. Delete an identity with an application confirmation dialog.

The page must expand one sender domain at a time.

Each DNS row must contain its requirement and its current check result.

## MPR Design Contract

### Color Tokens

| Role | Value | Use |
| --- | --- | --- |
| Canvas | `#0f1114` | Page background |
| Raised surface | `#16181c` | Header, footer, and overlays |
| Panel | `#1f2126` | Work panels and record rows |
| Strong border | `#2c2f36` | Default separation |
| Hover border | `#3b3f48` | Hover and strong separation |
| Primary text | `#e3e5ec` | Main content |
| Support text | `#c4c7d1` | Secondary content |
| Muted text | `#727887` | Metadata and help text |
| Primary signal | `#5d93ff` | Selection and primary actions |
| Success signal | `#95c23d` | Sent and verified states |
| Danger signal | `#cc4b4b` | Error and destructive actions |
| Warning signal | `#f2b84b` | Queued and pending states |
| Information signal | `#b487ff` | Informational state |

Use charcoal surfaces as the base experience.

Use signal colors for status, selection, focus, and primary actions.

Use tinted borders and backgrounds for most signal states.

### Typography

- Use `15px` as the base font size.
- Use approximately `0.78rem` for controls and primary interface copy.
- Use approximately `0.86rem` for record titles.
- Use approximately `0.65rem` for metadata.
- Use approximately `0.95rem` for page titles.
- Use medium or semibold weight for labels and controls.
- Use uppercase text and light tracking for short metadata.
- Use monospace text only for DNS values and SMTP settings.

### Shape And Space

- Use `4px` to `6px` radii for controls and panels.
- Use `10px` radii for dialogs and other overlays.
- Use full-round radii only for chips and badges.
- Use one-pixel borders for primary separation.
- Use gaps from `0.25rem` through `0.75rem`.
- Use restrained shadows only for dialogs, menus, and toasts.
- Use `120ms` through `150ms` transitions for controls.
- Use approximately `250ms` for a panel entrance.

### Theme

Use the MPR dark theme as the initial theme on all three pages.

Keep the shared theme control.

Map the light theme to the same density, shape, border, and signal roles.

## Shell And Navigation

Use one shell for all Pinguin pages.

The shell contains these elements:

1. A compact shared header.
2. A centered work surface.
3. A compact shared footer.

Keep the brand, workspace navigation, and authentication control in the shared header.

Keep Docs as a secondary destination.

Place Event log and SMTP relay in the shared `mpr-ui` header navigation slot.

Use a maximum width of `960px` for the landing page and Event log.

Permit a maximum width of `1180px` for expanded SMTP DNS content.

Reserve page space for the fixed footer and device safe-area inset.

## Landing Page Changes

Replace the marketing hero with a compact product header.

Use one short product description.

Keep the current notification preview as a bordered operator panel.

Render the preview as dense notification rows.

Use the shared header as the only sign-in location.

Show authentication progress and failure in a compact inline status region.

Remove the gradient canvas, large heading scale, and floating-card treatment.

## Event Log Changes

Replace the table-only layout with a semantic notification list.

Use a stable CSS grid for wide notification rows.

Reflow the same record into compact metadata rows on a small screen.

Use one toolbar for these controls:

- Tenant selection.
- Search.
- Status filters.
- Loaded record count.
- Refresh.

Use compact status chips for filter state and notification state.

Place the subject, recipient, and creation time in the primary record area.

Place tenant, status, and schedule in the metadata area.

Show Reschedule and Cancel only for queued notifications.

Use application dialogs instead of browser confirmation prompts.

Restore focus to the action control after each dialog closes.

Show loading, empty, error, and final-page messages inside the bordered list.

## SMTP Relay Changes

Divide the page into Sender domains and SMTP identities.

Use a compact toolbar for each section.

Show each sender domain as a collapsed summary row by default.

Include the domain, status, check progress, and primary action in each summary.

Expand the new or selected sender domain in one DNS setup panel.

Combine each DNS requirement and DNS check result in one row.

Add copy controls for each DNS host and value.

Use the existing event and toast pattern for copy feedback.

Open the identity editor from an explicit Create identity action.

Enable identity creation after a verified domain is available.

Build the full sender address from the local part and selected verified domain.

Send the current `email_address` and `forward_to` payload to the API.

Show existing identities as dense rows.

Place forwarding edits in the selected identity row.

Keep credential rotation in the credential dialog.

Use monospace text for literal SMTP values.

Keep one status region in the credential dialog.

## Responsive Contract

Use these validation widths:

- `390px` for a compact mobile viewport.
- `768px` for a tablet viewport.
- `1440px` for a desktop viewport.

At each width, keep the document width equal to the viewport width.

Keep all primary content and actions available without horizontal page movement.

Permit an internal code-value area to wrap within its record.

Keep the shared-header workspace navigation on one compact row when space is available.

Use an accessible compact menu when the header cannot contain all controls.

Keep the footer clear of content, dialogs, actions, and keyboard focus.

## Interaction And Accessibility Contract

- Show a visible focus indicator for every interactive control.
- Keep status text with every semantic status color.
- Give each icon control an accessible name.
- Keep keyboard access for all navigation, filters, record actions, and copy controls.
- Trap focus inside each modal dialog.
- Close each modal dialog with Escape.
- Restore focus after each modal dialog closes.
- Keep reduced-motion behavior for nonessential animation.
- Use one clear next action in each empty state.
- Keep error messages near the related work surface.
- Keep toast messages as secondary feedback.

## Front-End Structure

Split the current large CSS file into these responsibilities:

- `web/assets/css/tokens.css` for MPR tokens.
- `web/assets/css/base.css` for resets and shared primitives.
- `web/assets/css/landing.css` for the public surface.
- `web/assets/css/event-log.css` for notification records.
- `web/assets/css/smtp-relay.css` for domain and identity workflows.

Remove inline layout styles from the HTML files.

Keep `web/js/app.js` as the composition root.

Move the current session bridge to `web/js/core/sessionBridge.js`.

Replace `notificationsTable.js` with one canonical `notificationsList.js` module.

Split SMTP behavior into these semantic modules:

- `web/js/ui/smtpDomains.js`.
- `web/js/ui/smtpIdentities.js`.
- `web/js/ui/smtpCredentialsDialog.js`.

Keep user-visible strings in `web/js/constants.js`.

Keep shared type definitions in `web/js/types.d.js`.

Keep API access in `web/js/core/apiClient.js`.

Delete the superseded table-only and all-domains-expanded paths.

## Implementation Sequence

### 1. Add Baseline Acceptance Tests

- Preserve current authentication, authorization, API, and operation behavior.
- Add document-width checks for all validation widths.
- Add fixed-footer overlap checks.
- Add focused visual snapshots for Pinguin-owned content.
- Keep shared remote components outside the visual snapshot boundary.

### 2. Add The MPR Foundation

- Add the MPR token file.
- Add shared compact primitives.
- Set the initial theme to dark.
- Add workspace navigation to the shared `mpr-ui` header.
- Correct header and footer responsive behavior.

### 3. Refactor The Landing Page

- Replace the hero structure.
- Add the compact product header.
- Restyle the notification preview.
- Preserve the shared authentication transition.

### 4. Refactor The Event Log

- Add the responsive notification list.
- Add the compact toolbar and loaded count.
- Add queued-record actions.
- Add application dialogs.
- Preserve search, filter, page, reschedule, and cancel behavior.

### 5. Refactor The SMTP Relay

- Add collapsed domain summaries.
- Add the single expanded DNS panel.
- Merge DNS requirements and check results.
- Add DNS copy controls.
- Add the verified-domain identity editor.
- Add row-level forwarding edits.
- Restyle credential and confirmation dialogs.

### 6. Complete The Module Refactor

- Extract the session bridge.
- Split the SMTP components.
- Delete superseded modules and styles.
- Update the front-end architecture documentation.

### 7. Complete Validation

- Run the smallest front-end target after each implementation slice.
- Run `make test` after the final browser behavior change.
- Run `make lint` after the final JavaScript and CSS change.
- Run `make ci` after the final source, test, config, or documentation change.

## Acceptance Criteria

- All three pages use the MPR dark theme by default.
- All three pages use the defined MPR density and shape contract.
- The landing page has one shared-shell sign-in control.
- The landing page has no gradient hero or oversized marketing heading.
- The shared-header workspace navigation identifies the current protected page.
- The document has no horizontal overflow at each validation width.
- The fixed footer does not cover content or controls.
- Every notification keeps all required data and applicable actions available.
- Event log search, filtering, paging, rescheduling, and cancellation keep current behavior.
- SMTP sender domains use collapsed summaries and one expanded detail panel.
- Each DNS requirement and result appears once.
- Identity creation uses verified domain choices.
- Identity creation sends the current API payload.
- Credential copy, rotation, forwarding edit, and deletion keep current behavior.
- Each dialog supports keyboard access and focus restoration.
- Each status has text and a semantic color.
- Reduced-motion preferences remove nonessential motion.
- Playwright validates behavior and Pinguin-owned visual output.
- The final `make ci` command passes.

## Expected Files

The implementation will affect these current files:

- `web/index.html`.
- `web/event-log.html`.
- `web/smtp-relay.html`.
- `web/assets/css/base.css`.
- `web/js/app.js`.
- `web/js/constants.js`.
- `web/js/types.d.js`.
- `web/js/core/apiClient.js`.
- `web/js/ui/notificationsTable.js`.
- `web/js/ui/smtpIdentities.js`.
- `tests/e2e/landing.spec.ts`.
- `tests/e2e/dashboard.spec.ts`.
- `tests/e2e/profile-menu.spec.ts`.
- `README.md`.
- `ARCHITECTURE.md`.

The implementation will add the CSS and JavaScript modules that this document specifies.

## Authorization Boundary

The user explicitly authorized implementation on 2026-08-10.

The compact MPR interface, canonical module split, responsive acceptance coverage, accessibility dialogs, and visual baselines are complete. Publication, release, and deployment remain outside this implementation.
