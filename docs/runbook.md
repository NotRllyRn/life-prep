# Operations

Preparatory runbook, not activation authorization. **Manual verified receipts only: no automatic completion bridge exists.** Do not infer end-to-end readiness from the local fixture suite. Existing normally available Hermes harness sources remain approved for relevant read-only investigation without a per-source allowlist; new authentication, elevation, external mutation and undesignated notices remain prohibited without separate approval.

All commands take config path as second argument (demo instead takes a private directory).

- `serve`: initial/recovery sync, watch, pull receiver, hourly fallback/renew retry, daily briefing, artifacts and optional dispatch. Ctrl-C/SIGTERM stops it.
- `status` / `queue`: durable events and state; no misleading live health claim.
- `doctor`: local config/credential existence, no remote validation.
- `inspect CONFIG ID`: event state/artifact reference.
- `pause` / `resume`: durable dispatch pause, ingestion continues.
- `backfill`: explicit 21-day re-list for both accounts, idempotent queue; fails if daemon writer lock held.
- `watch`: register/renew both watches, never replace processed cursors.
- `retry CONFIG ID`: only definite failed dispatch, after fixing auth/config.
- `receipt CONFIG ID RESULT_FILE`: operator attests verified preparation outcome; copies nonempty human-readable result to private durable artifacts and records completed. Never use merely a webhook HTTP response as the result.

For uncertain/accepted events, inspect Hermes logs and actual artifacts/notices correlated by event ID. Reconcile first; there is deliberately no force replay flag. If no trustworthy outcome can be established, keep uncertain and investigate manually. No unattended bridge claims success. Save outcomes as human-readable result records with followup pointers and notice delivery evidence.

Before activation, define and capacity-test production API/model/tool budgets, bounded retries and runtime/resource limits, mailbox/resync memory, queue/artifact storage and overload handling. Define retention/deletion periods and backup/evidence lifecycle; these production controls are explicitly deferred and not automatically enforced.

Monitor service logs and queue backlog. Auth/config problems need manual repair; PubSub poison hints need subscription dead-letter policy review. Expired history recovery may be expensive for large mailboxes: current minimal implementation collects pages in memory before one atomic ingestion; capacity-test your mailbox before activation. Pending dispatch retries occur each minute only until an outcome is ambiguous. Watch/fallback errors retry hourly, pull receive failure retries in 30 seconds. Back up DB with SQLite online backup or stop service and copy DB/WAL together; do not copy only an active main DB. Preserve artifacts and protect private backups. Do not run from a public source tree with real credentials/state accidentally staged.
