# ISSUES

Entries record newly discovered requests or changes.

## BugFixes

- [x] [B001] (P1) Package the production tenant conversion command
  Goal:
  The published image contains the command for the production data conversion.
  Validation:
  - Verify the image build contract.
  - Verify the production volume command in the conversion runbook.

- [x] [B002] (P1) Keep a credential for an uncertain write result
  Goal:
  The tenant workspace can retry the same credential write after an uncertain response.
  Validation:
  - Verify that a create retry uses the same credential and idempotency key.
  - Verify that a rotation retry uses the same credential and version.

- [x] [B003] (P1) Clear tenant workspace data after logout
  Goal:
  The tenant workspace contains no tenant data or credential data after logout.
  Validation:
  - Verify the forms, dialogs, tenant list, and temporary credential state after logout.

- [x] [B004] (P1) Remove a disabled SMS profile
  Goal:
  The tenant update removes the SMS profile when the user disables SMS delivery.
  Validation:
  - Verify the SMS profile deletion API.
  - Verify the tenant update in the browser.

- [x] [B005] (P1) Report credential storage errors as internal errors
  Goal:
  The gRPC API reports a storage fault separately from an invalid credential.
  Validation:
  - Verify an `Internal` result for a credential storage fault.
  - Verify an `Unauthenticated` result for an invalid credential.

## Improvements

## Maintenance

## Features

- [!] [F001] (P1) {P002} Implement managed tenant configuration
  Goal:
  Let a TAuth user manage Pinguin tenants and tenant API access without tenant YAML.
  Requirements:
  - Keep the current TAuth and `mpr-ui` authentication integration.
  - Derive each tenant owner from the validated TAuth user ID.
  - Store tenant configuration in the Pinguin database.
  - Require one complete external email delivery profile and one API credential during tenant creation.
  - Store only the API credential digest.
  - Provide permanent tenant deletion that removes all tenant-owned records.
  - Make SMTP sender domains, identities, credentials, and forwarding routes tenant-owned.
  - Remove tenant YAML, the global gRPC token, and gRPC caller-selected tenant IDs.
  - Keep one forward-only runtime contract.
  Deliverables:
  - Implement the [managed tenant configuration plan](../docs/multitenancy-plan.md).
  - Add the managed HTTP API, browser interface, gRPC authentication, and client contract.
  - Add tenant-owned SMTP management under tenant resource routes.
  - Add the exact managed schema and the bounded production data conversion command.
  - Assign all current production tenants to the TAuth account for `temirov@gmail.com`.
  - Remove the conversion command after user-owned production acceptance.
  Validation:
  - Pass all acceptance criteria in the managed tenant configuration plan.
  - Validate the conversion with production-shape fixtures.
  - Pass the complete `make ci` target.
  Blocked:
  - The production operator must confirm the TAuth user ID for `temirov@gmail.com`.
  - The production operator must run the conversion and accept the managed runtime.
  - After acceptance, remove the bounded conversion command and archive F001.

## Planning
