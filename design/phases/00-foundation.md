# 00 — Foundation

**Goal:** v2 toolchain in place without changing any user-visible v1 behaviour.
**Depends on:** v1.0.0 · **Branch:** `feature/mab/p00-foundation`
**Read:** architecture.md (layout), conventions.md

## Scope

- Module path → `github.com/mahasenabheetha/codec/v2`; update imports.
- Bump `go.mod` to a currently supported Go (≥ 1.26); CI and release
  use `go-version-file: go.mod`.
- `frontend/`: Vite + Svelte 5 + TypeScript (strict) + `svelte-check`.
  `npm run build` outputs to `internal/web/dist`; `npm run dev` proxies
  `/api` to `127.0.0.1:8765`.
- Serve the new app at `/app/` (Vite `base: '/app/'`); v1 UI stays at `/`
  until phase 01 swaps them.
- Dev container: add Node LTS feature.
- CI: build frontend before Go steps; add a `windows-latest` job (vet,
  test, build). Keep `goreleaser check`.
- GoReleaser `before.hooks`: `npm --prefix frontend ci` and `run build`.
  Release workflow gets `actions/setup-node`.

## Out of scope

Any UI design, components, YAML features.

## Acceptance

- [ ] Fresh clone: `go build ./...` compiles even without a frontend
      build (placeholder page), and serves the real app after `npm run build`.
- [ ] `codec serve`: v1 UI at `/`, Svelte hello page at `/app/`, all v1
      API/CLI behaviour unchanged.
- [ ] `npm run dev` hot-reloads and reaches the Go API.
- [ ] CI green on Linux and Windows; `goreleaser build --snapshot` includes the frontend.

## Notes

- `go:embed` fails on an empty dir: use `//go:embed all:dist` with a
  committed `internal/web/dist/.keep`, gitignore the rest of `dist/`,
  and make sure the Vite build doesn't delete `.keep` (or the tree goes
  dirty and GoReleaser refuses to release).
- If `index.html` is missing from dist, serve a "frontend not built" page.

**Go concepts:** major-version module paths, `go:embed` patterns, build tags vs. placeholders.
