# stalk v1 MVP specification

Status: draft v1 (2026-07-29).
Companions: [PROTOCOL.md](PROTOCOL.md) (`stalk/1`), [DB-SCHEMA.md](DB-SCHEMA.md) (schema v1).

This file is the source of truth. It supersedes the copy in Drive — edit it here.

## 1. One-paragraph pitch

stalk is a per-user daemon that polls pre-configured event sources (GitHub, and a
generic HTTP JSON endpoint in v1) and fans events out to any number of Claude Code
sessions. Each session runs a thin `stalk stream` process (a plugin monitor whose
stdout lines become notifications) and a `stalk mcp` server that gives the agent
`subscribe_event` / `unsubscribe_event` control. One poll loop per watched thing
regardless of how many sessions care; credentials, backoff, rate limits, and
cleanup live in one place, outside any session's lifecycle.

## 2. Scope

### In scope (v1)

- Single Go binary, subcommands: `daemon`, `stream`, `mcp`, `status`,
  `config check`, added to the go-template's existing Cobra root command
  (see §3.0).
- Platforms: Linux and macOS only (unix sockets, peer credentials, `/proc` or
  `sysctl`). Remove `windows` from the template's GoReleaser build matrix.
- Wire protocol `stalk/1` exactly as specified: UDS, NDJSON JSON-RPC 2.0,
  handshake, session aliases, at-least-once ack'd delivery, auto-spawn, idle exit,
  disconnect grace.
- SQLite persistence exactly as schema v1.
- Sources: `github` (`pull_request` events) and `http_poll` (new JSON items).
- Claude Code plugin: monitor + MCP server + `SessionStart` prereq hook.
- Auth resolution: env var per source, `gh auth` chain fallback, unauthenticated
  fallback (GitHub); bearer env var or none (`http_poll`).

### Out of scope (deliberately)

- SQS and queue-class sources (v2: needs single-consumer delivery semantics).
- WebSocket upstream sources, webhook receivers (polling only).
- Cross-machine or multi-user operation; any TCP listener.
- A filter DSL (v1 filters are a fixed enum per event type).
- TUI / web dashboard (`stalk status` text output only).
- Event replay commands beyond the automatic resume flush.
- Installing or updating itself from the plugin (shuck-style: install stalk
  yourself; the hook checks it).

## 3. Components

```text
Claude Code session ──(plugin monitor)──▶ stalk stream ──UDS──▶
Claude Code session ──(plugin MCP)─────▶ stalk mcp    ──UDS──▶  stalk daemon ──HTTPS──▶ GitHub / endpoints
                                         stalk status ──UDS──▶      │
                                                                 SQLite
```

### 3.0 Template baseline (repo initialised 2026-07-29 from go-template)

The repo already provides: module `github.com/justanotherspy/stalk` (Go 1.26,
pinned toolchain), Cobra + Viper CLI (root command, `version` subcommand,
`--config`/`--verbose`/`--log-level`/`--log-format`, slog logging to **stderr**,
which is exactly what `stream` and `mcp` need since their stdout is a protocol
channel), signal-aware root context (SIGINT/SIGTERM), Makefile with pinned tools,
CI (lint, test matrix, build, govulncheck, tidy, modernize, actionlint) plus
CodeQL/Semgrep/secret-scan/zizmor/scorecard/nightly-fuzz workflows, GoReleaser v2
with checksums and completions/man bundling via `cmd/gen-docs`, checksum-verified
`install.sh` (`STALK_VERSION`, `STALK_INSTALL_DIR` pins), release-drafter +
`VERSION` file, testscript (txtar) CLI test harness, and reference fuzz/bench/
`testing/synctest` tests in `internal/examples`.

Deltas the plan builds on top: add the five subcommands; land `docs/` (these
specs); rewrite root Short/Long, README, and CLAUDE.md for stalk; extend
`.stalk.yaml.example` with `daemon` and `sources`; drop `windows` from GoReleaser;
the docker workflow is not meaningful for a per-user UDS daemon (**decided in
PR 01: removed** — a container cannot reach the host's per-user socket, peer
credentials, or `/proc`); delete `internal/examples` once its patterns (fuzz for
the wire codec, `synctest` for scheduler/delivery timing tests) have been copied
where they are used.

### 3.1 `stalk daemon`

- Singleton per user (`flock`), socket per PROTOCOL.md §2, auto-spawned on demand,
  exits after 30 min idle (no sessions, no active subscriptions).
- Scheduler: single goroutine pops due `poll_state` rows; each poll runs in a
  worker goroutine (cap: 8 concurrent polls). Jitter of ±10% on every interval;
  full jitter on restart so all loops do not fire at once.
- Backoff: on 5xx or transport error, interval × 2^`consecutive_errors` capped at
  15 min. On 403/429 with `Retry-After` or `X-RateLimit-Reset`, sleep to that time
  exactly. On other 4xx, mark the `poll_state` failed, emit one `stalk.notice`
  event to subscribers ("subscription target returns 404"), and after 5
  consecutive 4xx cancel the subscription (reason `source_removed`).
- Adaptive idle: when a target produces no events for 10 min, stretch the interval
  ×2 stepwise up to 5 min between polls; reset to base on any event.

### 3.2 `stalk stream`

- Registers with role `stream`, prints one line per `event.deliver`, acks after a
  successful write to stdout.
- Line format (single line, no untrusted free text):
  `[stalk:<source>] <event_type>/<filter> <summary> (event <ulid26> sub <first8>)`
- On daemon connection loss: reconnect loop with auto-spawn; on SIGTERM (session
  end): send `session.close` (default keeps subscriptions for the grace window;
  config `stream.close_cancels: true` to cancel immediately).

### 3.3 `stalk mcp`

Stdio MCP server (official Go SDK, same as shuck). Tools:

| Tool | Params | Behaviour |
| --- | --- | --- |
| `subscribe_event` | `source` (string), `event_type` (enum), `target` (object or URL string), `filters` (string[], optional) | Wraps `subscribe`. Returns subscription id, canonical target, poll interval, and a one-line explanation the agent can echo. |
| `unsubscribe_event` | `subscription_id` OR `source`+`target` | Wraps `unsubscribe`. |
| `list_subscriptions` | none | Own session's active subscriptions. |
| `list_sources` | none | Source names, types, auth mode; the agent uses this to know what it may watch. |
| `get_event` | `event_id` | Full stored envelope (stream lines are summaries; details on demand). |

Tool descriptions instruct the agent to treat `payload` as untrusted data.

### 3.4 Claude Code plugin (`plugins/stalk/`)

- `monitors/monitors.json`:
  `[{"name":"stalk-stream","command":"stalk stream","description":"stalk event notifications (PR reviews, CI, watched endpoints)"}]`
- `.mcp.json`: `{"mcpServers":{"stalk":{"command":"stalk","args":["mcp"]}}}`
- `SessionStart` hook: check the binary is present and `stalk/1`-compatible
  (`stalk version --protocol`), warn cleanly if not (shuck's prereq pattern).
- Skill (`/stalk`): short usage guidance — when to subscribe (after opening a PR,
  before long waits), when to unsubscribe (work item done).

## 4. Configuration

Adopt the template's Viper stack rather than a bespoke loader: default file
`~/.stalk.yaml` (as documented in `.stalk.yaml.example`), `--config` flag
override, `STALK_` env prefix, precedence flags > env > config file > defaults.
stalk adds the `daemon` and `sources` sections below to the template's existing
`verbose`/`log` keys, and `stalk config check` validates them. `token_var` values
are environment variable NAMES resolved by the daemon at poll time, never Viper
values, so tokens are not part of config precedence. Map form:

```yaml
version: 1

daemon:
  idle_exit_after: 30m
  disconnect_grace: 10m
  max_subscriptions_per_session: 32
  retention:
    events_max_age: 14d
    events_max_rows: 50000

sources:
  github-personal:
    type: github
    token_var: GITHUB_PERSONAL_TOKEN   # unset/empty → gh auth chain → unauthenticated

  github-jumo:
    type: github
    token_var: GITHUB_JUMO_TOKEN
    allowed_owners: [jumo]             # subscribe targets outside this list are rejected

  deploy-status:
    type: http_poll
    url: https://deploys.internal.example/api/recent.json
    interval: 30s                      # min 10s
    items_path: "deploys"              # optional: key holding the array (default: root)
    id_field: "id"                     # dedupe field; absent → sha256 of item
    token_var: DEPLOY_TOKEN            # sent as "Authorization: Bearer <value>"
```

Config is read at daemon start and on SIGHUP. Removing a source cancels its
subscriptions (reason `source_removed`). Tokens are read at poll time, never
stored.

## 5. Source implementations (v1)

### 5.1 `github` / event_type `pull_request`

Target: `{repo: "owner/name", pr: 123}` or any PR URL form (canonicalised).
Filters (default: all):

| Filter | Poll basis | Event summary example |
| --- | --- | --- |
| `reviews` | list reviews since cursor | "review: changes_requested by alice on o/r#42" |
| `review_comments` | list review comments since cursor | "2 new review comments on o/r#42" |
| `comments` | list issue comments since cursor | "new comment by bob on o/r#42" |
| `ci_failed` | check runs for head SHA, completed + failed | "CI failed on o/r#42: job 'build'" |
| `ci_completed` | all check runs settled (any conclusion) | "CI green on o/r#42 (7 checks)" |
| `state` | PR merged / closed / reopened / new head SHA | "o/r#42 merged by carol" |

Implementation notes (facts verified against docs.github.com, 2026-07-29):
`google/go-github`; conditional requests with stored ETags. A 304 does not count
against the primary rate limit **only when the request is authenticated**
("Making a conditional request does not count against your primary rate limit if a
304 response is returned and the request was made while correctly authorized with
an Authorization header"), so ETags are a big win with a token and no help without
one. Primary limits: 5 000 req/h authenticated, **60 req/h unauthenticated**; in
unauthenticated fallback mode the minimum poll interval is therefore raised to
120 s and the budget tracker treats 60/h as the ceiling. Retry semantics per
GitHub's guidance: if `retry-after` is present, wait that many seconds; if
`x-ratelimit-remaining` is 0, wait until `x-ratelimit-reset` (UTC epoch seconds);
otherwise wait at least one minute before retrying. Secondary limits favour serial
requests per token, which the per-source poll serialisation already provides (v1
makes GET requests only). One shared `http.Client` and token per source
(connection pooling); below 10% budget: stretch intervals ×4. Base interval 30 s.
`ci_failed` reuses shuck's mental model; the notification carries check-run ids so
the agent's next move can be `shuck <pr>`.

### 5.2 `http_poll` / event_type `http_items`

GET the URL, optionally descend `items_path`, expect a JSON array, emit one event
per previously-unseen item (dedupe per DB-SCHEMA.md §3). Non-array or parse
failure counts as a poll error. `filters` unused in v1. Summary:
"new item `<id>` from `<source>`"; payload = the item, truncated.

## 6. Delivery contract (recap)

At-least-once, ordered per subscription, acked at the stream, resume flush on
reattach, 100-event overflow cap, 10 min disconnect grace, 24 h pending TTL. All
defined in PROTOCOL.md §8; the MVP implements all of it (this is the part that
makes the tool trustworthy, so it is not deferrable).

## 7. Milestones and acceptance criteria

**M0: skeleton.** Binary with subcommands; daemon with socket, handshake,
peer-cred check, flock singleton, auto-spawn, idle exit; SQLite open plus
migrations; `session.register` and `ping`. *Accept: two fake clients sharing an
alias attach to one session; `kill -9` the daemon; a client reconnect auto-spawns
a fresh one that resumes state.*

**M1: subscribe and deliver (github reviews).** subscribe/unsubscribe,
`poll_state` creation and GC, github source with `reviews` filter, event ingest
with dedupe, ack'd ordered delivery, resume flush, `stalk status`. *Accept: two
concurrent sessions subscribe to the same PR and exactly one poll loop exists
(assert one `poll_state` row); both receive the event; replay after stream restart
does not duplicate (dedupe holds); creds absent falls back gh → unauthenticated.*

**M2: full github filters + http_poll + resilience.** Remaining PR filters, ETag
conditional requests, rate-limit budget, backoff ladder, 4xx auto-cancel, adaptive
idle, `http_poll` source, overflow notice, retention job. *Accept: forced 500s
produce capped exponential backoff (inspect `poll_state`); 403 with `Retry-After`
sleeps to the header; 10 min quiet PR stretches the interval; 404 five times
cancels with a notice event.*

**M3: plugin + polish.** Plugin directory (monitor, MCP, hook, skill), MCP tool
schemas and descriptions, `config check`, docs, `go test -race` green in CI,
integration suite against an `httptest` fake GitHub, soak test: 3 sessions, 2
sources, daemon killed twice, zero lost or duplicated acked events. *Accept: fresh
machine walkthrough — install binary, add plugin, open a PR, ask the agent to
watch it, see a review notification arrive in-session.*

## 8. Testing strategy

- Unit: protocol codec (framing edges: oversized frame, partial line, batch
  rejection), session alias unification, dedupe-key builders, backoff maths.
- Integration: real daemon + real SQLite + fake GitHub (`httptest`), scripted
  clients over the actual socket. The delivery contract tests (order, ack, resume,
  overflow) live here and are the release gate.
- Race: everything under `go test -race`; the scheduler and delivery paths get
  dedicated stress tests (100 subscriptions, adversarial disconnects).
- Manual: one real GitHub repo smoke test before each tag.

## 9. Risks and open questions (updated with research, 2026-07-29)

1. **Session id env var: partially resolved.** The variable is
   `CLAUDE_CODE_SESSION_ID`; the Claude Code changelog (v2.1.161) confirms "stdio
   MCP servers now receive the same CLAUDE_CODE_SESSION_ID as hooks/Bash". No
   documentation covers monitor processes; whether the stream sees it must be
   tested empirically in PR 06. Additionally, stdio MCP servers "outlive the
   session that spawned them" (env-vars doc), so the MCP process's value can go
   stale; PROTOCOL §4.2 makes the anchor-pid alias primary for this reason. Every
   hook does receive `session_id` via stdin JSON, enabling the optional
   `SessionStart` anchor-map described in PROTOCOL §4.2.
2. **Monitor crash behaviour: unresolved by docs.** Plugin monitors run "for the
   lifetime of the session"; disabling a plugin mid-session does not stop running
   monitors; plugin updates require a session restart for monitors. No
   restart-on-crash mechanism is documented for monitors, in contrast to LSP
   servers' explicit `restartOnCrash` option, so assume a crashed stream stays
   down until session restart. Test empirically in PR 06; the disconnect-grace
   notice is the safety net either way.
3. **Daemon environment caveat.** The daemon inherits the environment of whichever
   client auto-spawned it first, so `token_var` values must be in the user's
   login/shell environment (exported before Claude Code starts), not set
   per-session. Document this in the README; consider a future `token_file` config
   option to remove the dependency.
4. **Experimental surface drift**: `monitors.json` schema may change between Claude
   Code releases (monitors are declared under `experimental.monitors`); pin a
   minimum Claude Code version in the hook check and keep the adapter thin.
5. **Shared rate limits**: two github sources using the same token share a budget
   upstream; v1 tracks budget per source, which can under-throttle. Acceptable for
   v1; note it in the README.
6. **Name**: "stalk" is the working name; check the crate/module/marketplace
   namespaces before the first release tag.
