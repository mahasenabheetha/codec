# 05 — Helm

**Goal:** render any chart in the workspace with layered values, see
exactly where every value came from, and preview what-if changes live.
**Depends on:** 04 · **Branch:** `feature/mab/yaml-tools`
**Read:** decisions.md (#9), architecture.md

## Scope

Engine `internal/helm` (content in, results out — no disk access):
- Load chart from in-memory files via the Helm 4 SDK loader; render
  with values layers, `--set` overrides, release name, namespace,
  kube version / API versions.
- **Values provenance:** own layered merge mirroring Helm's coalesce
  rules (maps deep-merge, lists/scalars replace, `null` deletes,
  subchart scoping, `global`) → merged values + per-path source chain
  (file, line). Tests assert our merged result equals the SDK's.
- Rendered docs keep their template origin (`# Source:`); template
  errors map to template file:line.
- Reference check: `.Values` paths used in templates but defined
  nowhere (warning); defined but never used (info).
- Vendored dependencies in `charts/` (dirs or `.tgz`) work; missing
  ones produce a clear diagnostic (no network fetch).

Adapter + API:
- Workspace finds chart roots and candidate values files.
- Profiles in user config: per chart, named env → ordered values files + `--set`.
- `GET /api/v2/helm/charts`; `POST /api/v2/helm/render` {chart,
  values[], set[], overrides{path: content}, release, namespace,
  kubeVersion} → {docs[{source, content}], values, provenance, diagnostics}.

UI (Helm view per chart):
- Values layer list (reorder, toggle), `--set` input, profile picker.
- Editor on a values file or template (what-if) ⇄ rendered output
  grouped by template; live re-render (debounced ~200 ms).
- Hover a value or rendered field → provenance chain, e.g.
  `3 ← values-prod.yaml:12 (overrides 1 ← values.yaml:4)`.
- Merged-values tab with provenance gutter; errors panel.

CLI: `codec helm render <chart> -f … --set …`,
`codec helm values <chart> -f … --provenance`.

## Acceptance

- [ ] Output identical to `helm template` of the same Helm version on
      fixture charts (verify manually, e.g. `docker run --rm -v "$PWD:/c"
      alpine/helm:<ver> template /c/<chart> -f …`; command documented).
- [ ] Provenance correct for lists, `null` deletion, subcharts, globals.
- [ ] Typical chart renders < 300 ms; binary size before/after in PR.

## Notes

Verify Helm 4 SDK package paths/APIs first. `lookup` returns empty in
template mode — say so in the UI.

**Go concepts:** consuming a large SDK, recursion over `map[string]any`, type switches, interfaces for testability.
