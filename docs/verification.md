# Local verification handoff

Prior build evidence recorded below (not re-executed by this documentation-only review):

- `go test -race ./...`: PASS, core and Google adapter packages; CLI currently has no package tests.
- `go vet ./...`: PASS.
- `go build -o /home/hermes/.hermes/cache/scratch/life-prep ./cmd/life-prep`: PASS.
- Repeated `demo` into private scratch directory: same single pending event and durable JSON artifact both times; no live result claimed.
- `doctor config.example.json`: expected nonzero exit with explicit integrations-disabled explanation.
- `git diff --check`: PASS.

Tests use credential-free fixtures. Gmail httptest covers profile/list/history pagination, MIME retrieval, expired history and watch. Core tests exercise cursor/queue transaction rollback, duplicate ingestion, signed HTTP fixture acceptance, crash-to-uncertain recovery, no automatic ambiguous replay, pause, completion receipt and DST briefing keys. Full daemon/live OAuth/IAM/Discord/Hermes agent completion has not been exercised. CI has not run remotely.

Source review identified Hermes generic V2 signing, timestamp tolerance, per-event sessions, elevated tool controls and non-durable dedup. Installed source validator was probed separately by extracting its actual method: signature accepted, tamper and absent timestamp rejected. This is not end-to-end route activation proof.

Publication requires a separate authorized workflow. This build makes only local commits; an origin URL was already present and was left unchanged. No remote API operation, push, live credentials, Hermes configuration change or systemd activation was performed.

Documentation review found the pre-change Gmail adapter lacked the exact received filter `-in:sent -in:drafts` and history label exclusions. Code changes and targeted fixture evidence must establish archived inclusion and default spam/trash exclusion before this gap is closed. This review does not claim the correction or live validation has happened.

Production budgets, retention/deletion and capacity controls are explicitly deferred. Relevant read-only investigation of normally available Hermes harness sources is already approved without a per-source allowlist; new authentication, elevation, external mutations and undesignated notices remain restricted. No activation was performed by this review.

Before activation: validate read-only grants, topic/subscription IAM, background tool approvals, shared private artifact access, designated readiness/urgent/briefing Discord notices, reliable completion evidence, capacity and retention. Receipt is operator-assisted; automatic receipt bridge and automatic notice delivery remain unsupported until externally configured/verified. Latest selected SDK dependencies raise the required Go toolchain to 1.26; the installed Go 1.24.4 launcher downloaded a compatible newer toolchain successfully.
