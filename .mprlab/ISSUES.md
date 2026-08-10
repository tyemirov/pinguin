# ISSUES

Entries record newly discovered requests or changes.

## BugFixes

## Improvements

## Maintenance

## Features

## Planning

- [ ] [P001] (P1) Define managed tenant configuration
  Goal:
  Define the current contract that lets a TAuth user manage Pinguin tenants without tenant YAML.
  Requirements:
  - Keep the current TAuth and `mpr-ui` authentication integration.
  - Define owner-scoped tenant management and tenant-bound programmatic access.
  - Define API secret storage that prevents secret recovery from Pinguin data.
  - Define one forward-only production data conversion.
  - Record unsupported product choices as open decisions.
  Deliverables:
  - Maintain the durable [managed tenant configuration plan](../docs/multitenancy-plan.md).
  - Record the confirmed architecture, implementation order, acceptance criteria, and open decisions.
  Validation:
  - Examine the plan against current configuration, HTTP, gRPC, database, browser, and deployment contracts.
  - Confirm that the plan adds no Pinguin authentication flow.
  - Confirm that the plan contains no compatibility runtime path.
