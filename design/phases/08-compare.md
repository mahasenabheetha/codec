# 08 — Compare + query

**Goal:** answer "what's different?" and "where is X?" precisely.
**Depends on:** 06 · **Branch:** `feature/mab/yaml-tools`

## Scope

- Semantic diff in `yamlkit`: ignores key order; matches list items by
  identity (docs by kind/namespace/name, containers/env/ports by
  `name`); reports added/removed/changed by path. Text-diff fallback.
  Ignore-paths setting for noise.
- Compare targets: any two files; what-if buffer vs disk; two Helm
  profiles' renders (dev vs prod); two docs.
- UI: side-by-side view + change list by path (click → both sides jump).
- Query: jq expressions via `gojq` over one file, a render, or the
  workspace; results link back to source positions.
- CLI: `codec yaml diff a b` (exit 2 when different),
  `codec yaml query '<expr>' files…`.

## Acceptance

- [ ] Reordered keys/list items → no diff; renamed container → shown as change.
- [ ] Helm dev-vs-prod diff shows only real differences.
- [ ] Query results jump to the right line.

**Go concepts:** recursive comparison, generics for identity matching.
