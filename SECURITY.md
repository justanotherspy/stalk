# Security Policy

## Supported versions

This project follows a rolling release model: only the latest release is
supported. Please upgrade before reporting an issue.

## Reporting a vulnerability

**Please do not open a public issue for security vulnerabilities.**

Report privately through GitHub's [security advisories][advisories] ("Report a
vulnerability" on the **Security** tab), or contact the maintainers listed in
[CODEOWNERS](.github/CODEOWNERS).

[advisories]: https://github.com/justanotherspy/go-template/security/advisories/new

Please include:

- a description of the issue and its impact,
- steps to reproduce or a proof of concept,
- the affected version(s).

We aim to acknowledge reports within a few days and will keep you updated on
remediation progress. Once a fix ships we're happy to credit you.

## Hardening already in place

- Dependencies and GitHub Actions are kept current by Dependabot; all actions
  are pinned to commit SHAs.
- CI runs CodeQL, Semgrep, secret scanning, and `govulncheck`.
- Release artifacts ship a `checksums.txt` signed with [cosign][cosign] and an
  SPDX SBOM per archive.

[cosign]: https://github.com/sigstore/cosign
