# SMTP Delivery Plan

Pinguin delivers email notifications through `SMTPEmailSender`, a provider-agnostic implementation that speaks the SMTP protocol directly. This document captures the agreed plan for how the sender is configured, how it connects to remote servers, and how we will extend and test the functionality.

Pinguin can also expose an authenticated SMTP submission listener for Gmail Send-As. That listener does not deliver directly to recipient MX hosts; it validates the authenticated exact sender, preserves the raw submitted message, and relays it through the independent `smtpSubmission.relay` upstream profile.

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
- **smtpSubmission.relay** exposes the upstream SMTP account used only by the authenticated SMTP submission listener.
- **smtpSubmission.senderDomains** defines the global domain allowlist for exact sender identities.

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
