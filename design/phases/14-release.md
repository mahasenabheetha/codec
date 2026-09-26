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

## Acceptance

- [ ] No open bugs labelled for 2.0.
- [ ] All budgets met on the user's machine.
- [ ] A new user can go from download to rendered chart in < 2 minutes.

**Next:** v3 (Wails desktop app) — plan in a new phase set.
