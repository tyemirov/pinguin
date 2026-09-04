# Health endpoints

Unauthenticated `GET /healthz` checks the API datastore with a read-only query and a one-second deadline.
It returns minimal `200` or `503` JSON with `Cache-Control: no-store`.
Successful probes produce no request events. Failed probes retain diagnostic events.
Probes do not resolve a tenant, send notifications, or change application data.

The website publishes `web/healthz`. Local serving adds `Cache-Control: no-store`.
GitHub Pages uses its production cache policy for the static health resource.
The operator approved this exception on 2026-09-04. API and local health
responses still require `Cache-Control: no-store`.
A cached Pages response proves artifact availability, not current API readiness.

Docker startup probes run every second for 30 seconds. Later probes run every 30 seconds.
The gRPC and SMTP checks keep their existing protocol contracts.
