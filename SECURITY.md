# Security policy

codec runs a local web server that can read the folders you open, so
security reports are taken seriously.

## Reporting a vulnerability

Please **don't open a public issue**. Report it privately through
GitHub's [private vulnerability reporting](https://github.com/mahasenabheetha/codec/security/advisories/new)
with steps to reproduce and the output of `codec version`. You'll get
an answer as soon as possible, and credit in the release notes if you
want it.

## Supported versions

Fixes go into the latest release. Older major versions get security
fixes only when a `release/vN` branch exists.

## Security model

How codec protects your files (loopback-only binding, host and origin
checks, a per-run token, confined file access, no telemetry) is
described in [docs/security.html](docs/security.html).

## How the code is checked

Every change runs the tests (with the race detector), staticcheck and
govulncheck in CI, and GitHub's CodeQL code scanning on the Go server,
the frontend and the workflows (also weekly). OpenSSF Scorecard rates
the project's security practices weekly; its score is the badge in the
README. Parsers are fuzz-tested, and dependency updates come from
Dependabot. The workflows' tokens are read-only except where a job
writes, and every action is pinned to a commit hash.

## Verifying a download

Each release's `checksums.txt` is signed with cosign (keyless, by the
release workflow; the bundle is `checksums.txt.sigstore.json`), and
every archive and desktop file has GitHub build provenance:

```bash
gh attestation verify codec_<version>_linux_amd64.tar.gz -R mahasenabheetha/codec
cosign verify-blob checksums.txt --bundle checksums.txt.sigstore.json \
  --certificate-identity-regexp '^https://github.com/mahasenabheetha/codec/' \
  --certificate-oidc-issuer https://token.actions.githubusercontent.com
```
