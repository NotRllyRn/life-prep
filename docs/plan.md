# Design before core implementation

Small Go daemon: official Gmail/PubSub adapters -> serialized account synchronization -> SQLite atomic cursor/outbox -> private durable JSON artifacts -> optional Hermes HMAC trigger. No public listener. Account IDs namespace message IDs. Notification history IDs are hints, never cursor arithmetic. Initial sync is 21 days; expired history requires full mailbox resync. Queue states pending, dispatching, accepted, uncertain, completed. Before network dispatch persist dispatching; restart changes dispatching to uncertain, never blindly redispatch. Receipt CLI records operator-verified completion. Briefing uses civil 08:00 America/Los_Angeles time and unique daily event key. Integrations default disabled.

Implementation checklist:
- [x] Research official API docs and installed Hermes protocol
- [ ] SQLite atomic ingestion and duplicate protection
- [ ] Artifacts, dispatch state and completion receipts
- [ ] Daemon and diagnostic CLI
- [ ] Credential-free fixture tests, race/vet/build/demo
- [ ] Setup, operations, systemd and CI

Activation gates remain separate: OAuth grants, PubSub IAM/resources, Hermes approval validation, notice routing, receipt bridge verification. No deployment or Hermes config changes during this build.
