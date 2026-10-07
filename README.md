# life-prep

Preparatory, opt-in local Gmail context queue for Hermes. No integrations are activated by installation. Two independently configured read-only mailboxes feed official Gmail and PubSub pull clients, durable SQLite queue, private human-readable JSON artifacts and optional signed local Hermes triggers. This is not a mail sender, calendar editor, dashboard or sandbox.

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

Public repository must contain only code, generic examples and documentation. Credentials, actual mailbox identifiers, state and artifacts belong outside version control. No remote has been created or pushed by this build.
