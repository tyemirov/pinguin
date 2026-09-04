# Health endpoints

Unauthenticated `GET /healthz` checks the API datastore with a read-only query and a one-second deadline.
It returns minimal `200` or `503` JSON with `Cache-Control: no-store`.
Successful probes produce no request events. Failed probes retain diagnostic events.
Probes do not resolve a tenant, send notifications, or change application data.

The website publishes `web/healthz`. Local serving adds `Cache-Control: no-store`.
GitHub Pages controls production response headers and does not expose a repository header setting.
I003 retains the production cache requirement as an open hosting decision.

Docker startup probes run every second for 30 seconds. Later probes run every 30 seconds.
The gRPC and SMTP checks keep their existing protocol contracts.
