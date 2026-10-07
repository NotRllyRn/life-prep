# New-only operation

Two optional top-level JSON booleans default to `false`:

- `NewOnly`: reject missing history cursors, expired Gmail history, and the
  explicit `backfill` command instead of downloading old messages. With `false`,
  existing initial and expired-history backfill behavior remains unchanged.
- `DailyBriefing`: generate daily briefing events in `serve` only when `true`.
  `false` prevents new briefing generation; it does not remove existing events.

For new-mail-only operation, set `NewOnly: true` and `DailyBriefing: false` in
an operator-managed config. This document does not activate integrations.

Before serving, initialize both configured accounts:

```sh
bin/life-prep seed /path/to/config.json
```

`seed` holds the mailbox writer lock, requires exactly two unique account IDs,
and reads only each account's **current Gmail profile history ID**. It neither
lists nor processes mail, starts watches, generates artifacts, nor dispatches
Hermes work. It works with `Enabled: false` and does not require a Hermes runtime
secret. Gmail OAuth credentials/token files are still required. Both baselines
are committed atomically after both profile reads succeed.

Any existing event (including completed events or prior briefings) causes seed
to refuse **before** calling Gmail, preventing old pending work from being
mistaken for a clean start. Use a deliberately fresh database/artifacts path;
do not delete existing state without reviewing it. An event-free database may
be reseeded, intentionally skipping changes since its previous baseline.

Only after reviewing configuration and activation approvals, an operator may
set `Enabled: true` and run the normal `serve` command. Expired history in
new-only mode is an error, not an implicit backfill: preserve/reconcile existing
work and seed a fresh event-free database if choosing to skip the missed mail.
Startup receive logs identify account ID and subscription; successful initial
and periodic watch renewals identify account ID. No tokens, credentials, or
notification payloads are added to these logs.
