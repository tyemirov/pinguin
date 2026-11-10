# Delivery behavior

Pingwin has three delivery paths: gRPC notifications, authenticated SMTP submission, and inbound SMTP forwarding.
Their storage and retry behavior differ.

## Notification delivery

A tenant email profile supplies SMTP host, port, username, password, and from address.
The notification service resolves the tenant profile from SQLite before dispatch.
An optional tenant Twilio profile supplies SMS account credentials and a from number.
Shared SMTP listener profiles do not replace the tenant notification profile.

The gRPC boundary validates the request and constructs the domain request.
A future schedule produces a persisted `queued` record.
An immediate request attempts delivery before database insertion.
Successful provider handoff produces `sent`.
An initial provider error produces an `errored` record when persistence succeeds.
A database failure can occur after provider acceptance.

The worker selects due `queued` and `errored` records below the configured retry limit.
The worker uses `github.com/tyemirov/utils/scheduler` through repository and dispatcher adapters.
It uses exponential backoff and resolves current tenant credentials for each retry.
Status `errored` can be transient or exhausted.
The retry count and configured limit distinguish those conditions.
Neither `sent` nor the provider response establishes final receipt.

SQLite stores notification metadata, recipients, subjects, message text, schedules, retry state, and attachment bytes.
Provider secrets and SMTP identity passwords have encryption at rest.
That protection does not encrypt notification content or attachment bytes in the application schema.
Ambiguous provider responses or a client retry after a failed database write can produce duplicate delivery.
The service does not promise exactly-once delivery.

## Email composition and transport

`SMTPEmailSender` implements the SMTP provider boundary.
Its config contains the SMTP target, credentials, from address, and timeout inputs.
The service creates a sender from the current tenant email profile.

`buildEmailMessage` creates a MIME message with `From`, `To`, `Subject`, and body content.
Attachments produce `multipart/mixed` messages with base64 data and `Content-Disposition` metadata.
Email permits 10 attachments, 5 MiB per attachment, and 25 MiB combined attachment bytes.
Validation occurs at the request boundary.
Persisted attachment bytes are reused for scheduled sends and retries.

Port `465` uses an implicit TLS connection through `tls.DialWithDialer`.
That path respects `CONNECTION_TIMEOUT_SEC` for connection establishment.
Its current TLS config sets `InsecureSkipVerify: true`.
Certificate verification on this path remains an implementation limitation.

Other ports use `smtp.SendMail`.
That helper negotiates STARTTLS when the server advertises support.
This path does not establish a universal enforced-TLS policy.
`OPERATION_TIMEOUT_SEC` does not impose a full SMTP operation deadline on this helper path.
Caller context alone does not bound that helper's I/O.

SMTP authentication uses `smtp.PlainAuth` with the tenant username, password, and host.
Delivery issues `MAIL FROM`, `RCPT TO`, and `DATA` in sequence.
The writer closes to finish the data transaction.
Errors return to the service and worker for the applicable failure behavior.
The SMTP client or TLS connection closes after the operation.

## Authenticated SMTP submission

The optional submission listener authenticates an exact SMTP identity under a verified tenant sender domain.
The login, envelope sender, and RFC 5322 `From` header must match that identity.
The listener preserves the raw submitted message.
Delivery completes synchronously before the final SMTP success response.
This path has no persisted notification retry record.

`smtpSubmission.deliveryMode` selects `upstream` or `direct`.
Upstream mode uses the independent `smtpSubmission.relay` profile.
Direct mode permits one recipient domain per message.
It resolves that domain's MX hosts and connects on port `25`.

When no MX record exists, SMTP domain-host resolution applies.
A null MX record is rejected.
The authenticated identity remains the envelope sender.

Direct mode applies the operation budget to DNS lookup and SMTP I/O.
It uses STARTTLS when the remote server advertises support.
A direct sender domain needs SPF authorization for the gateway address to support DMARC alignment.
Pingwin supplies no DKIM signer, bounce processor, or mailbox service.

`smtpSubmission.publicPort` and `publicSecurityMode` describe the external client settings.
Production declares SMTPS on `pingwin-api.mprlab.com:465`.
Caddy terminates TLS and forwards to Pingwin's private plaintext port `587`.
The [SMTP setup guide](smtp-setup.md) defines DNS, identities, listener limits, and Gmail settings.

## Inbound forwarding

The optional MX listener accepts only active SMTP identities with forwarding recipients.
It accepts `MAIL FROM:<>` for DSNs and other messages with a null reverse path.
Unknown recipients are rejected before `DATA`.
Message-size and recipient limits apply.

Each accepted route forwards through `smtpForwarding.relay`.
For parsed messages, the relay rewrites `From` to the shared address and preserves the body bytes.
It keeps an existing `Reply-To`, or uses the original `From` address for replies.
It records the original `From` in `X-Pingwin-Original-From` when present.

The rewrite removes previous DKIM, DomainKey, ARC, and authentication-result headers.
It also removes `Bcc`, `Return-Path`, and `Sender`.
If header rewriting fails, the current code logs `smtp_forwarding_rewrite_skipped` and relays the original message bytes.

The shared address becomes the outbound envelope sender.
The relay profile is independent of tenant notification and SMTP submission profiles.
Forwarding stores no message body, mailbox state, search index, read state, or retention data.
It provides no IMAP or POP3 access.

The listener waits for forwarding before its final success response.
A route or relay failure returns a temporary SMTP failure when acceptance is incomplete.
The sender's mail server can then retry.
Partial fanout or uncertain SMTP responses can produce duplicate copies.

Customer MX records normally use a dedicated mail subdomain, such as `help.example.com`.
Production declares `mx.pingwin.mprlab.com:25` through the Gateway listener.
The [SMTP setup procedure](smtp-setup.md#inbound-smtp-forwarding-for-shared-addresses) includes the exact DNS examples.

## Validation scope

The repository contains SMTP protocol tests and public-entry-point integration tests.
The scheduled email integration test exercises worker dispatch through an injected recording sender.
Separate protocol tests exercise SMTP exchanges against local test servers.
Managed tenant tests exercise notification isolation and tenant HTTP operations.
Direct relay tests exercise recipient-MX delivery, the one-domain restriction, DNS deadlines, and SMTP failures.

Browser tests use local API and authentication boundaries.
Live provider connectivity and hosted authentication require their separate acceptance procedures.

## Future enhancements

The former SMTP plan identified enforced STARTTLS, per-request from-address overrides, and more SMTP-stage diagnostics as future work.
Those items remain proposals, rather than current capabilities or committed release scope.
The current tenant profile controls the notification from address.
