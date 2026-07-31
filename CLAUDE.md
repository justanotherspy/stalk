# CLAUDE.md

Guidance for Claude Code (and humans) working in this repository.

## What this is

stalk is a per-user daemon that polls configured event sources (GitHub pull
requests, any HTTP JSON endpoint) and fans events out to Claude Code sessions.
A session gets events through `stalk stream` (a plugin monitor whose stdout
lines become notifications) and controls its own subscriptions through
`stalk mcp`. Built with [Cobra](https://github.com/spf13/cobra) and
[Viper](https://github.com/spf13/viper) on the go-template baseline, so CI,
linting, security scanning, and releases were wired up from commit one.

**Unix only** — unix domain sockets, peer credentials, and `/proc` (or `sysctl`
on macOS). There is no Windows build and no container image: a container cannot
reach the host's per-user socket, peer credentials, or process table, so PR 01
removed the Dockerfile and its workflow rather than ship something that only
looks like it works.

**Status: v1 in progress.** `daemon`, `stream`, `mcp`, `status`, and
`config check` are registered stubs that exit non-zero. `version` is the only
command that does anything.

## Specs

`docs/` holds the source of truth for v1. They supersede the copies in Drive —
edit them here, in the same PR as the code they describe.

| Document | Owns |
| -------- | ---- |
| [`docs/MVP-SPEC.md`](docs/MVP-SPEC.md) | Scope, sources, config schema, milestones, acceptance criteria |
| [`docs/PROTOCOL.md`](docs/PROTOCOL.md) | `stalk/1`: socket path, framing, methods, error codes, delivery semantics |
| [`docs/DB-SCHEMA.md`](docs/DB-SCHEMA.md) | SQLite schema v1: DDL, dedupe keys, cursors, retention, migrations |

## Layout

```
cmd/stalk/            main package; injects build info and calls internal/cli
cmd/gen-docs/         dev tool: renders shell completions + man pages
internal/cli/         command tree (root + subcommands), config loading
  root.go             root command, viper wiring, Execute (exit codes)
  stub.go             the shared "not implemented" RunE; deleted when the last stub goes
  daemon.go stream.go mcp.go status.go config.go version.go
  testdata/script/    testscript (txtar) CLI tests
docs/                 PROTOCOL.md, DB-SCHEMA.md, MVP-SPEC.md (source of truth)
.github/workflows/    CI, fuzz (nightly), CodeQL, Semgrep, secret-scan, zizmor,
                      scorecard, vuln, labeler, release-drafter, release
.github/ISSUE_TEMPLATE/  bug-report + feature-request issue forms
.github/labels.yml    canonical repo labels (synced by the labeler workflow)
.stalk.yaml.example   every config key, documented
.golangci.yml         golangci-lint v2 config (linters + formatters)
.goreleaser.yaml      GoReleaser v2 build/release config (linux + darwin only)
.mcp.json             MCP servers for dev sessions (github, linear, context7, …)
install.sh            checksum-verified prebuilt-binary installer (curl | bash)
SECURITY.md           security policy / private vulnerability reporting
CONTRIBUTING.md       contributor guide
VERSION               single source of truth for the next release version
Makefile              all developer + CI tasks
```

## Common commands

Run `make help` for the full list. The essentials:

| Command              | Purpose                                            |
| -------------------- | -------------------------------------------------- |
| `make deps`          | Download and verify modules                        |
| `make tools`         | Install pinned dev tools (lint, releaser, gopls…)  |
| `make check-tools`   | Verify required tools are installed                |
| `make lint`          | Run golangci-lint v2                               |
| `make fmt`           | Format (gofmt + goimports via golangci-lint)       |
| `make modernize`     | Apply go1.26 modernizers in place (`go fix`)       |
| `make modernize-check` | Report (don't apply) code `go fix` would modernize |
| `make test`          | Tests with race detector + coverage                |
| `make cover-report`  | Markdown coverage report (CI posts it on PRs)      |
| `make fuzz FUZZ=Fuzz…` | Actively fuzz one target (FUZZTIME, FUZZPKG)      |
| `make fuzz-all`      | Briefly fuzz every target (nightly workflow; no targets yet) |
| `make bench`         | Run benchmarks (BENCH, BENCHPKG, BENCHTIME)        |
| `make bench-save` / `make benchstat-cmp` | Sample benchmarks → compare with benchstat |
| `make profile`       | Write CPU+mem profiles for a benchmark             |
| `make pprof-cpu` / `make pprof-mem` | Open a profile in the pprof web UI  |
| `make build`         | Build to `./bin`                                   |
| `make run ARGS=...`  | Run the CLI                                        |
| `make vuln`          | govulncheck vulnerability scan                     |
| `make secrets`       | TruffleHog secret scan of the working tree         |
| `make zizmor`        | Audit GitHub Actions workflows (zizmor)            |
| `make actionlint`    | Lint workflows (+ shellcheck on run: blocks)       |
| `make release-check` | Validate `.goreleaser.yaml`                        |
| `make snapshot`      | Local snapshot build (no publish)                  |
| `make ci`            | What CI runs: deps + lint + modernize + test + build |

## Conventions

- Go **1.26+** (module floor is `go 1.26.0`; CI tests 1.26.x).
- `GOTOOLCHAIN=auto` — the correct Go toolchain is fetched on demand. The
  `toolchain` directive in `go.mod` pins a patched build toolchain (a bare
  `go 1.26.0` stdlib can carry vulnerabilities flagged by govulncheck); bump it
  when a newer patch fixes a reported issue.
- Lint must pass: `make lint`. Format with `make fmt` before committing.
- Code is kept modern with `go fix`: Go 1.26 rewrote `go fix` to run the
  [`modernize`](https://pkg.go.dev/golang.org/x/tools/go/analysis/passes/modernize)
  analyzer suite (e.g. `any`, `minmax`, `rangeint`, `slicescontains`,
  `stringscut`, `newexpr`). `make modernize` applies the fixes; CI runs
  `make modernize-check` (`go fix -diff`, which exits non-zero on any diff), so
  the tree must stay modernized. Run `go tool fix help` to list the fixers.
- All GitHub Actions are pinned to commit SHAs; Dependabot keeps them current.
- Add new subcommands under `internal/cli/`, one file per command, and register
  them in `newRootCmd`. A constructor you forget to register fails lint, not
  just the tests (`unused`).
- Commands that exist but aren't built yet use `RunE: notImplemented`
  (`internal/cli/stub.go`), which exits non-zero with the command path. When you
  implement one, drop that line; when the last one goes, delete `stub.go`.
- A command group (like `config`) must set **both** `RunE` and `Args`. Cobra
  skips argument validation entirely for a command that isn't runnable, so a
  group with no `RunE` prints help and exits **zero** on an unknown subcommand.
- Build metadata (`version`, `commit`, `date`) lives in `package main` and is
  injected via `-ldflags`. Update the user-facing version in the `VERSION` file.

## Testing, fuzzing & profiling

The template shipped an `internal/examples/` package demonstrating each
convention below; PR 01 deleted it, so the conventions are recorded here and the
tooling currently has nothing to chew on. Where each pattern lands:

| Pattern | Where it belongs |
| ------- | ---------------- |
| Fuzzing | The `stalk/1` NDJSON codec — framing edges, oversized frames, batch rejection |
| `testing/synctest` | Poll-scheduler backoff/jitter, and delivery retry/TTL timing |
| Benchmarks | Whatever turns out to be hot; nothing is yet |

- **Coverage on PRs.** `make test` writes `coverage.out`; `make cover-report`
  renders it as Markdown. CI (`ci.yml`) publishes that report to the job summary
  and posts/updates one sticky comment per PR via GitHub's REST API
  (`GITHUB_TOKEN` + `curl`/`jq` — no third-party action or SaaS account). It is
  **report-only**: coverage never fails the build. To gate later, compare
  `make cover-total` against a threshold in the workflow.
- **Fuzzing.** Write `FuzzXxx(f *testing.F)` targets next to the code; seed them
  with `f.Add(...)` for representative inputs and assert *invariants*, not fixed
  outputs. Seeds run as ordinary unit tests under `make test`, so invariants are
  checked on every PR. `make fuzz FUZZ=FuzzXxx` does active mutation fuzzing
  locally; the nightly **`fuzz.yml`** workflow runs `make fuzz-all`
  (auto-discovers every target, and is a no-op until the first one exists). A
  crasher is minimized into `testdata/fuzz/<FuzzXxx>/` — **commit it as a
  regression seed**, then fix the bug.
- **Concurrency with `testing/synctest`.** Stable since Go 1.25; the old
  `synctest.Run` was removed in 1.26 — always use **`synctest.Test(t, func(t
  *testing.T){…})`** with `synctest.Wait()`. It runs goroutines in an isolated
  "bubble" with a fake clock, so time-dependent concurrent code is deterministic
  and instant (no real sleeps, no flakes). Keep these at the unit layer: no real
  network, processes, or goroutines started outside the bubble.
- **CLI behaviour** is covered by testscript/txtar scripts in
  `internal/cli/testdata/script/`. They are auto-discovered — drop in a `.txtar`
  file and it runs. `! exec stalk foo` asserts a non-zero exit, `stderr '…'` the
  message, `! stdout .` that nothing reached stdout.
- **Benchmarks & profiling.** Use the modern **`for b.Loop() { … }`** form
  (Go 1.24+) with `b.ReportAllocs()`; it keeps setup out of the timed region and
  prevents dead-code elimination. `make bench` runs them; `make profile`
  captures CPU+mem profiles and `make pprof-cpu`/`pprof-mem` open them (the
  Go 1.26 pprof web UI defaults to the flame-graph view). For before/after
  comparisons, `make bench-save BENCHFILE=bench-old.txt` on the base revision,
  again as `bench-new.txt`, then `make benchstat-cmp`.
- **Go 1.26 goroutine-leak profile (experimental).** Build/run with
  `GOEXPERIMENT=goroutineleakprofile` to enable the `goroutineleak` profile in
  `runtime/pprof` (and the `/debug/pprof/goroutineleak` endpoint) — it reports
  goroutines blocked on primitives that can never unblock. Handy when chasing a
  leak; pair it with `make test` (which already runs the race detector).

## Tooling / LSP

`gopls` is the Go language server. `make lsp` installs it, and a `SessionStart`
hook in `.claude/settings.json` installs it automatically for web sessions.

**Claude Code code intelligence.** `.claude/skills/go-lsp/` is a project-scoped
[skills-directory plugin](https://code.claude.com/docs/en/plugins-reference#skills-directory-plugins)
that wires `gopls` into Claude Code via its [`.lsp.json`](.claude/skills/go-lsp/.lsp.json)
config. Checked into the repo, it loads automatically as `go-lsp@skills-dir` for
anyone who clones it (after accepting the workspace-trust prompt — project-scope
LSP servers start only once the workspace is trusted), giving Claude live
diagnostics after edits, go-to-definition, find-references, and hover/type info.
It only configures the connection; the `gopls` binary itself comes from `make lsp`
/ the `SessionStart` hook. Run `/reload-plugins` after editing the config.

## Release process

1. Merge PRs into `main`. release-drafter keeps a **draft** release updated; its
   version/tag come from the `VERSION` file (`v<VERSION>`).
2. To cut a release, edit the draft and publish it **as a pre-release**.
3. Publishing as pre-release triggers `release.yml`: it runs lint + tests, then
   GoReleaser builds binaries and appends them to the release. It also:
   - signs `checksums.txt` with **cosign** (keyless OIDC — the job has
     `id-token: write`); verify with the published `checksums.txt.sigstore.json`,
   - generates an **SPDX SBOM** per archive (via `syft`, installed in the job),
   - pushes a **Homebrew cask** to `justanotherspy/homebrew-tap` when the
     `HOMEBREW_TAP_GITHUB_TOKEN` secret is set (skipped otherwise, so a missing
     token never fails a release).
4. On success the release is automatically promoted (pre-release flag cleared,
   marked "latest").

To release a new version, bump `VERSION` on `main` first so the draft picks up
the new number.

### Distribution / cask

- Release artifacts are **linux and darwin, amd64 and arm64** only. Adding
  `windows` back to `.goreleaser.yaml` would produce binaries that cannot run.
- Each archive bundles `docs/`, the generated `completions/` and `man/`, plus
  LICENSE and README. Those two generated directories come from the GoReleaser
  `before` hook (`go run ./cmd/gen-docs`) — don't commit them.
- `homebrew_casks` in `.goreleaser.yaml` generates the cask; the tap owner is
  `justanotherspy`. The cask name, binary, homepage, and url track the repo name.
- `install.sh` is a standalone `curl | bash` installer that downloads the
  matching release archive, verifies it against `checksums.txt`, and installs
  the binary. It refuses to run anywhere but Linux and macOS. Its override env
  vars (`STALK_VERSION` / `STALK_INSTALL_DIR`) track the binary name, matching
  the `viper` env prefix.
- `make snapshot` builds locally with `--skip=sign,sbom`, so cosign and syft are
  only needed in CI.
