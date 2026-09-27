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
