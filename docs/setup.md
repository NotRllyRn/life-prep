# Research-backed setup (not executed)

Primary sources reviewed:
- https://developers.google.com/workspace/gmail/api/guides/push
- https://developers.google.com/workspace/gmail/api/guides/sync
- https://developers.google.com/workspace/gmail/api/reference/rest/v1/users/watch
- https://docs.cloud.google.com/pubsub/docs/pull-messages
- https://docs.cloud.google.com/go/docs/reference/cloud.google.com/go/pubsub/v2/latest
- https://hermes-agent.nousresearch.com/docs/user-guide/messaging/webhooks

Gmail watch supports gmail.readonly. Request only that scope using an independently consented OAuth token per mailbox; a scope requested during refresh does not narrow an already overprivileged grant. Verify grants separately. Use a standard installed-app client JSON and OAuth token JSON with refresh_token obtained through an approved Google OAuth flow. No interactive authorization helper is bundled. Refreshes occur in memory; the token file is not rewritten. Review OAuth app verification and testing-mode token expiry before relying on unattended operation.

Enable Gmail/PubSub APIs in the OAuth project's Cloud project. Operator creates one topic and pull subscription per account; topic project must match the watch caller's project. Grant Gmail's documented publisher service principal topic publish rights, and give the subscriber credential only consume access to designated subscriptions. Avoid broad owner/editor roles. No public listener or push endpoint is required. Subscription messages are decoded by the official Go SDK; notification data is a mailbox/history hint, not mail content. Account Email must privately match the actual mailbox. Malformed or mismatched hints NACK and need operator/DLQ investigation.

Copy config.example.json to a private config path, replace placeholders, provide both token/client files and subscriber credentials, and make state dirs accessible only to the service user. Use absolute database/artifact paths for shared Hermes access. Leave Enabled and Hermes.Enabled false until approved. `doctor` checks local files, not live permissions or token scopes. Initial serve does live 21-day sync and watch; it is an activation action. Gmail watch is renewed daily (maximum seven-day validity); failures retry hourly. Hourly history reconciliation covers dropped hints. SDK handles transport retries; receiver restart retries after 30 seconds; failed synchronization never advances the cursor.

Before Hermes activation: validate exact installed version, provision a loopback-only signed route manually, install the charter only with approval, validate background tool approvals and operation-specific permissions in a harmless test, constrain credentials and filesystem/egress access, and configure designated readiness/urgent/08:00 briefing Discord destinations privately. See hermes.md. No Discord destination is supplied or implicitly created. The daemon does not classify urgency or send notices; the approved Hermes route must do contextual preparation and designated notice delivery. Notice success and agent completion require verified receipts. Automatic receipt bridge is deliberately not fabricated: operator-assisted receipt CLI is supplied until an approved reliable bridge is exercised.

The service template is an example, not installed. Create the unprivileged user and private paths, place binary/config/secrets at the indicated locations, explicitly approve startup. Hermes must read artifacts under the shared approved path. Never expose the webhook beyond loopback without a separately reviewed network/security design.
