# life-prep

Preparatory, opt-in local Gmail context queue for Hermes. No integrations are activated by installation. Two independently configured read-only mailboxes feed official Gmail and PubSub pull clients, durable SQLite queue, private human-readable JSON artifacts and optional signed local Hermes triggers. This is not a mail sender, calendar editor, dashboard or sandbox.

## Readiness: preparatory only — manual receipts

**No automatic completion bridge is implemented.** Signed dispatch/HTTP 202 is acceptance only; completion is recorded solely through an operator-verified `receipt CONFIG ID RESULT_FILE`. No end-to-end Gmail → Hermes preparation → notice → completion workflow has been validated. Installation does not activate accounts, routes, skills, schedules or services. This is a credential-free tested preparatory foundation, not production-ready unattended automation.

The received-mail selection contract is exactly `-in:sent -in:drafts`: archived received mail is included; spam/trash are excluded by default. Apply the same exclusions to initial/backfill listing, history additions and expired-history recovery. See [architecture](docs/architecture.md) for the implementation-verification gate.

Production API/model/tool budgets, retry/resource ceilings, retention/deletion policies and mailbox/queue/artifact capacity limits are **explicitly deferred**; they are neither enforced nor approved by this repository. Resolve and exercise them before activation. Normally available Hermes harness sources are already approved for relevant read-only investigation; no per-source allowlist is required. That approval does not grant new authentication, tool elevation, external mutations or undesignated notices.

## Try without credentials

```
go test -race ./...
go vet ./...
go build -o bin/life-prep ./cmd/life-prep
bin/life-prep demo /absolute/private/demo-directory
bin/life-prep demo /absolute/private/demo-directory
```

The demo uses explicitly synthetic fixture content in the actual transaction/artifact pipeline; repeating it leaves one event. HTTP fixture tests exercise real SDK Gmail requests and HMAC dispatch. No demo contacts Google or Hermes. Dependencies currently require Go 1.26: the installed Go 1.24.4 automatically selected a newer toolchain. Offline builds require the toolchain/dependency cache first.

Read [design](docs/plan.md), [architecture](docs/architecture.md), [setup](docs/setup.md), [Hermes protocol](docs/hermes.md), [runbook](docs/runbook.md), and the proposed [charter](skills/life-prep/SKILL.md).

Public repository must contain only code, generic examples and documentation. Credentials, actual mailbox identifiers, state and artifacts belong outside version control. Local documentation work does not authorize remote publication or deployment; no push is performed by this review.
