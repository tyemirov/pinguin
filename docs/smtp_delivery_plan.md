# SMTP Delivery Plan

Pinguin delivers email notifications through `SMTPEmailSender`, a provider-agnostic implementation that speaks the SMTP protocol directly. This document captures the agreed plan for how the sender is configured, how it connects to remote servers, and how we will extend and test the functionality.

Pinguin can also expose an authenticated SMTP submission listener for Gmail Send-As. That listener validates the authenticated exact sender, preserves the raw submitted message, and either relays it through the independent `smtpSubmission.relay` upstream profile or delivers it directly to recipient-domain MX hosts.

Pinguin can separately expose an inbound SMTP forwarding listener for shared addresses. That listener is MX-facing, unauthenticated, accepts only active SMTP identities with forwarding owners, forwards the raw message immediately through `smtpForwarding.relay`, and stores no mailbox state.

## Components

- **SMTPEmailSender** wraps all SMTP interactions. It accepts an `SMTPConfig` that contains the target host, port, credentials, `From` address, and timeout budget.
- **NotificationService** provisions a new `SMTPEmailSender` for each service instance using the values supplied through `config.Config`.
- **config.Config** exposes the following SMTP-specific settings:
  - `SMTP_HOST`
  - `SMTP_PORT`
  - `SMTP_USERNAME`
  - `SMTP_PASSWORD`
  - `FROM_EMAIL`
  - `CONNECTION_TIMEOUT_SEC`
  - `OPERATION_TIMEOUT_SEC`
- **smtpSubmission.deliveryMode** selects `upstream` provider relay or `direct` recipient-MX delivery for authenticated SMTP submissions.
- **smtpSubmission.relay** exposes the upstream SMTP account used only when `smtpSubmission.deliveryMode` is `upstream`.
- **smtpSubmission.senderDomains** defines the global domain allowlist for exact sender identities and shared-address forwarding routes.
- **smtpSubmission.publicPort** and **smtpSubmission.publicSecurityMode** control the Gmail-facing settings shown with one-time SMTP identity credentials. These can differ from the private listener when Caddy owns public TLS termination.
- **SMTP identities** map exact shared addresses such as `support@help.example.com` to one or more persisted forwarding recipients.
- **smtpForwarding.relay** exposes the outbound SMTP account used to deliver forwarded copies.

## Delivery Sequence

1. **Input validation** happens in `NotificationService` before dispatch. Requests missing a recipient or message are rejected immediately.
2. **Message composition** uses `buildEmailMessage` to generate a MIME-compliant payload containing the headers (`From`, `To`, `Subject`) and body. When attachments are present, the helper emits a `multipart/mixed` body, base64-encodes each attachment, and adds `Content-Disposition` metadata so SMTP relays understand filenames and MIME types.
3. **Connection setup** selects the transport based on the configured port:
   - Port `465` triggers an implicit TLS connection with `tls.DialWithDialer`. The dialer respects `CONNECTION_TIMEOUT_SEC`, and the connection is established before issuing SMTP commands.
   - Any other port uses the standard `smtp.SendMail` helper, which negotiates STARTTLS when the server advertises support.
4. **Authentication** relies on `smtp.PlainAuth`, passing through the configured username and password. The host component from `SMTP_HOST` is used for the authentication scope.
5. **Envelope commands** (`MAIL FROM`, `RCPT TO`, `DATA`) are issued sequentially. The implementation writes the composed message bytes to the SMTP data stream and closes the writer to finalize the transaction.
6. **Error handling** wraps failures with context using `%w` so callers receive actionable diagnostics (e.g., connect failures, auth failures, write failures). Failures propagate back to the notification worker so they can trigger retries.
7. **Cleanup** always closes the SMTP client or TLS connection to free sockets quickly.

For authenticated SMTP submission in direct mode, Pinguin groups recipients by domain, resolves MX records, falls back to the domain host when no MX is present, connects to each target on port `25`, uses the authenticated identity as `MAIL FROM`, and writes the exact submitted RFC 5322 payload through SMTP `DATA`. Domains using this mode must publish SPF records that authorize the gateway IP so DMARC can align with the RFC 5322 `From` domain.

For inbound forwarding, customer DNS should normally point a dedicated mail subdomain at `smtp.pinguin.mprlab.com`:

```dns
help.example.com. MX 10 smtp.pinguin.mprlab.com.
```

Pinguin accepts `MAIL FROM:<>` null reverse-path traffic for DSNs and other auto-generated mail, rejects unknown `RCPT TO` addresses before `DATA`, enforces configured size and recipient limits, and returns a temporary SMTP failure when route lookup or forwarding through `smtpForwarding.relay` fails before acceptance. Forwarded copies preserve the original message headers and use the shared address as the outbound SMTP envelope sender. Because Pinguin stores no message body, operators should treat forwarding retries and duplicate delivery risk as part of the no-mailbox tradeoff.

## Timeout Strategy

- `CONNECTION_TIMEOUT_SEC` bounds how long we wait to establish TCP/TLS connections.
- `OPERATION_TIMEOUT_SEC` is reserved for future I/O deadlines; until then we rely on context cancellation supplied by the caller.
- The background worker respects the same configuration when retrying emails.

## Attachment Limits

- Each email may include up to **10 attachments**.
- Individual attachments are capped at **5 MiB**, and the combined payload must remain under **25 MiB**.
- Attachments are validated at the service edge and persisted in a dedicated table so retries use the original bytes.

## Testing Strategy

- **Unit tests** validate that `NotificationService` wires the SMTP sender with the exact configuration values (added in `notification_service_email_sender_test.go`) and that `buildEmailMessage` produces correct MIME boundaries for attachments.
- **Integration tests** (future work) should use a fake SMTP server to assert protocol exchanges without reaching the public internet.

## Future Enhancements

- Support STARTTLS enforcement by checking the server extension list and failing when encryption is required but unavailable.
- Expose optional per-request overrides for the `From` address when business rules require branding-specific senders.
- Add structured logging around each SMTP stage so operators can diagnose delivery issues without enabling verbose debugging.
