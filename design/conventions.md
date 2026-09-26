# Conventions

## Architecture

- **The engine decides what things are; presentation layers decide what
  they look like.**
- Engine packages (`internal/codec`, future `internal/yamlkit` etc.) are
  pure: no I/O, no CLI/HTTP knowledge, fully unit-tested.
- Adapters are thin: `internal/cli` (cobra), `internal/web` (HTTP +
  `go:embed` frontend). Anything doing I/O (files, running binaries)
  lives in an adapter package, never the engine.
- `cmd/codec/main.go` only calls `cli.Execute()`.

## Stack

- Go (version in `go.mod`), cobra for the CLI.
- Web frontend embedded in the binary, no runtime dependencies. Server
  binds to `127.0.0.1` only.

## CLI behaviour

- Data on stdout, notices/warnings on stderr.
- Exit codes: `0` success, `1` invalid input or usage error.

## Testing

- Table-driven tests with `t.Run` subtests.
- Use real (trimmed, anonymized) inputs as fixtures; every fixed bug
  becomes a named regression test.
- CI runs `go vet`, `go test`, `go build`, a Windows cross-compile, and
  `goreleaser check` on every push and PR.

## Git workflow (non-negotiable)

- Never commit directly to `main`.
- Work on a feature branch (`feature/mab/<topic>`), open a PR, CI must
  pass, merge with a merge commit.
- Only the repo owner merges, unless an agent is explicitly told to.
- Commits made with AI assistance carry a `Co-Authored-By` trailer; the
  README's Acknowledgements section discloses AI assistance.
