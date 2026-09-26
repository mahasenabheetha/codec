# 02 — YAML engine core

**Goal:** a pure, well-tested YAML engine every later feature builds on.
**Depends on:** 00 (can run in parallel with 01) · **Branch:** `feature/mab/p02-yaml-engine`
**Read:** architecture.md (layout, provider model)

## Scope

`internal/yamlkit`:
- `Parse(content)` → documents + diagnostics; tolerant (one bad doc
  doesn't lose the others). Byte offset + line/col ranges for every node.
- **Expression masking:** find `{{…}}`, `${{…}}`, `{%…%}` spans, replace
  them with same-length placeholders before parsing (positions stay
  valid), return spans separately. This is how Helm templates and
  Jinja files get parsed at all.
- Friendly errors: tab indentation, duplicate key, bad indentation,
  unquoted `: `, unclosed quote — plain-English message + range.
- Multi-document: split, name docs (`Kind/name` when present).
- `PathAt(pos)`, `NodeAt(path)`; path formats: dotted `a.b[0].c`,
  yq `.a.b[0].c`, JSONPath `$.a.b[0].c`, Helm `.Values.a.b`, `--set a.b[0].c=`.
- Transform for display/copy only: format (2-space, keep comments,
  optional key sort / K8s key order), YAML↔JSON, flatten/unflatten,
  anchors & merge keys resolved view.

`internal/provider`: interface + registry (architecture.md), generic
YAML provider, and **detection only** for: Kubernetes, Helm (Chart.yaml,
values*.yaml, templates/), Kustomization, Argo kinds, ArgoCD kinds,
GitHub Actions, GitLab CI, Azure Pipelines, Compose, Ansible playbook/inventory.

CLI: `codec yaml identify|outline|path|fmt|convert|flatten`.

## Out of scope

Lens features (outline labels, rendering, lint rules), any UI.

## Acceptance

- [ ] Positions correct for LF, CRLF and UTF-8 BOM files.
- [ ] A real (anonymized) Helm template parses via masking with correct spans.
- [ ] Detection fixtures for every listed type, plus "unknown".
- [ ] Friendly-error fixtures for each listed mistake.

## Notes

Confirm `goccy/go-yaml` is maintained and its comment-preserving output
is good enough for `fmt`; if not, record the alternative in decisions.md.

**Go concepts:** interfaces + registries, AST walking, byte vs rune offsets, `testdata/` fixtures.
