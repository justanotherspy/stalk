# stalk storage schema (SQLite, schema v1)

Status: draft v1 (2026-07-29).
Companions: [PROTOCOL.md](PROTOCOL.md) (`stalk/1`), [MVP-SPEC.md](MVP-SPEC.md) (scope).

This file is the source of truth. It supersedes the copy in Drive — edit it here.

The database is the daemon's **private** persistence. No other process opens it;
all access goes through the wire protocol (PROTOCOL.md). Source definitions and
credentials live in the YAML config, never in the DB; the DB references sources by
name only.

## 1. Engine settings

- Driver: `modernc.org/sqlite` (pure Go). GoReleaser builds with
  `CGO_ENABLED=0`, so the cgo `mattn/go-sqlite3` driver would break static
  cross-compiled release builds; the pure-Go driver preserves them.
- File: `$XDG_STATE_HOME/stalk/stalk.db` (fallback `~/.stalk/state/stalk.db`),
  mode `0600`.
- `PRAGMA journal_mode = WAL;` (readers never block the writer; safe crash
  recovery).
- `PRAGMA foreign_keys = ON;` on every connection.
- `PRAGMA synchronous = NORMAL;` (fine under WAL for this workload).
- All tables are `STRICT` (SQLite >= 3.37).
- IDs are ULIDs stored as `TEXT` (lexicographic order = creation order, which
  delivery ordering relies on).
- Timestamps are `INTEGER` unix epoch **milliseconds**, UTC. Column names end
  `_at`. Upstream-supplied times keep their own column (`occurred_at`) and may be
  NULL when the source does not provide one.
- Booleans are `INTEGER` 0/1.

## 2. DDL

```sql
-- schema v1

CREATE TABLE meta (
  key   TEXT PRIMARY KEY,
  value TEXT NOT NULL
) STRICT;
-- rows: ('schema_version','1'), ('daemon_instance','<ulid>'), ('created_at','<ms>')

CREATE TABLE sessions (
  session_key      TEXT PRIMARY KEY,          -- first alias seen (see session_aliases)
  status           TEXT NOT NULL CHECK (status IN ('active','disconnected','closed')),
  stream_attached  INTEGER NOT NULL DEFAULT 0,
  cwd              TEXT,
  client_version   TEXT,
  created_at       INTEGER NOT NULL,
  last_seen_at     INTEGER NOT NULL,
  closed_at        INTEGER
) STRICT;

CREATE TABLE session_aliases (
  alias        TEXT PRIMARY KEY,              -- 'anchor:<pid>:<start>' | 'ccsid:…' | 'manual:…'
  session_key  TEXT NOT NULL REFERENCES sessions(session_key) ON DELETE CASCADE
) STRICT;

CREATE INDEX idx_aliases_session ON session_aliases(session_key);

CREATE TABLE subscriptions (
  id             TEXT PRIMARY KEY,            -- ULID
  session_key    TEXT NOT NULL REFERENCES sessions(session_key) ON DELETE CASCADE,
  source_name    TEXT NOT NULL,               -- validated against config at runtime
  event_type     TEXT NOT NULL,               -- e.g. 'pull_request', 'http_items'
  target         TEXT NOT NULL,               -- canonical JSON, e.g. {"repo":"o/r","pr":812}
  filters        TEXT NOT NULL DEFAULT '[]',  -- canonical JSON array, sorted
  status         TEXT NOT NULL CHECK (status IN ('active','cancelled')),
  created_at     INTEGER NOT NULL,
  cancelled_at   INTEGER,
  cancel_reason  TEXT CHECK (cancel_reason IN
                   ('unsubscribed','session_closed','grace_expired','source_removed'))
) STRICT;

-- idempotent subscribe: one active row per identical tuple
CREATE UNIQUE INDEX idx_subs_identity
  ON subscriptions(session_key, source_name, event_type, target, filters)
  WHERE status = 'active';

CREATE INDEX idx_subs_by_source  ON subscriptions(source_name, status);
CREATE INDEX idx_subs_by_session ON subscriptions(session_key, status);

CREATE TABLE events (
  id                 TEXT PRIMARY KEY,        -- ULID, observed order
  source_name        TEXT NOT NULL,
  event_type         TEXT NOT NULL,
  dedupe_key         TEXT NOT NULL,           -- source-type specific, see §3
  occurred_at        INTEGER,                 -- upstream time, nullable
  observed_at        INTEGER NOT NULL,        -- daemon poll time
  summary            TEXT NOT NULL,           -- one plain-text line, daemon-composed
  payload            TEXT NOT NULL,           -- JSON, truncated to 8 KiB
  payload_truncated  INTEGER NOT NULL DEFAULT 0
) STRICT;

-- idempotent ingestion across cursor resets and daemon restarts:
CREATE UNIQUE INDEX idx_events_dedupe ON events(source_name, dedupe_key);
CREATE INDEX idx_events_recent ON events(observed_at);

CREATE TABLE deliveries (
  event_id         TEXT NOT NULL REFERENCES events(id) ON DELETE CASCADE,
  subscription_id  TEXT NOT NULL REFERENCES subscriptions(id) ON DELETE CASCADE,
  matched_filter   TEXT,                      -- which subscription filter matched
  status           TEXT NOT NULL CHECK (status IN ('pending','sent','acked','dropped')),
  attempts         INTEGER NOT NULL DEFAULT 0,
  last_attempt_at  INTEGER,
  acked_at         INTEGER,
  drop_reason      TEXT CHECK (drop_reason IN ('overflow','ttl','subscription_cancelled')),
  PRIMARY KEY (event_id, subscription_id)
) STRICT;

CREATE INDEX idx_deliveries_undone
  ON deliveries(subscription_id, status)
  WHERE status IN ('pending','sent');

-- One row per polled upstream unit of work. Several subscriptions can share a
-- row: this table IS the cross-session dedup ("one poll loop per (source,
-- target) regardless of subscriber count").
CREATE TABLE poll_state (
  source_name         TEXT NOT NULL,
  target              TEXT NOT NULL,          -- canonical JSON, same form as subscriptions.target
  etag                TEXT,                   -- conditional-request validator
  cursor              TEXT,                   -- JSON, source-type specific (see §4)
  interval_ms         INTEGER NOT NULL,
  last_poll_at        INTEGER,
  next_poll_at        INTEGER NOT NULL,
  backoff_until       INTEGER,
  consecutive_errors  INTEGER NOT NULL DEFAULT 0,
  last_http_status    INTEGER,
  PRIMARY KEY (source_name, target)
) STRICT;

CREATE INDEX idx_poll_due ON poll_state(next_poll_at);

CREATE TABLE poll_errors (
  id           INTEGER PRIMARY KEY,           -- rowid alias
  source_name  TEXT NOT NULL,
  target       TEXT NOT NULL,
  at           INTEGER NOT NULL,
  http_status  INTEGER,                       -- NULL for transport errors
  error        TEXT NOT NULL                  -- sanitised message, no tokens
) STRICT;

CREATE INDEX idx_poll_errors_recent ON poll_errors(source_name, at);
```

## 3. Dedupe keys (per source type)

The `(source_name, dedupe_key)` unique index makes ingestion idempotent even if a
cursor resets or the daemon restarts mid-poll. Conventions:

| Source type | Event | `dedupe_key` |
| --- | --- | --- |
| `github` | review submitted | `pr:<repo>#<n>:review:<review_id>` |
| `github` | review comment | `pr:<repo>#<n>:rcomment:<comment_id>` |
| `github` | issue comment | `pr:<repo>#<n>:comment:<comment_id>` |
| `github` | check run completed | `pr:<repo>#<n>:check:<check_run_id>:<attempt>` |
| `github` | PR state change | `pr:<repo>#<n>:state:<merged\|closed\|reopened>:<sha>` |
| `http_poll` | new item | `item:<id_field value>` (else `sha256:<payload>`) |
| (internal) | `stalk.notice` | `notice:<ulid>` (never deduped) |

## 4. Cursors (per source type)

`poll_state.cursor` is a JSON object owned by the source implementation:

- **`github` / `pull_request`**:
  `{"last_review_id":123,"last_comment_id":456,"last_check_poll_sha":"abc123","checks_settled":false}`
  plus `etag` for the list endpoints. Cursors minimise fetching; the dedupe index
  guarantees correctness when they lag or reset.
- **`http_poll`**: `{"seen_window":["id1","id2"]}` capped at 500 ids (ring); items
  falling out of the window are still caught by the dedupe index.

## 5. Lifecycle queries (reference)

Reconciliation at daemon start:

```sql
-- forget sockets that no longer exist; sessions are re-attached on register
UPDATE sessions SET stream_attached = 0, status = 'disconnected'
 WHERE status = 'active';

-- resume polling only where someone is listening
SELECT DISTINCT s.source_name, s.target
  FROM subscriptions s
  JOIN sessions ss ON ss.session_key = s.session_key
 WHERE s.status = 'active' AND ss.status != 'closed';
```

Next due poll:

```sql
SELECT * FROM poll_state
 WHERE next_poll_at <= :now
   AND (backoff_until IS NULL OR backoff_until <= :now)
 ORDER BY next_poll_at;
```

Fan-out on ingest (inside the ingest transaction):

```sql
INSERT OR IGNORE INTO events (id, source_name, event_type, dedupe_key, occurred_at,
                              observed_at, summary, payload, payload_truncated)
VALUES (:id, :src, :type, :dk, :occ, :obs, :summary, :payload, :trunc);
-- if changes() = 0 the event was seen before: stop.

INSERT INTO deliveries (event_id, subscription_id, matched_filter, status)
SELECT :id, s.id, :filter, 'pending'
  FROM subscriptions s
 WHERE s.status = 'active' AND s.source_name = :src
   AND s.event_type = :type AND s.target = :target
   AND (s.filters = '[]' OR EXISTS (
         SELECT 1 FROM json_each(s.filters) WHERE value = :filter));
```

Garbage collection: when the last active subscription for a `(source, target)`
pair is cancelled, delete its `poll_state` row and stop the poll loop.

## 6. Retention

Run hourly by the daemon (config: `daemon.retention.*`, defaults shown):

| Data | Rule |
| --- | --- |
| `events` (+ cascaded `deliveries`) | delete when older than 14 days, and cap at 50 000 rows (oldest first); rows with undone deliveries are skipped unless past TTL |
| `deliveries` pending > 24 h | mark `dropped`, reason `ttl` |
| `poll_errors` | delete when older than 7 days |
| `sessions` closed > 7 days | delete (cascades aliases, subscriptions) |

`PRAGMA incremental_vacuum` after large deletes. `PRAGMA auto_vacuum = INCREMENTAL`
MUST be executed in migration 001 **before the first table is created**: verified
empirically (SQLite 3.45) that setting it after any table exists is a silent
no-op until a full `VACUUM` is run.

## 7. Migration policy

`meta.schema_version` is checked at open. Migrations are forward-only, applied in
a transaction at daemon start (embedded in the binary, `migrations/NNN.sql`). A
downgrade (older binary, newer schema) refuses to start with a clear error rather
than guessing.
