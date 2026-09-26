# 03 — Workspace

**Goal:** open any repo folder read-only, browse it, view files.
**Depends on:** 01, 02 · **Branch:** `feature/mab/yaml-tools`
**Read:** architecture.md (security, runtime modes), ui.md (layout)

## Scope

Backend:
- `internal/config`: JSON settings in `os.UserConfigDir()/codec`
  (recent folders, preferences). Never write inside workspaces.
- `internal/workspace`: open root; tree listing that honours
  `.gitignore`, skips `.git`, `node_modules`, binaries and files > 2 MB;
  read file; classify via provider registry; lazy parse cache keyed by
  mtime+size; watcher (fsnotify, recursive, debounced) with polling
  fallback (`--poll`, automatic with `--root`).
- Security middleware exactly per architecture.md (Host check, token,
  Origin, root confinement incl. symlink escapes).
- `serve` flags: `--root`, `--host`, `--poll`, `--open` (launch browser).
- API: `GET /api/v2/workspace`, `POST /api/v2/workspace/open`,
  `GET /api/v2/fs/dirs?path=` (folder browser; Windows drive letters),
  `GET /api/v2/files/tree`, `GET /api/v2/files/content?path=`,
  `GET /api/v2/events` (SSE: file changed/added/removed).

Frontend:
- Open-folder dialog: path input, folder browser, recents.
- File tree with type badges + logos, type filter, `Ctrl+P` fuzzy file search.
- Open file in a tab: read-only CodeView with YAML highlighting;
  auto-refresh on disk change.

## Out of scope

Diagnostics, outline, hover, editing (04).

## Acceptance

- [ ] ~2k-file repo: tree shown < 1 s; disk change visible < 1 s (native).
- [ ] Tests: bad Host rejected, missing token rejected, `..` and symlink
      escape rejected.
- [ ] Works on Windows paths (drives, backslashes) and in Docker (`--root /work`, polling).
- [ ] No file is ever written under the workspace root.

## Notes

OneDrive folders emit noisy events and temporary locks: debounce
~200 ms and tolerate transient read errors.

**Go concepts:** `fs.WalkDir`, goroutines + channels, `context` cancellation, HTTP middleware, SSE, `sync.RWMutex`.
