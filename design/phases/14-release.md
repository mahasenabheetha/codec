# 14 — v2.0.0 release

**Goal:** polish everything to a stable, shareable 2.0.
**Depends on:** 13 · **Branch:** `feature/mab/yaml-tools`

## Scope

- Polish pass against ui.md principles: consistency, empty/error
  states, loading skeletons, keyboard coverage, palette completeness.
- Settings screen (editor link target, lint rules, schema/offline,
  K8s version, profiles management).
- First-run onboarding with an embedded sample workspace (no repo needed).
- Performance: large repo (10k files), 5 MB file, 500-doc render —
  budgets met, measured, reported.
- Accessibility: contrast, focus order, screen-reader labels on controls.
- Cross-platform check: Windows (no admin), macOS arm64, Linux, Docker.
- README + screenshots/GIF, CHANGELOG, `design/` docs refreshed.
- After merge, user tags `v2.0.0`; Docker `latest` updated.

## Budgets and results

Measured on the user's Windows machine (22 cores) with
`scripts/perf.sh` (API) and the UI in Chrome; fixtures from
`scripts/perfgen`.

| Case | Budget | Measured |
|---|---|---|
| 10k files: folder open + tree | ≤ 1 s | 0.7–0.9 s (scan 0.3 s) |
| 10k files: every file typed | ≤ 10 s | 1.4–2.2 s (cold disk: ~9 s) |
| 10k files: Go to file | ≤ 100 ms per key | 13–62 ms |
| 10k files: workspace lint | ≤ 30 s | 6.2–7.0 s |
| 5 MB file: editor shows it | ≤ 2 s | 0.9 s |
| 5 MB file: analysis done | ≤ 3 s | 2.0 s (server 0.7–0.8 s) |
| 5 MB file: typing | ≤ 16 ms per key | 6.8 ms avg |
| 500-doc render | ≤ 2 s | 0.12–0.19 s; view 0.48 s |
| First chart render, empty schema cache | ≤ 2 s | 1.1 s (findings follow) |

Changes that got there: parallel document parsing, trimmed outlines,
windowed trees, schema downloads started together, non-blocking schema
checks on renders (decisions #70–#71).

## Acceptance

- [ ] No open bugs labelled for 2.0. *(The user checks the tracker.)*
- [x] All budgets met on the user's machine. *(Table above; macOS not
      measured.)*
- [x] A new user can go from download to rendered chart in < 2 minutes.
      *(`codec serve --open` → Open the sample → chart rendered in ~2 s.)*

Cross-platform: Windows native without admin rights; Linux: the image
run with a read-only mount and with `--sample` (the full suite runs on
Linux in CI; a local `-race` run was stopped before it finished);
macOS arm64 cross-compiles, run it natively before tagging. The GIF was not made (no recorder here); README has a
new first-run screenshot.

**Next:** v3 (Wails desktop app) — plan in a new phase set.
