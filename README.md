# go-template

[![CI](https://github.com/justanotherspy/go-template/actions/workflows/ci.yml/badge.svg)](https://github.com/justanotherspy/go-template/actions/workflows/ci.yml)
[![CodeQL](https://github.com/justanotherspy/go-template/actions/workflows/codeql.yml/badge.svg)](https://github.com/justanotherspy/go-template/actions/workflows/codeql.yml)
[![Release](https://img.shields.io/github/v/release/justanotherspy/go-template?sort=semver)](https://github.com/justanotherspy/go-template/releases)
[![Go Reference](https://pkg.go.dev/badge/github.com/justanotherspy/go-template.svg)](https://pkg.go.dev/github.com/justanotherspy/go-template)

A batteries-included template for building Go command-line tools, with CI,
linting, security scanning, and automated releases wired up from day one.

<!-- TEMPLATE:START -->
## Using this template

1. Click **Use this template** → **Create a new repository**.
2. A one-shot GitHub Action (`template-cleanup.yml`) automatically rewrites the
   module path, command directory, binary name, and `CODEOWNERS` to match your
   new repository, then deletes itself.
3. Wait for the `Initialize from template` action to finish and pull the
   resulting commit.
4. Start building in `internal/cli/`.

> If you cloned this repo manually instead of using the template button, you can
> run the same substitutions yourself by replacing `justanotherspy/go-template`
> (module path) and `go-template` (binary / command directory) throughout.
<!-- TEMPLATE:END -->

## Features

- **CLI** built on Cobra + Viper (config file + env var support).
- **Makefile** covering deps, tools, lint, format, test, build, run, security,
  and release tasks. `make help` lists everything.
- **golangci-lint v2** with a curated linter + formatter set.
- **`go fix` modernizers** (Go 1.26): `make modernize` rewrites code to current
  idioms (`any`, `minmax`, `rangeint`, …); CI enforces it stays modernized.
- **CI** (`ci.yml`): lint, a **`go fix` modernizer check**, test matrix
  (Go 1.26.x), build, govulncheck, and a `go mod tidy` check. Each PR gets a
  **sticky code-coverage comment** (and a job summary) generated from
  `go tool cover` — no third-party service or account.
- **Testing standards** (`internal/examples/`): native **fuzzing** (with a
  nightly `fuzz.yml` workflow), deterministic concurrency tests via
  **`testing/synctest`**, and **benchmarks/profiling** (`b.Loop`, pprof,
  benchstat). See [`CLAUDE.md`](CLAUDE.md#testing-fuzzing--profiling).
- **Security**: CodeQL (Go), Semgrep CE uploading SARIF to code scanning, and
  govulncheck.
- **Releases**: release-drafter + GoReleaser, driven by a `VERSION` file.
  `checksums.txt` is cosign-signed (keyless) and each archive ships an SPDX SBOM.
- **Distribution**: GoReleaser publishes a Homebrew cask to a shared tap, and
  `install.sh` downloads a checksum-verified binary in one line.
- **Dependabot** for Go modules and GitHub Actions, with update groups.
- All GitHub Actions **pinned to commit SHAs**.
- **Community health files**: `SECURITY.md`, `CONTRIBUTING.md`, and issue forms.
- `gopls` LSP, `CLAUDE.md`, and a curated [`.mcp.json`](#mcp-servers-claude-code)
  preconfigured for Claude Code, plus a project-scoped `go-lsp` plugin
  ([`.claude/skills/go-lsp/.lsp.json`](.claude/skills/go-lsp/.lsp.json)) that gives
  Claude Code real-time Go code intelligence (diagnostics, go-to-definition,
  references, hover) through gopls.

## Requirements

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
go-template            # prints help
go-template version    # prints version / build info
go-template --help
```

Configuration is read (in order of precedence) from flags, environment
variables prefixed with `GO_TEMPLATE_`, and an optional config file
(`--config`, default `$HOME/.go-template.yaml`).

## Install

Once your project has a published release, users can install it any of these
ways. (Repos created from the template inherit this wiring — the binary, cask,
and tap references are rewritten to match the new repo.)

### Homebrew (macOS and Linux)

```sh
brew install --cask justanotherspy/tap/go-template
```

Or tap once, then install by short name:

```sh
brew tap justanotherspy/tap
brew install --cask go-template
```

The cask is regenerated and pushed to
[`justanotherspy/homebrew-tap`](https://github.com/justanotherspy/homebrew-tap)
on every release (see [Releasing](#releasing)). Publishing requires a
`HOMEBREW_TAP_GITHUB_TOKEN` repo secret; without it the cask push is skipped and
the rest of the release still succeeds.

### Install script

Download a checksum-verified prebuilt binary (no Go toolchain needed):

```sh
curl -fsSL https://raw.githubusercontent.com/justanotherspy/go-template/main/install.sh | bash
```

Pin a version or target directory with environment variables (the prefix is the
binary name upper-cased):

```sh
curl -fsSL https://raw.githubusercontent.com/justanotherspy/go-template/main/install.sh \
  | GO_TEMPLATE_VERSION=v0.1.0 GO_TEMPLATE_INSTALL_DIR=/usr/local/bin bash
```

### From source

```sh
go install github.com/justanotherspy/go-template/cmd/go-template@latest
```

Prebuilt binaries are also on the
[releases](https://github.com/justanotherspy/go-template/releases) page. Verify
`checksums.txt` against its cosign bundle before trusting a download:

```sh
cosign verify-blob --bundle checksums.txt.sigstore.json \
  --certificate-identity-regexp '^https://github.com/justanotherspy/go-template' \
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
| `make fuzz FUZZ=Fuzz…` | Actively fuzz one target  |
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
| `github` | remote | Issues, PRs, CI status, code search on GitHub | Export `GITHUB_MCP_TOKEN` (a [fine-grained PAT](https://github.com/settings/personal-access-tokens)), or run `/mcp` to authenticate via OAuth |
| `linear` | remote | Find/create/update Linear issues & projects | Export `LINEAR_API_KEY` (Linear → Settings → Security & access). Drop the `Authorization` header to use `/mcp` OAuth instead |
| `context7` | remote | Up-to-date, version-specific library docs | Export `CONTEXT7_API_KEY` from [context7.com/dashboard](https://context7.com/dashboard) |
| `sprite` | remote | [sprites.dev](https://sprites.dev) agent sandboxes | Export `SPRITES_API_TOKEN` from the Sprites dashboard (or `sprite login`) |
| `semgrep` | local | Scan code for security vulnerabilities | Needs [`uv`](https://docs.astral.sh/uv/) (`uvx`); optional `SEMGREP_APP_TOKEN` for platform features |
| `fly` | local | Provision & manage Fly.io apps | Install [`flyctl`](https://fly.io/docs/flyctl/) and `fly auth login`; optional `FLY_ACCESS_TOKEN` |

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
