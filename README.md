# stalk

[![CI](https://github.com/justanotherspy/stalk/actions/workflows/ci.yml/badge.svg)](https://github.com/justanotherspy/stalk/actions/workflows/ci.yml)
[![CodeQL](https://github.com/justanotherspy/stalk/actions/workflows/codeql.yml/badge.svg)](https://github.com/justanotherspy/stalk/actions/workflows/codeql.yml)
[![Release](https://img.shields.io/github/v/release/justanotherspy/stalk?sort=semver)](https://github.com/justanotherspy/stalk/releases)
[![Go Reference](https://pkg.go.dev/badge/github.com/justanotherspy/stalk.svg)](https://pkg.go.dev/github.com/justanotherspy/stalk)

stalk watches things on behalf of your Claude Code sessions.

One per-user daemon polls the sources you configure — GitHub pull requests, any
HTTP JSON endpoint — and fans events out to every session that asked for them.
Each session runs a thin `stalk stream` process, whose stdout lines become
Claude Code notifications, and a `stalk mcp` server that lets the agent
subscribe and unsubscribe on its own.

One poll loop per watched thing no matter how many sessions care. Credentials,
backoff, rate limits, and cleanup live in the daemon, not in any session's
lifecycle.

> **v1 is under construction.** The command tree is in place but the
> subcommands below are stubs that exit non-zero — only `stalk version` does
> anything today. The specs in [`docs/`](#docs) are the source of truth for what
> is being built.

## Docs

| Document | What's in it |
| -------- | ------------ |
| [`docs/MVP-SPEC.md`](docs/MVP-SPEC.md) | Scope, sources, config, milestones, acceptance criteria |
| [`docs/PROTOCOL.md`](docs/PROTOCOL.md) | The `stalk/1` wire protocol: socket, framing, methods, delivery semantics |
| [`docs/DB-SCHEMA.md`](docs/DB-SCHEMA.md) | SQLite schema v1: DDL, dedupe keys, cursors, retention |
| [`CLAUDE.md`](CLAUDE.md) | Repo layout, commands, and conventions |
| [`CONTRIBUTING.md`](CONTRIBUTING.md) | Development workflow |
| [`SECURITY.md`](SECURITY.md) | Reporting a vulnerability |

## How it fits together

```text
Claude Code session ──(plugin monitor)──▶ stalk stream ──UDS──▶
Claude Code session ──(plugin MCP)─────▶ stalk mcp    ──UDS──▶  stalk daemon ──HTTPS──▶ GitHub / endpoints
                                         stalk status ──UDS──▶      │
                                                                 SQLite
```

The daemon owns all state. Clients hold none, connect over a unix domain socket
in the user's runtime directory, and are auto-spawned on demand. There is no TCP
listener and no multi-user mode.

## Requirements

- Linux or macOS. stalk is built on unix domain sockets, peer credentials, and
  `/proc` (or `sysctl` on macOS); there is no Windows build.
- Go 1.26 or newer (`GOTOOLCHAIN=auto` will fetch the right toolchain).
- `make`.

## Quick start

```sh
make tools     # install dev tooling (golangci-lint, goreleaser, gopls, …)
make ci        # deps + lint + modernize-check + test + build
make run ARGS="version"
```

## Usage

```sh
stalk                 # prints help
stalk version         # prints version / build info
stalk --help
```

The v1 command tree, all of it stubbed out for now:

| Command | What it will do |
| ------- | --------------- |
| `stalk daemon` | Run the per-user daemon: socket, SQLite state, poll loops |
| `stalk stream` | Print this session's events to stdout, one line each (run as a plugin monitor) |
| `stalk mcp` | Serve the subscribe/unsubscribe MCP tools to the agent over stdio |
| `stalk status` | Report daemon uptime, sessions, subscriptions, and poll health |
| `stalk config check` | Validate the resolved configuration |

Each of these exits non-zero with a "not implemented" message until the PR that
builds it lands. See [`docs/MVP-SPEC.md`](docs/MVP-SPEC.md) §7 for the
milestones.

Configuration is read (in order of precedence) from flags, environment
variables prefixed with `STALK_`, and an optional config file
(`--config`, default `$HOME/.stalk.yaml`). See
[`.stalk.yaml.example`](.stalk.yaml.example) for every key.

## Install

There is nothing to install yet — the first release is still ahead. Once it
lands, any of these will work.

### Homebrew (macOS and Linux)

```sh
brew install --cask justanotherspy/tap/stalk
```

Or tap once, then install by short name:

```sh
brew tap justanotherspy/tap
brew install --cask stalk
```

The cask is regenerated and pushed to
[`justanotherspy/homebrew-tap`](https://github.com/justanotherspy/homebrew-tap)
on every release (see [Releasing](#releasing)). Publishing requires a
`HOMEBREW_TAP_GITHUB_TOKEN` repo secret; without it the cask push is skipped and
the rest of the release still succeeds.

### Install script

Download a checksum-verified prebuilt binary (no Go toolchain needed):

```sh
curl -fsSL https://raw.githubusercontent.com/justanotherspy/stalk/main/install.sh | bash
```

Pin a version or target directory with environment variables (the prefix is the
binary name upper-cased):

```sh
curl -fsSL https://raw.githubusercontent.com/justanotherspy/stalk/main/install.sh \
  | STALK_VERSION=v0.1.0 STALK_INSTALL_DIR=/usr/local/bin bash
```

### From source

```sh
go install github.com/justanotherspy/stalk/cmd/stalk@latest
```

Prebuilt binaries are also on the
[releases](https://github.com/justanotherspy/stalk/releases) page. Verify
`checksums.txt` against its cosign bundle before trusting a download:

```sh
cosign verify-blob --bundle checksums.txt.sigstore.json \
  --certificate-identity-regexp '^https://github.com/justanotherspy/stalk' \
  --certificate-oidc-issuer 'https://token.actions.githubusercontent.com' \
  checksums.txt
```

## Development

| Command         | Purpose                          |
| --------------- | -------------------------------- |
| `make lint`     | Run golangci-lint                |
| `make fmt`      | Format code                      |
| `make modernize`| Apply `go fix` modernizers       |
| `make test`     | Run tests with race + coverage   |
| `make cover-report` / `make cover-total` | Markdown coverage report / total % |
| `make fuzz FUZZ=Fuzz…` | Actively fuzz one target (none yet — the wire codec brings the first) |
| `make fuzz-all` | Briefly fuzz every target        |
| `make bench`    | Run benchmarks                   |
| `make bench-save` / `make benchstat-cmp` | Sample benchmarks → compare with benchstat |
| `make profile`  | CPU+mem profile a benchmark      |
| `make pprof-cpu` / `make pprof-mem` | Open a profile in the pprof web UI |
| `make build`    | Build the binary into `./bin`    |
| `make vuln`     | Vulnerability scan (govulncheck) |
| `make snapshot` | Local GoReleaser snapshot build  |

See [`CLAUDE.md`](CLAUDE.md) for the full layout and conventions.

## MCP servers (Claude Code)

[`.mcp.json`](.mcp.json) registers a curated set of [Model Context
Protocol](https://modelcontextprotocol.io) servers for Claude Code. They load
automatically when you open the repo in Claude Code (run `/mcp` to authenticate
or check status). Secrets are never committed — each server reads its token from
an environment variable, so export the ones you use and skip the rest (a server
with a missing token simply won't connect; the others still work).

| Server | Type | What it's for | Setup |
| ------ | ---- | ------------- | ----- |
| `github` | remote | Issues, PRs, CI status, code search on GitHub | Export `GITHUB_TOKEN` (a [fine-grained PAT](https://github.com/settings/personal-access-tokens)), or run `/mcp` to authenticate via OAuth |
| `linear` | remote | Find/create/update Linear issues & projects | Export `LINEAR_API_KEY` (Linear → Settings → Security & access). Drop the `Authorization` header to use `/mcp` OAuth instead |
| `context7` | remote | Up-to-date, version-specific library docs | Export `CONTEXT7_API_KEY` from [context7.com/dashboard](https://context7.com/dashboard) |
| `sprite` | remote | [sprites.dev](https://sprites.dev) agent sandboxes | Export `SPRITES_TOKEN` from the Sprites dashboard (or `sprite login`) |
| `semgrep` | local | Scan code for security vulnerabilities | Needs [`uv`](https://docs.astral.sh/uv/) (`uvx`); optional `SEMGREP_APP_TOKEN` for platform features |
| `fly` | local | Provision & manage Fly.io apps | Install [`flyctl`](https://fly.io/docs/flyctl/) and `fly auth login`; optional `FLY_API_TOKEN` |
| `shuck` | local | Failing-CI drill-down for a PR (see [Claude plugins](#claude-plugins)) | Install the [`shuck`](https://github.com/justanotherspy/shuck) binary; uses the same `GITHUB_TOKEN` |

Remove any server you don't want by deleting its entry from `.mcp.json`.

## Claude plugins

[`.claude/settings.json`](.claude/settings.json) enables the
[`shuck`](https://github.com/justanotherspy/shuck) plugin from the central
[`justanotherspy/claude-plugins`](https://github.com/justanotherspy/claude-plugins)
marketplace, so it loads automatically when you open the repo in Claude Code.
`shuck` extracts the exact failing CI step logs for a PR (plus PR reviews and
security alerts) via a `/shuck` skill and a local MCP server; it needs the
[`shuck`](https://github.com/justanotherspy/shuck) binary on your `PATH`.

## Releasing

1. Bump [`VERSION`](VERSION) on `main`.
2. release-drafter maintains a draft release tagged `v<VERSION>`.
3. Publish the draft **as a pre-release** — this triggers tests, lint, and a
   GoReleaser build that attaches binaries, signs `checksums.txt` with cosign,
   generates per-archive SBOMs, and (if `HOMEBREW_TAP_GITHUB_TOKEN` is set)
   pushes the Homebrew cask, then auto-promotes the release to "latest".

To enable cask publishing, add a `HOMEBREW_TAP_GITHUB_TOKEN` repository secret —
a PAT with `contents:write` on the tap repo. The push is skipped when the secret
is absent, so releases never fail just because cask publishing isn't configured.

## License

[MIT](LICENSE)
