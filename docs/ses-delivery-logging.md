# SES delivery logging

Pingwin records SES delivery events for short-term operational checks.
The configuration applies to every recipient sent through its configured SES identities, including forwarding recipients.
It has no recipient-specific rules.

## Pingwin UI and runtime activation

The source includes an authenticated **Email delivery** workspace at `/email-delivery.html`.
Select a tenant and search by subject, sender, recipient, or SES message ID.
The view loads and updates automatically.
Compact rows show the time, subject, sender-to-recipient route, recipient outcome counts, and forwarding state.
Expand a row for individual recipient responses, message IDs, expiry, and observed forwarding links.
`Unconfirmed` means no conclusive outcome is available.
An accepted delivery remains recorded separately if a later complaint arrives.

History is limited to SES mail from the selected tenant's currently verified, owned sender domains.
An editable notification-profile From address alone never grants access to history.
An exact source-domain match is required; sibling domains and subdomains are separate scopes.
Messages predating the sender-domain registration are excluded.
Registration time is a historical access floor, not proof of when DNS ownership first began.

The API enforces a moving 24-hour window for both message and event timestamps, even when CloudWatch has not physically deleted an expired record.
The page removes expired entries while open and clears its mail data on tenant changes or sign-out.
Pingwin adds no delivery-history database, disk cache, or browser storage.
Ordinary persisted notification records retain their existing lifecycle; this window applies to provider delivery history.
Other SMTP providers and direct MX delivery need their own telemetry and do not appear as confirmed SES outcomes.

The UI source and runtime wiring require deployment before they become available on the hosted service.
The existing SES logging resources below are already active.
Production UI activation requires a separate dedicated AWS credential with only this permission:

```json
{
  "Version": "2012-10-17",
  "Statement": [{
    "Effect": "Allow",
    "Action": "logs:FilterLogEvents",
    "Resource": "arn:aws:logs:us-east-1:299335570791:log-group:/aws/events/pingwin-ses-delivery:*"
  }]
}
```

Keep that credential separate from SMTP sending credentials.
The existing SMTP principals have no log-reading permission.
Put these explicitly declared inputs in the manifest-selected private source before the next production deployment:

```dotenv
EMAIL_DELIVERY_ENABLED=true
EMAIL_DELIVERY_ACCESS_KEY_ID=<dedicated-reader-access-key-id>
EMAIL_DELIVERY_SECRET_ACCESS_KEY=<dedicated-reader-secret-access-key>
```

The production YAML selects the account, region, log group, and configuration set recorded below.
For a separate local runtime, configure the optional YAML section explicitly:

```yaml
emailDelivery:
  enabled: true
  region: us-east-1
  accountId: "299335570791"
  logGroup: /aws/events/pingwin-ses-delivery
  configurationSet: pingwin-delivery-events
  accessKeyId: ${EMAIL_DELIVERY_ACCESS_KEY_ID}
  secretAccessKey: ${EMAIL_DELIVERY_SECRET_ACCESS_KEY}
```

Keep mail history out of diagnostic exports and durable application logs.
After authorized deployment, sign in, select a tenant with a verified domain, and confirm a recent simulator send and a controlled forwarding chain in **Email delivery**.
Source tests and local browser acceptance do not establish hosted activation.

## Active configuration

Activated November 18, 2026, in AWS account `299335570791`, region `us-east-1`:

| Resource | Name or setting |
| --- | --- |
| SES configuration set | `pingwin-delivery-events` |
| SES event destination | `delivery-events`, enabled, EventBridge default bus |
| EventBridge rule | `pingwin-ses-delivery` |
| CloudWatch Logs group | `/aws/events/pingwin-ses-delivery` |
| Log retention | `retentionInDays: 1` |
| EventBridge target retries | At most 10 retries; maximum event age 3,600 seconds |
| Logs resource policy | `PingwinSesDeliveryEvents` |

The configuration set is the default for `poodlescanner.com`, `pingwin.mprlab.com`, and `kamutales.com`.
These were all SES identities in the account at activation; there were no separately verified email identities.
Domain defaults cover addresses that inherit that domain's verification.

The event destination publishes `SEND`, `DELIVERY`, `DELIVERY_DELAY`, `BOUNCE`, `COMPLAINT`, `REJECT`, and `RENDERING_FAILURE`.
The EventBridge rule matches `aws.ses`, the account and region above, and the `ses:configuration-set` tag equal to `pingwin-delivery-events`.
It sends the complete SES event metadata and headers to the log group.
SES events contain no message body. Open and click tracking are not enabled.

The resource policy permits only `logs:CreateLogStream` and `logs:PutLogEvents`, for `events.amazonaws.com` and `delivery.logs.amazonaws.com`, against this log group's ARN and streams.
The target has no execution role.
The existing SMTP policies for `pingwin-runtime-smtp` and `pinguin-poodlescanner-smtp` include the exact configuration-set ARN for `ses:SendRawEmail`.
Their existing sender-address conditions remain in effect.

At activation, the default EventBridge bus had no other rules or archives.
CloudWatch Logs had no account-level retention policy.
Check additional consumers and retention overrides before changing this routing.

## Retention and delivery evidence

CloudWatch Logs expires events after one day.
Physical deletion typically takes up to another 72 hours and can take longer.
This is a one-day retention policy, not a guarantee of physical deletion at exactly 24 hours.
See [AWS retention behavior](https://docs.aws.amazon.com/AmazonCloudWatchLogs/latest/APIReference/API_PutRetentionPolicy.html).

`Delivery` confirms that the destination mail server accepted the listed recipients.
It does not establish inbox placement or whether a person read the message.
`Send` confirms SES accepted the sending request; inspect a later delivery, delay, bounce, or rejection event for its outcome.
SES sends EventBridge events on a best-effort basis and events can arrive out of order.
An absent event leaves delivery unconfirmed.
See [SES event semantics](https://docs.aws.amazon.com/ses/latest/dg/monitoring-eventbridge.html).

Logging begins at activation. It cannot recover receipts for earlier emails.
These records cover SES delivery attempts; a message rejected before SES accepts it requires the originating client or Pingwin error record.

## Check a delivery or forwarding chain

Open [CloudWatch Logs Insights in us-east-1](https://us-east-1.console.aws.amazon.com/cloudwatch/home?region=us-east-1#logsV2:logs-insights).
Select `/aws/events/pingwin-ses-delivery` and a time range within the last day.
Replace the example address with the recipient under investigation:

```text
fields @timestamp, detail.eventType, detail.mail.messageId,
       detail.mail.source, detail.mail.destination,
       detail.delivery.recipients, detail.delivery.smtpResponse,
       detail.bounce.bouncedRecipients
| filter @message like /recipient@example\.com/
| sort @timestamp desc
| limit 200
```

For a particular message, replace the address filter with its SES message ID or subject.
Inspect recipient-specific delivery and bounce fields when a message has multiple recipients.

For forwarding, locate the incoming message's delivery to the Pingwin identity.
Then search its SES message ID in the full event text.
When present, the forwarded event's `detail.mail.headers` contains that incoming ID in its `Message-ID` header, while `detail.mail.messageId` identifies the new forwarding attempt.
The activation test confirmed this relationship for the existing forwarding path.
Missing or truncated headers leave ID-based correlation unconfirmed; inspect `detail.mail.headersTruncated`.
Use the new ID to check delivery to every forwarding destination.
For mail arriving from another provider, begin with its RFC `Message-ID`; the incoming leg has no SES sending event in this account.
Subject, sender, and time can help locate candidates, but the IDs establish the observed chain.

An AWS CLI session in the same account can also retrieve recent records:

```bash
aws logs filter-log-events \
  --region us-east-1 \
  --log-group-name /aws/events/pingwin-ses-delivery \
  --start-time <UTC-epoch-milliseconds> \
  --filter-pattern '"<SES-message-ID>"' \
  --no-cli-pager
```

## New identities and credentials

Assign `pingwin-delivery-events` as the default for each new SES domain identity.
Assign it explicitly to separately verified email identities because their settings can override domain defaults.
Permit the exact configuration-set ARN in each sending principal's SES policy while retaining its sender restrictions:

```text
arn:aws:ses:us-east-1:299335570791:configuration-set/pingwin-delivery-events
```

An explicitly supplied configuration set overrides the identity default.
Keep application sending configured for this set, either through the identity default or an explicit selection.
Repeat a live delivery check after changing identity defaults or SMTP credentials.
See [SES default configuration sets](https://docs.aws.amazon.com/ses/latest/dg/managing-configuration-sets.html).

## Activation acceptance

Checks used the configured shared SES SMTP credential without a configuration-set header, proving automatic domain defaults.
The test subject prefix was `pingwin-delivery-check-20261118T212230Z`.

| Check | Observed result |
| --- | --- |
| Success simulator from each of the three domains | `Send` and `Delivery`; SMTP `250 Ok` |
| Bounce simulator | `Bounce`, permanent failure, status `5.1.1`, SMTP `550` |
| Delivery to `support@pingwin.mprlab.com` | `250 Message accepted for forwarding` at `2026-11-18T21:22:35.819Z` |
| Forwarding to the configured operator Gmail destination | `Delivery`, Gmail SMTP `250 2.0.0 OK` at `2026-11-18T21:22:36.337Z` |

The original forwarding-test SES message ID was `010001a1f08a3ba6-d903635a-d8bc-4a12-8ca6-7cf18af4e552-000000`.
The forwarded SES message ID was `010001a1f08a43b5-c559e5df-37b7-4e3d-92ad-ad6cbdf8e2de-000000`.
The forwarded event retained the original ID in its raw `Message-ID` header.
The simulator bounce ID was `010001a1f08a3a06-9154cc59-6488-4073-a393-86a01a8b473d-000000`.

IAM simulation allowed the intended sender and denied `outside@example.com` for each updated SMTP principal.
The shared SMTP principal was exercised live; the tenant-specific PoodleScanner principal was validated through IAM simulation.
These checks establish SES delivery logging and the existing public forwarding path.
No application deployment was required.
