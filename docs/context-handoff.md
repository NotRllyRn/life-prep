# Durable readiness context handoff

## Native integration, not an upstream fork

`integrations/hermes/life-prep-context` is a native Hermes plugin. It registers
`pre_gateway_dispatch` and, on installations providing it, `gateway_ready`.
The inbound hook also initializes adapter wiring after a native hot reload;
there is no gateway restart requirement and no upstream source patch.

Official references checked during implementation (2026-10-07):

- https://hermes-agent.nousresearch.com/docs/user-guide/features/hooks
- https://hermes-agent.nousresearch.com/docs/user-guide/features/plugins
- https://hermes-agent.nousresearch.com/docs/user-guide/messaging/webhooks

Verify the installed runtime, not merely the newest documentation. This plugin
needs native pre-dispatch rewrite support, plugin unload callbacks, Discord
message fetching and webhook cross-platform delivery. It uses an instance-local
webhook `send` wrapper because the installed hook catalog has no equivalent
post-delivery hook exposing every Discord message ID. That wrapper is removed on
unload and does not replace tools or modify upstream files. It relies on the
installed adapter's `_delivery_info`, Discord `_client` and routing-store
`_entries` fields; test these integration seams after upgrading Hermes.

## Binding and failure behavior

After a successful **agent-mode life-prep route** send, the plugin:

1. Confirms the route targets the privately configured fixed Discord feed.
2. Looks up the precise event in the life-prep ledger and validates its source
   artifact under `artifacts/<event-id>.json`.
3. Requires real files under `prepared/<event-id>/`, rejecting path escapes and
   symlink files. It records source email ID/account, prepared paths, plan paths,
   originating webhook chat and native session ID/key when one actually exists.
4. Reads back each returned Discord ID (including continuation chunks), checks
   channel and bot authorship, and persists its exact notice text and context in
   private `notice-context.db`.
5. Mirrors only this verified generated preparation notice through native
   `mirror_to_session`. It never mirrors incoming email or `deliver_only`
   payloads. Leave the route's own `mirror_to_session` disabled to avoid a
   duplicate or unsafe raw-payload mirror.

Explicit replies resolve the message reference. Message-started Discord threads
resolve their starter ID (Discord thread ID equals the starter message ID).
The adapter-provided parent channel must match the fixed designated feed.
No source text or path supplied by the reply is used as a routing authority.
Known bindings provide context automatically for requests such as **expand** or
**brief**. Unknown references, missing preparation or unavailable persistence
produce a fail-closed clarification instruction: **never use the latest notice**.
An unknown explicit reply does not fall back to a known thread or prior chat.
Unrelated channels are untouched. Newly created standalone threads are not
implicitly associated with any event.

Persistence is independent of process/module lifetime. A unique
`(channel,message)` cannot silently rebind to another event. SQLite connections
are short-lived, explicitly closed and writes transactional. Email contents are
not copied into the notice mapping. Referenced artifacts and generated notice
text remain data, not new authority to send email, edit calendars or act
externally.

A delivered notice whose mapping fails is **uncertain**, not a failed send.
The plugin logs exact chat/channel/message identifiers, returns the actual send
result, and never resends. Reconcile that exact message through authenticated
readback and the originating event; do not select recent messages heuristically.
HTTP 202 and session closure still do not prove completed preparation. Existing
operator-verified `receipt` semantics are unchanged.

## Reproducible deployment

Install only into the explicitly approved profile. With `HERMES_HOME` already
set to that profile and the repo path in `REPO`:

```sh
mkdir -p "$HERMES_HOME/plugins/life-prep-context"
cp "$REPO/integrations/hermes/life-prep-context/__init__.py" \
   "$REPO/integrations/hermes/life-prep-context/plugin.yaml" \
   "$HERMES_HOME/plugins/life-prep-context/"
hermes plugins enable life-prep-context --no-allow-tool-override
```

Before enabling, create a **private**, mode-0600
`$HERMES_HOME/life-prep-context.json` using operator-approved values:

```json
{"root":"/absolute/private/life-prep-root","discord_channel":"FIXED_APPROVED_FEED_ID"}
```

Do not commit this file, credentials, email data, notice IDs, state or prepared
artifacts. `root` must contain the existing `state.db`, `artifacts` and
`prepared` directories. The route's trusted prompt must require event-specific
`prepared/<event-id>/` output. Enabling the plugin on the tested runtime reloads
hooks into the running gateway via the native control socket. If needed, use
`gateway.control_socket.reload_gateway_plugins(Path(HERMES_HOME))` from the
installed Hermes interpreter and check `reloaded`, the plugin's activation
summary and `adapters_rewired`. Do not restart a gateway from one of its workers:
its service cgroup can kill that worker and its parent.

The Go daemon also accepts numeric Gmail history hints and coalesces new-mail
wakeups to dispatch immediately after ingestion, without blocking ingestion on
an agent request. A one-minute sweep remains for recovery. Deploy its verified
binary atomically and restart **only life-prep**, through the approved service
operator. Verify the service PID, running executable checksum and status.

Rollback: disable this plugin and hot-reload hooks; its unload callback restores
the previous adapter method. Preserve `notice-context.db` for exact-message
reconciliation. Restore the previous daemon binary and restart only its service
if needed. Do not delete source cursors or retry accepted/uncertain work blindly.

## Tests and bounded synthetic verification

```sh
go test ./...
go test -race ./...
go vet ./...
go build -o bin/life-prep ./cmd/life-prep
python3 -m unittest discover -s integrations/hermes -p 'test_*.py' -v
```

Credential-free tests recreate both the Python module and store with two
records, verify distinct reply/thread resolutions and unknown-reference failure,
check path confinement/collisions, map continuation chunks and native session
identity, unload wrappers, and prove mapping failures never resend. CI runs
these tests alongside Go race/vet/build and the local synthetic demo.

The operator-only `integrations/hermes/synthetic_smoke.py` uses existing private
credentials without printing them. Its `prepare` phase inserts two explicitly
synthetic ledger items and signs local webhook requests. Its `resolve` phase
verifies real readiness messages, creates synthetic bot replies/message-started
threads **only in the specified approved feed**, fetches them through the live
Discord adapter, and exercises the actual gateway pre-dispatch rewrite method.
It verifies two distinct reply cases, two thread cases and an unknown-pointer
negative case. It does not impersonate a human or bypass gateway authorization
for a human agent turn.

To run these probes, temporarily add `"synthetic_probes": true` to the private
plugin configuration and hot-reload. This feature accepts only locally recorded
`activation-synthetic` events and own-bot messages in the designated feed.
Remove the flag and hot-reload after verification. The smoke tool requires the
existing Hermes environment's `python-dotenv`; no new account access is needed.

```sh
HERMES_PYTHON "$REPO/integrations/hermes/synthetic_smoke.py" \
  --root "$PRIVATE_ROOT" --channel "$APPROVED_FEED" \
  --secret-env-file "$PRIVATE_WEBHOOK_ENV" \
  --discord-env-file "$HERMES_HOME/.env" --phase prepare
# Same arguments, with --phase resolve, once preparation completes.
```

If the live gateway is already draining and refusing new agent turns, do not
restart it from the worker or falsely claim an LLM run. The explicit
`--deterministic` preparation option invokes the opt-in native synthetic fixture:
it creates a real checklist/plan locally and delivers an honestly labeled
synthetic readiness notice via the existing route. This validates the actual
signed-webhook → local preparation → Discord readback → durable mapping →
native reply/thread context path, but **not a live LLM-generated brief**.
A human follow-up and ordinary LLM preparation must be verified after the
operator safely clears the pre-existing drain/restart condition.
