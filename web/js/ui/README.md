# Front-end UI modules

UI modules own Alpine state for the semantic Pinguin work surfaces.

- `notificationsList.js` owns tenant selection, search, status filters, cursor loading, notification records, and reschedule/cancel dialogs.
- `smtpDomains.js` owns sender-domain creation, single-panel disclosure, DNS checks, and DNS copy feedback.
- `smtpIdentities.js` composes the SMTP workspace and owns identity creation, forwarding edits, rotation, and deletion.
- `smtpCredentialsDialog.js` owns credential-dialog state, copy feedback, and focus restoration.
- `toastCenter.js` renders secondary feedback from the shared DOM event contract.

The HTML templates remain declarative. Each user action enters through one Alpine method and each obsolete table or native-confirmation path is absent.
