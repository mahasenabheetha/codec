# 02 — YAML engine core

**Goal:** a pure, well-tested YAML engine every later feature builds on.
**Depends on:** 00 (can run in parallel with 01) · **Branch:** `feature/mab/yaml-tools`
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

- [x] Positions correct for LF, CRLF and UTF-8 BOM files (plus non-ASCII, multi-doc).
- [x] A Helm template parses via masking with correct spans (`testdata/helm-deployment.yaml`,
      a `helm create`-style template; add anonymized fixtures from real repos as found).
- [x] Detection fixtures for every listed type, plus plain YAML (`provider/detect_test.go`).
- [x] Friendly-error fixtures for each listed mistake (`yamlkit/friendly_test.go`).

## Notes

- goccy/go-yaml parses (exact error positions, token extents); go.yaml.in/yaml/v3
  emits `fmt`/JSON→YAML output (comments kept). See decisions #10, #20–22.
- goccy quirks handled (each has a regression test):
  - leaks a BOM into the first key → BOM stripped before parsing;
  - counts comment lines twice in CRLF files → goccy gets LF text, ends computed by line/col;
  - shifts a plain scalar's start right by its trailing spaces → start re-anchored on its line.
- Whole-line template expressions are masked as `#` + spaces (not plain spaces): inside
  block scalars, over-indented blank lines are invalid YAML (broke Jinja playbooks).
- Verified on 2,290 real YAML files (Ansible, GitLab CI, Helm): no crashes, no false
  parse errors, 221k key/value ranges exact. Five genuine duplicate keys found.
- yaml.v3 prints merge keys as `!!merge <<` unless the tag is cleared (done; regression test).
- `Provider` has Detect + Symbols for now; phase 04 adds diagnostics/hover/definition/complete.
- Non-goals kept: no lens outlines, rendering or lint rules yet.

**Go concepts:** interfaces + registries, AST walking with type switches, byte vs rune offsets, `testdata/` fixtures, `errors.As`.
