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

- [x] ~2k-file repo: tree shown < 1 s; disk change visible < 1 s (native).
      Measured on merge-delivery (5,377 files, OneDrive): open 0.5 s warm
      (1.1 s cold), watches added in the background,
      tree 74 ms, types ~0.4 s later; change visible ~0.2 s.
- [x] Tests: bad Host rejected, missing token rejected, `..` and symlink
      escape rejected (`web/security_test.go`, `workspace_test.go`;
      symlink cases skip on Windows without symlink rights, run in CI).
- [~] Works on Windows paths (drives, backslashes) and in Docker (`--root /work`, polling).
      Windows paths/drives and `--poll` verified; Docker not yet run
      (Docker Desktop was stopped) — check in phase 06 with the image.
- [x] No file is ever written under the workspace root (snapshot test;
      settings go to the user profile).

## Notes

OneDrive folders emit noisy events and temporary locks: debounce
~200 ms and tolerate transient read errors (reads retry; unreadable
directories keep their last known files).

Deviations, see decisions #23–#28: polling is automatic in containers
(not with `--root`); tree before types; direct reads for classification.

**Go concepts:** `fs.WalkDir`, goroutines + channels, `context` cancellation, HTTP middleware, SSE, `sync.RWMutex`.
