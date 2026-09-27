# Conventions

## Go

- **The engine decides what things are; adapters decide how they look.**
  Engine packages are pure and fully unit-tested; see
  [architecture.md](architecture.md) for the package map.
- `cmd/codec/main.go` only calls `cli.Execute()`.
- Errors: wrap with `fmt.Errorf("context: %w", err)`; typed errors when
  callers must branch (`errors.Is/As`), like `codec.ErrUnknownMode`.
- Comments explain *why*; doc comments on every exported identifier.
- Minimal dependencies; justify each new module in the PR description.
- Cross-platform: `filepath` for OS paths, `path` for slash paths in
  APIs; never assume case-sensitive filenames; preserve CRLF/BOM when
  displaying files.

## CLI

- Data on stdout, notices/warnings on stderr.
- Exit codes: `0` ok, `1` error/usage, `2` findings (lint issues, diff
  differences).
- New commands: `codec yaml|helm|k8s|argo …`; every engine feature
  reachable from the CLI where it makes sense.

## Frontend

- Svelte 5 runes (`$state`, `$derived`, `$props`), TypeScript strict.
- Structure: `frontend/src/lib/{api,components,shell,stores,styles,utils}`
  for shared code; `frontend/src/features/<feature>/` per tool/lens, with
  cross-tool pieces in `features/shared/`. New tools register in
  `lib/tools.ts` (rail, tabs, home and palette are generated from it).
- Styling: only design tokens from [design-language.md](design-language.md) — no hard-coded
  colors, sizes or fonts in components. Shared primitives in
  `lib/components` before any feature-specific variant.
- Dependencies allowed: CodeMirror 6 packages, `@lucide/svelte` (import
  per icon: `@lucide/svelte/icons/<name>`), Bits UI (headless primitives),
  `@fontsource-variable/*`. Anything else needs a reason.
- Tool content the user pastes is never persisted (it may hold secrets);
  `persisted()` is for UI preferences only.
- No runtime network calls to third parties. All assets bundled.

## Testing

- Table-driven tests with `t.Run`; real (trimmed, anonymized) fixtures
  in `testdata/`. Every fixed bug gets a named regression case.
- Test engine logic, not adapters' plumbing. Frontend: no unit tests
  unless logic is non-trivial (then Vitest on plain TS modules).
- Run only affected packages while working; full `go test ./...` once
  before a PR. The user does manual/UI testing.
- CI: vet, test, build, Windows job, frontend build, `goreleaser check`.

## Git

- Never commit to `main`. Work on a short-lived branch
  (`feature/mab/<topic>`, `fix/<topic>`), then open a pull request; CI
  must be green; the user merges. (v2 was built on the integration
  branch `feature/mab/yaml-tools` until it merged [16, 72].)
- One logical change per commit, message in the imperative
  ("Fix the Resources view when…"), body saying why.
- AI-assisted commits carry a `Co-Authored-By` trailer; README
  Acknowledgements disclose AI assistance.

## Docs

Every user-visible change updates, in the same pull request:
`CHANGELOG.md` `[Unreleased]`; the matching page in `docs/` (and its
screenshot if the view changed); the README feature list if a feature
was added; `design/` files whose facts changed. See
[workflow.md](workflow.md#what-to-update) for the full matrix.
