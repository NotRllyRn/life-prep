# Preparation completion bridge

## Contract and deployment

The existing instance-local webhook send wrapper now verifies an event-specific `outcome.json` plus separate nonempty preparation before recording a prepared completion. Fields: `event_id`, absolute validated `source_ref`, `status` (`prepared` / `no_action`), and concrete `summary`. The notice must contain that summary verbatim, the exact event ID and an absolute prepared path. Discord readback must have the expected channel, bot author and exact concatenated message text. Transformed or differently chunked text fails closed rather than completing on an unrelated bot message.

The bridge writes private receipt evidence and invokes the existing Go `receipt` CLI, then reads back the completed row and exact copied evidence. Install the built binary at `<private-root>/life-prep`; configuration remains `<private-root>/config.json`. No new queue, framework or upstream modifications. Completion checks plus event-local async serialization suppress sequential/concurrent duplicate delivery after verified completion, including across bridge recreation. This is not an exactly-once distributed transaction: a process crash after Discord delivery but before receipt remains a reconciliation case, never grounds for automatic replay.

A legitimate no-action message writes the same provenance record with `status=no_action` and its source-specific reason, then returns `[SILENT]`. The route-local wrapper uses Hermes' existing autonomous silence matcher and returns successful no-delivery even when evidence is missing; missing evidence is logged locally and remains accepted, not a user-facing retry error. Quiet completion has an empty notice list because no notice is intended. No global response-filter changes.

Production prompt: `integrations/hermes/production-prompt.txt`. Synthetic events are skipped by ordinary dispatch; deterministic probes remain explicitly opt-in (`synthetic_probes: true`) and default off. They do not read mail. The default profile plugin and static route prompt are deployed on disk, but no gateway restart or live request is authorized. Do not assume a running gateway has reloaded a static route prompt merely because disk validation passed.

## Quota noise limitation

Inspected installed Hermes `gateway/run_turn_runner.py::_notice_callback_sync`, `gateway/run_notifications.py::_deliver_platform_notice`, and the webhook adapter send path. Out-of-band credits/operational notices reach `adapter.send` without a typed quota-notice marker. `notice_delivery` is platform-level public/private (not a route-local off switch); diagnostic notification filtering likewise is not an exact life-prep route switch. No proven route-local quota mechanism was found. No speculative keys, content-substring suppression, runner monkeypatch or global changes were introduced. Quota chatter may still appear until a typed route-scoped notice seam exists. It cannot receipt a preparation unless it also matches the explicit outcome/event/path contract. Actual quota exhaustion affecting preparation is still a genuine failure requiring reconciliation.

## Backlog reconciliation (do not replay)

The original 18 accepted real events and 2 completed synthetic events are unchanged. Old test checklists are not evidence of useful production completion. Keep intake disabled. Operator review should correlate each exact event/source pointer with existing prepared artifacts, outcome relevance, and exact Discord notice identity/content. Do not choose latest or bulk retry. Where genuine prior evidence is complete, create/verify the missing outcome and receipt once through the existing CLI after explicit review. For irrelevant mail, record a source-specific no-action decision quietly. Otherwise prepare that exact event with explicit authorization and avoid duplicate notices; do not acknowledge accepted events merely to clear the queue. Preserve uncertain deliveries for manual evidence-led reconciliation.

## Offline regression command

Build a local test binary, then run against the installed Hermes Python/source:

    go build -o /home/hermes/.hermes/cache/scratch/life-prep-offline ./cmd/life-prep
    PYTHONPATH=/home/hermes/.hermes/hermes-agent LIFE_PREP_TEST_BINARY=/home/hermes/.hermes/cache/scratch/life-prep-offline /home/hermes/.hermes/hermes-agent/venv/bin/python -m unittest discover -s integrations/hermes -p 'test_*.py' -v
    go test -count=1 ./...
    go vet ./...

The completion suite executes real Go demo/receipt commands and SQLite result readback in temporary roots. It exercises the actual bridge wrapper and installed WebhookAdapter send seam with offline fake delivery/readback, production versus synthetic dispatch, quiet/no-evidence behavior, exact readback failure, missing/empty artifacts, receipt-command failure, context mapping, restart persistence and concurrent/sequential duplicates. It does not invoke an LLM, live mail or Discord. Existence/content/provenance checks are mechanical safeguards, not proof that an LLM's preparation is semantically excellent; a real authorized source-to-result review remains required before activation.
