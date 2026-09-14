# Front-end core modules

Core modules own browser boundaries and shared application behavior.

- `apiClient.js` is the only Pinguin API transport boundary. It maps current JSON payloads into validated browser records.
- `events.js` defines the DOM-scoped refresh and toast event contracts.
- `sessionBridge.js` is the only bridge from `mpr-ui` authentication events and profile snapshots into Alpine authentication state.

UI modules consume these contracts. They do not call `fetch` or interpret shared-shell DOM state directly.
