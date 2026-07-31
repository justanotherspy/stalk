# stalk wire protocol (`stalk/1`)

Status: draft v1 (2026-07-29), amended 2026-07-31.
Companions: [DB-SCHEMA.md](DB-SCHEMA.md) (schema v1), [MVP-SPEC.md](MVP-SPEC.md) (scope).

This file is the source of truth. It supersedes the copy in Drive — edit it here.

This document specifies the inter-process protocol between the stalk daemon and
its clients: `stalk stream` (per-session event channel), `stalk mcp` (per-session
MCP server exposing subscription tools), and the `stalk` CLI subcommands (admin
and introspection). The daemon is the sole owner of all state; clients hold none.

## Amendments

**2026-07-31 — §4.2 session identity.** Corrected against primary sources
(Claude Code changelog v2.1.161 and the env-vars documentation); the findings are
recorded in the Linear project document *Research notes: verified facts for
implementation handoff*.

- The environment variable is `CLAUDE_CODE_SESSION_ID`, not `CLAUDE_SESSION_ID`.
- The `anchor:` alias is now **primary** and `ccsid:` is advisory. Stdio MCP
  server subprocesses "are long-lived and outlive the session that spawned them",
  so an `stalk mcp` process can hold a stale session id across `/clear` or resume.
  The anchor is derived from the shared Claude Code ancestor process, so it stays
  correct.
- `/proc/<pid>/stat` field 22 must be parsed by splitting after the **last** `)`:
  the `comm` field can contain spaces and parentheses.

## 1. Design basis

The protocol is **JSON-RPC 2.0 over a unix domain socket, newline-delimited**.

Rationale, by precedent:

- JSON-RPC 2.0 is the message layer of both LSP and MCP. The `stalk mcp` process
  already speaks JSON-RPC to the agent, so the same conventions apply on both of
  its faces, and the official Go SDK patterns carry over.
- A per-user daemon behind a unix socket is the Docker Engine / tailscaled /
  gpg-agent pattern: no TCP port, no TLS, OS-enforced access control.
- Newline-delimited JSON (one message per line) is the MCP stdio framing. It is
  trivially debuggable: `socat - UNIX-CONNECT:$SOCK | jq .`
- gRPC was considered and rejected: it adds protobuf toolchain and code-gen cost,
  loses easy shell debugging, and buys streaming we get natively from a
  long-lived socket with server-to-client requests.

## 2. Transport

### 2.1 Socket path resolution (in order)

1. `$STALK_SOCKET` if set (absolute path).
2. `$XDG_RUNTIME_DIR/stalk/daemon.sock` if `XDG_RUNTIME_DIR` is set (Linux).
3. `~/.stalk/run/daemon.sock` (macOS and fallback).

The containing directory is created `0700`, owned by the user. The socket file is
created `0600`. Both the daemon and clients resolve the path with the same rules.

### 2.2 Peer verification

On every accepted connection the daemon reads peer credentials (`SO_PEERCRED` on
Linux, `LOCAL_PEERCRED` on macOS) and drops the connection unless
`peer.uid == daemon.uid`. Directory permissions are the first gate; the peer
check is defence in depth.

### 2.3 Connection limits

- Max concurrent connections: 64 (config: `daemon.max_connections`).
- Max inbound message rate per connection: 50/s sustained; excess triggers
  `-32009 RATE_LIMITED`, repeated abuse closes the connection.

## 3. Framing

- One complete JSON-RPC 2.0 message per line, UTF-8, terminated by `\n`. JSON
  string escaping guarantees no literal newlines inside a message.
- Maximum frame size: 256 KiB. Oversized frames close the connection with a final
  `-32010 FRAME_TOO_LARGE` error where possible.
- JSON-RPC batch arrays are not supported (same restriction as MCP, which removed
  batching in its 2025-06-18 revision).
- Requests carry `id` (string or number); notifications carry none. Responses
  echo the request `id`. Both directions may send requests: the daemon sends
  `event.deliver` and `ping` requests to clients and awaits their responses.

## 4. Session model

### 4.1 Roles

Each connection declares a role in `handshake`:

| Role | Cardinality | Purpose |
| --- | --- | --- |
| `stream` | one per session | receives `event.deliver`, prints lines to stdout |
| `mcp` | one per session | issues `subscribe` / `unsubscribe` on the agent's behalf |
| `cli` | ephemeral | `daemon.status`, `events.get`, admin |

### 4.2 Session identity

The stream and MCP processes of one Claude Code session are separate OS processes
and must resolve to the same session. Each computes an ordered list of **session
aliases** and sends all of them in `session.register`:

1. `anchor:<pid>:<starttime>` via the anchor algorithm: walk the parent-pid chain
   from self; skip ancestors whose executable basename is a shell (`sh`, `bash`,
   `zsh`, `dash`, `fish`); the first non-shell ancestor is the anchor.
   `<starttime>` is the kernel process start tick (`/proc/<pid>/stat` field 22 on
   Linux — parse by splitting after the last `)`, since `comm` may contain spaces
   and parens; `sysctl kern.proc.pid` on macOS), which makes the alias immune to
   pid reuse. **This is the primary alias.**
2. `ccsid:<value>` if the `CLAUDE_CODE_SESSION_ID` environment variable is set.
   Advisory only: stdio MCP servers outlive the session that spawned them, so
   this value can go stale across `/clear` or resume.
3. `manual:<value>` if `--session <value>` was passed (tests, debugging).

The daemon unifies: if any alias in a registration matches an existing session,
the connection attaches to that session and any new aliases are added to it.
Otherwise a new session record is created keyed by the first alias.

Two consecutive Claude Code sessions inside one process unify into one stalk
session via the anchor. That is correct behaviour — the delivery target is the
process's current stream.

Optional enrichment: every hook receives `session_id` in its JSON stdin, so a
plugin `SessionStart` hook can write an `anchor` → `session_id` map under
`$CLAUDE_PLUGIN_DATA`. The hook shares the Claude Code ancestor process and so
computes the same anchor. Not load-bearing.

### 4.3 Registration ordering

`mcp` may register and subscribe before `stream` has attached. Events for a
session with no attached stream accumulate as pending deliveries (bounded, see
§8.4) and flush when the stream attaches.

## 5. Handshake and versioning

The first message on every connection MUST be a `handshake` request. Any other
method first gets `-32001 NOT_HANDSHAKEN` and the connection is closed.

```json
{"jsonrpc":"2.0","id":1,"method":"handshake","params":{"protocol":"stalk/1","role":"stream","client_version":"0.1.0"}}
{"jsonrpc":"2.0","id":1,"result":{"protocol":"stalk/1","daemon_version":"0.1.0","daemon_instance":"01J3ZV7Q0RWX5E9T4M2K8C6D1P"}}
```

- `protocol` is a single opaque version token, bumped only on breaking change.
  Unknown token: `-32000 VERSION_UNSUPPORTED` with `data.supported: ["stalk/1"]`,
  then close. Clients may then advise the user to restart the daemon (binary
  upgrades while an old daemon runs are the expected skew case).
- `daemon_instance` is a ULID minted at daemon start. Clients that observe a
  changed instance after reconnect know state was reconciled from disk.

## 6. Methods: client to daemon

All examples omit `jsonrpc` and `id` for brevity; all are standard requests.

### 6.1 `session.register`

Registers (or attaches to) a session. Stream and MCP both call it after handshake.

```json
{"method":"session.register","params":{
  "aliases":["anchor:41213:8843921","ccsid:9f8a1c2e"],
  "role":"stream",
  "pid":41290,
  "cwd":"/home/dan/src/garlic",
  "client_version":"0.1.0"
}}
```

```json
{"result":{
  "session_key":"anchor:41213:8843921",
  "resumed":true,
  "active_subscriptions":2,
  "pending_deliveries":1
}}
```

`resumed` is true when the session already existed (stream restart, daemon
restart, or the MCP registered first). After a stream registers, the daemon
flushes pending deliveries in order (§8.3).

### 6.2 `session.close`

Graceful goodbye (the stream receives SIGTERM at session end). Cancels nothing by
itself: subscriptions enter the disconnect grace period (§8.5) unless
`params.cancel_subscriptions` is true.

```json
{"method":"session.close","params":{"cancel_subscriptions":true}}
```

### 6.3 `subscribe`

`mcp` role only. Validated against the source config; idempotent — an identical
active tuple (session, source, event_type, target, filters) returns the existing
subscription.

```json
{"method":"subscribe","params":{
  "source":"github-jumo",
  "event_type":"pull_request",
  "target":{"url":"https://github.com/jumo/lending-api/pull/812"},
  "filters":["reviews","ci_failed"]
}}
```

```json
{"result":{
  "subscription_id":"01J3ZVAX7GQ2M4N8B5C1D9E0FK",
  "created":true,
  "target":{"repo":"jumo/lending-api","pr":812},
  "poll_interval_ms":30000
}}
```

The daemon canonicalises `target` (URL parsed to `{repo, pr}`) and returns the
canonical form. Validation failures: `-32003 UNKNOWN_SOURCE`,
`-32004 INVALID_TARGET`, `-32005 SOURCE_AUTH_UNAVAILABLE`,
`-32007 LIMIT_EXCEEDED` (per-session subscription cap, default 32).

### 6.4 `unsubscribe`

By id, or by target match when the agent lost the id.

```json
{"method":"unsubscribe","params":{"subscription_id":"01J3ZVAX7GQ2M4N8B5C1D9E0FK"}}
{"method":"unsubscribe","params":{"source":"github-jumo","target":{"repo":"jumo/lending-api","pr":812}}}
```

Result: `{"cancelled": 1}`. Unknown id: `-32006 SUBSCRIPTION_NOT_FOUND`.

### 6.5 `subscriptions.list`

Own session by default; `{"all": true}` from the `cli` role lists every session.

### 6.6 `sources.list`

Names, types, and the auth mode actually resolved (`token_env`, `gh_cli`,
`credential_chain`, `unauthenticated`, `none`), never secret values.

```json
{"result":{"sources":[
  {"name":"github-personal","type":"github","auth":"token_env"},
  {"name":"deploy-status","type":"http_poll","auth":"unauthenticated"}
]}}
```

### 6.7 `events.get`

Full stored envelope for an event id (stream lines are summaries; the agent
fetches detail on demand through the MCP tool that wraps this).

### 6.8 `daemon.status` / `daemon.shutdown`

`cli` role. Status returns uptime, session/subscription counts, and per-source
poll health (`consecutive_errors`, `backoff_until`, `last_http_status`).
Shutdown performs a clean stop; streams see the socket close and reconnect-loop
until a new daemon spawns (§9).

## 7. Methods: daemon to client

### 7.1 `event.deliver` (to stream)

A request, not a notification: the client's response IS the acknowledgement.

```json
{"jsonrpc":"2.0","id":907,"method":"event.deliver","params":{
  "event":{
    "id":"01J3ZVCK2P8R5T1W9X4Y7Z0QAB",
    "subscription_id":"01J3ZVAX7GQ2M4N8B5C1D9E0FK",
    "source":"github-jumo",
    "event_type":"pull_request",
    "filter":"ci_failed",
    "dedupe_key":"pr:jumo/lending-api#812:check:9912734:completed",
    "occurred_at":"2026-07-29T09:14:02Z",
    "observed_at":"2026-07-29T09:14:31Z",
    "summary":"CI failed on jumo/lending-api#812: job 'integration-tests' (2 steps failed)",
    "payload":{"check_run_id":9912734,"conclusion":"failure","failed_steps":["Run tests","Upload coverage"]},
    "payload_truncated":false
  }
}}
```

```json
{"jsonrpc":"2.0","id":907,"result":{"ack":true}}
```

The stream prints exactly one stdout line per delivered event, then responds.
Payloads are truncated daemon-side to 8 KiB before storage and delivery; the
summary line is always plain text with no untrusted content beyond names and
counts (§10.3).

### 7.2 `ping`

Sent by the daemon to stream connections every 15 s: `{"method":"ping"}` as a
request; response `{"result":{}}` within 5 s. Three consecutive misses: the
connection is closed as wedged and the session marked disconnected. Socket close
is always the primary liveness signal; ping only catches hung processes.

## 8. Delivery semantics

1. **At-least-once.** A delivery is acked only after the stream's response. No
   ack within 10 s: retry, max 3 attempts with 2 s / 8 s backoff, then the
   delivery is left pending and the connection is health-checked.
2. **Ordered per subscription.** The daemon serialises deliveries within one
   subscription (no next event until the previous is acked). Different
   subscriptions interleave freely.
3. **Resume flush.** On stream attach, all pending and unacked sent deliveries
   for the session replay in event-ULID order. The dedupe key lets any downstream
   consumer discard replays.
4. **Backpressure.** Max 100 undelivered events per subscription. Overflow drops
   the oldest (`dropped`, reason `overflow`) and enqueues one synthetic
   `stalk.notice` event stating how many were dropped.
5. **Disconnect grace.** When a session's stream disconnects without
   `session.close`, its subscriptions keep polling for the grace period (default
   10 min, config `daemon.disconnect_grace`). Expiry cancels them with reason
   `grace_expired`; a re-attach within the window resumes cleanly.
6. **TTL.** Deliveries pending for more than 24 h are marked `dropped` (reason
   `ttl`) by the retention job.

## 9. Daemon auto-spawn

Any client that fails to connect (`ENOENT`, `ECONNREFUSED`) may spawn the daemon:

1. Take an exclusive `flock` on `<runtime_dir>/spawn.lock` (non-blocking; if held,
   someone else is spawning: skip to step 3).
2. Start `stalk daemon` detached (new session, stdio to the log file).
3. Retry connect with 100 ms backoff, up to 5 s, then fail with a clear error.

The daemon itself takes an exclusive `flock` on `<runtime_dir>/daemon.lock` for
its whole lifetime (the singleton guarantee) and removes a stale socket file
before binding. It exits after `daemon.idle_exit_after` (default 30 min) with no
connected sessions and no active subscriptions.

## 10. Security

1. **No secrets on the wire.** Tokens are resolved daemon-side from the source
   config (env var name, `gh auth` chain, AWS credential chain). No method
   returns a credential; `sources.list` returns the auth *mode* only.
2. **Access control** is the socket directory mode plus the peer-uid check
   (§2.2). There is no multi-user mode.
3. **Untrusted payloads.** Event payloads are third-party text that will enter an
   agent context: the envelope keeps them in a distinct `payload` field,
   truncated to 8 KiB, and `summary` is composed daemon-side from structural
   fields only (repo names, counts, job names), never from free-text bodies.
   Consumers should treat `payload` as data, not instructions.
4. **Log hygiene.** The daemon log never prints tokens or full payloads.

## 11. Error codes

JSON-RPC standard codes apply (`-32700` parse, `-32600` invalid request,
`-32601` method not found, `-32602` invalid params, `-32603` internal).
Application codes:

| Code | Name | Meaning |
| --- | --- | --- |
| `-32000` | `VERSION_UNSUPPORTED` | Unknown protocol token; `data.supported` lists the known ones |
| `-32001` | `NOT_HANDSHAKEN` | Method before a successful handshake |
| `-32002` | `NOT_REGISTERED` | Session method before `session.register` |
| `-32003` | `UNKNOWN_SOURCE` | Source name not in config |
| `-32004` | `INVALID_TARGET` | Target fails source-type validation |
| `-32005` | `SOURCE_AUTH_UNAVAILABLE` | No usable credential for the source |
| `-32006` | `SUBSCRIPTION_NOT_FOUND` | Unknown or already-cancelled subscription |
| `-32007` | `LIMIT_EXCEEDED` | Per-session or per-daemon cap hit |
| `-32008` | `SHUTTING_DOWN` | Daemon is stopping; retry after reconnect |
| `-32009` | `RATE_LIMITED` | Connection message-rate cap exceeded |
| `-32010` | `FRAME_TOO_LARGE` | Frame over 256 KiB |

## 12. Sequence sketches

Startup and subscribe:

```text
stream ──connect──▶ daemon        mcp ──connect──▶ daemon
stream ──handshake─▶ ok           mcp ──handshake─▶ ok
stream ──session.register─▶ ok    mcp ──session.register─▶ ok (same aliases → same session)
                                  mcp ──subscribe(github-jumo, PR#812)─▶ id, poll starts
daemon ──(poll tick, new review)──▶ event stored + delivery created
daemon ──event.deliver──▶ stream ──stdout line──▶ Claude Code notification
stream ──result {ack}──▶ daemon   (delivery marked acked)
```

Daemon restart:

```text
daemon dies ──▶ streams' connections drop ──▶ reconnect loop (§9 spawns new daemon)
new daemon ──▶ loads sessions/subscriptions/poll_state from SQLite
streams ──register (resumed:true)──▶ pending deliveries flush, polling resumes from cursors
```
