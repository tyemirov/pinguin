# ISSUES Archive

This file contains resolved non-recurring issue history.

The active [ISSUES.md](ISSUES.md) file contains current work.

## Planning

- [x] [P001] (P1) Define managed tenant configuration
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
  Resolved 2026-08-11:
  - Verified the plan against the current configuration, HTTP, gRPC, database, browser, and schema version 4 deployment contracts.
  - Replaced the action route with one notification resource patch contract.
  - Added protobuf reservation, exact schema validation, and UUID data conversion requirements.
  - Recorded P002 for product decisions and F001 for implementation.

- [x] [P002] (P1) Select managed tenant product contracts
  Goal:
  Select the remaining product contracts for managed tenant implementation.
  Requirements:
  - Select the required tenant state at creation.
  - Select suspension, deletion, or both tenant lifecycle operations.
  - Select one credential or multiple named credentials for each tenant.
  - Select tenant ownership or TAuth user ownership for SMTP resources.
  - Define retention and deletion behavior for the selected lifecycle.
  Deliverables:
  - Record one canonical selection for each open decision in the [managed tenant configuration plan](../docs/multitenancy-plan.md).
  - Remove the corresponding open-decision text after each selection.
  - Complete all selections.
  - Change F001 from blocked to open.
  Validation:
  - Confirm that each selection has one implementation path.
  - Confirm that the selections keep TAuth as the authentication service.
  - Confirm that the selections add no compatibility path.
  Resolved 2026-08-11:
  - Tenant creation requires one complete external email delivery profile and one API credential ID and digest.
  - The tenant lifecycle has permanent deletion and no suspension.
  - Each tenant has exactly one API credential with atomic rotation.
  - Each SMTP sender domain, identity, credential, and forwarding route belongs to one tenant.
  - Shared SMTP listeners and upstream relays remain service configuration.
  - F001 changed from blocked to open.
