# Automatic notice debrief threads

## Architecture
Keep private preparation and public conversation separate. Extend the existing instance-local delivery wrapper; do not change Hermes core or add a thread manager. Only verified useful notices in the configured feed qualify. Existing exact starter-message bindings supply debrief context; the persistent Discord participation tracker supplies mention-free admission, subject to existing authorization and thread mention policy.

## Implementation sequence
1. After exact notice readback, durable context mapping, outcome validation and receipt, create one public message-started thread on the first delivered chunk. Derive a short single-line title from the verified outcome summary. Use Discord's existing message thread API and 24-hour archive duration.
2. Verify returned thread ID equals the starter ID and parent equals the approved feed, then persist participation through the adapter's existing tracker. Reuse an already-associated thread if available. Never post a second notice or auto-replay an uncertain delivery.
3. Isolate thread failures from successful notice delivery/completion. Log exact identifiers and leave the notice usable through explicit replies. Do not relax channel-wide mention requirements or existing user authorization.
4. Cover chunked delivery, single creation, duplicate completion, thread failure, identity mismatch, title sanitization, quiet/no-action and unrelated routing. Exercise the real installed Discord ingress predicate with its real persistent tracker; mock only network transport.
5. Run Python integration tests and Go tests; review the diff and commit only this feature. Preserve existing unrelated working-tree changes. Push and compare remote HEAD with the local commit.

## Side-effect and recovery contract
No-action, failed sends and unverified notices create no thread. Thread creation happens after the existing completion receipt so a thread error cannot duplicate already delivered preparation. There is no new retry queue: ambiguous creation is logged for reconciliation. Existing threads and notices are not migrated. Restart persistence is owned by Hermes's tracker and Life Prep's SQLite bindings.

## Deployment and verification
Code publication is not activation. The installed plugin must load the new code; permissions must allow public-thread creation and thread messages. A live notice followed by an unmentioned request remains the final deployment smoke test. Do not restart a gateway from its own child process or post synthetic production notices merely to test publication.
