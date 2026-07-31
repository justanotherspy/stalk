# Contributing

Thanks for taking the time to contribute! This document covers the basics for
working in this repository.

## Getting started

```sh
make tools   # install pinned dev tooling (golangci-lint, goreleaser, gopls, …)
make ci      # deps + lint + modernize-check + test + build — what CI runs
```

Run `make help` for the full list of targets.

## Development workflow

1. Create a topic branch off `main`.
2. Make your change. Add or update tests where it makes sense.
3. Format, lint, and test locally before pushing:

   ```sh
   make fmt    # gofmt + goimports
   make lint   # golangci-lint
   make test   # tests with the race detector + coverage
   ```

4. Open a pull request, fill out the template, and make sure CI is green.

## Commit messages

Use clear, present-tense messages. Conventional-commit prefixes (`feat:`,
`fix:`, `docs:`, `chore:`, …) are encouraged — release-drafter uses them to
group changelog entries and pick the next version.

## Code style & conventions

- Go 1.26+. `GOTOOLCHAIN=auto` fetches the right toolchain on demand.
- Linux or macOS only — stalk relies on unix domain sockets, peer credentials,
  and `/proc` (or `sysctl` on macOS).
- Keep `make lint` clean; format with `make fmt` before committing.
- Add new subcommands under `internal/cli/`, one file per command, and register
  them in `newRootCmd` (`root.go`).
- Behaviour changes belong in the specs too: [`docs/PROTOCOL.md`](docs/PROTOCOL.md),
  [`docs/DB-SCHEMA.md`](docs/DB-SCHEMA.md), and [`docs/MVP-SPEC.md`](docs/MVP-SPEC.md)
  are the source of truth and should move in the same PR as the code.
- See [CLAUDE.md](CLAUDE.md) for the full layout and conventions.

## Reporting bugs & requesting features

Open an issue using one of the [issue templates][issues]. For security
vulnerabilities, follow the [security policy](SECURITY.md) instead of opening a
public issue.

[issues]: https://github.com/justanotherspy/stalk/issues/new/choose
