# Design before core implementation

Small Go daemon: official Gmail/PubSub adapters -> serialized account synchronization -> SQLite atomic cursor/outbox -> private durable JSON artifacts -> optional Hermes HMAC trigger. No public listener. Account IDs namespace message IDs. Notification history IDs are hints, never cursor arithmetic. Initial sync is 21 days; expired history requires full mailbox resync. Queue states pending, dispatching, accepted, uncertain, completed. Before network dispatch persist dispatching; restart changes dispatching to uncertain, never blindly redispatch. Receipt CLI records operator-verified completion. Briefing uses civil 08:00 America/Los_Angeles time and unique daily event key. Integrations default disabled.

Implementation checklist:
- [x] Research official API docs and installed Hermes protocol
- [x] SQLite atomic ingestion and duplicate protection
- [x] Artifacts, dispatch state and operator-verified completion receipts
- [x] Daemon and diagnostic CLI
- [x] Credential-free fixture tests, race/vet/build/demo
- [x] Setup, operations, systemd and CI

Verification: Go tests with race detector, vet and build passed locally. Repeated fixture demo yielded the same single durable event and artifact. Disabled doctor failed explicitly. Tests cover SDK pagination/MIME/watch/history expiration, transactional rollback, HMAC acceptance, pause, uncertain-response no replay, receipt validation, crash recovery and DST date dedup. CI is supplied but has not run remotely.

Remaining hookup gates: live OAuth/PubSub IAM validation; automatic verified receipt bridge; Hermes background approvals; generic private Discord readiness/urgent/briefing route configuration and actual delivery evidence. These are not claimed deployed. Full resync memory capacity and manual retention need operator validation. Go dependencies require Go 1.26; installed 1.24.4 auto-downloaded a newer toolchain.

Activation gates remain separate: OAuth grants, PubSub IAM/resources, Hermes approval validation, notice routing, receipt bridge verification. No deployment or Hermes config changes during this build.
