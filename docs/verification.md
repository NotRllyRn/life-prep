# Local verification handoff

Pre-public-push review: local preparatory scaffolding only, with no activation.

## Executed verification

- `go test -race -count=1 ./...`: PASS in CLI, core and Google adapter packages.
- `go vet ./...`: PASS.
- `go build -o bin/life-prep ./cmd/life-prep`: PASS.
- Repeated credential-free `demo`: both runs returned the same single pending event and existing JSON artifact. Artifact contents were read and checked as explicitly synthetic input, not live mail or completed preparation.
- `doctor config.example.json`: expected exit 1 with integrations-disabled explanation.
- `git diff --check`: PASS.

## Review fixes and regression coverage

Gmail preserves label IDs (including school origin), original Date, internalDate, and attachment metadata/references. Readonly Thread and explicit single Attachment helpers have httptest coverage. Message/thread retrieval does not automatically download attachments; external MIME body data is preserved as a reference for deliberate retrieval. Backfill uses `-in:sent -in:drafts` plus the age cutoff when applicable, never requires INBOX, and retains archived school mail. Current SENT/DRAFT/SPAM/TRASH labels are filtered during synchronization. Pagination, retrieval and decode failures return no partial results or advanced cursor. Existing artifacts are not retroactively updated.

Config decoding rejects unknown keys at every nesting level and trailing JSON. Account IDs are nonempty and unique; configured emails are unique case-insensitively and enabled accounts require emails. Disabled generic examples may omit emails. Hermes URL validation permits HTTPS or HTTP loopback only and rejects URL userinfo; remote cleartext cannot carry the configured authentication. Tests cover disabled integration commands and invalid configuration.

Core tests cover paused/unconfigured dispatch, propagation of pause-query failures, unknown receipts writing nothing, actual network cancellation becoming uncertain without unsafe retry, concurrent receipts across Store instances, late dispatch acceptance not undoing completed receipts, legacy Result-column migration and school context surviving JSON materialization. Completion paths are separate Result fields, not Error diagnostics. Receipt copies use atomic rename and per-event locking. Conditional dispatch claims/outcome updates protect state transitions. Schema setup is serialized across CLI opens.

Lifecycle review fixed partial-startup teardown: cancel and join existing workers before releasing the writer lock or closing SQLite. Existing cursor transaction rollback, duplicate ingestion, HMAC acceptance, crash recovery, ambiguous-response no replay, receipt validation and DST tests continue passing. A live multi-account daemon startup/failure sequence was not exercised.

README prominently documents the missing automatic completion bridge and manual operator-verified receipts. Status is a queue listing; doctor checks local prerequisites, not live health. The charter recognizes existing normally available Hermes sources as approved for relevant readonly investigation without per-source allowlists; new auth/elevation, external mutations and undesignated notices remain restricted.

## Not verified or implemented

No live OAuth/IAM/mailbox validation, Hermes agent completion, notice delivery or remote CI was exercised. The prior installed-source signature probe documented in `hermes.md` is not an end-to-end activation test. Automatic completion reconciliation remains absent. Production API/model/tool budgets, retry/resource ceilings, retention/deletion, capacity/backpressure and load/soak validation are explicitly deferred.

Before activation, validate grants and PubSub resources, background approvals, private artifact access, approved notice routing and actual delivery evidence, completion reconciliation, capacity and retention. No remote creation/push, service startup, skill installation or modification of Hermes outside this repository was performed. Only local commits were made; publication requires its own authorized workflow.
