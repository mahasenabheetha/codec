# codec 2.0 — plan

Status: **under discussion, nothing built.** Decisions below are
proposals until marked settled.

## Goal

A lightweight local DevOps workbench: the v1 tools (base64, JSON, JWT,
Ansible logs) plus a new **YAML workbench** as the flagship section.
Not Argo-only — general YAML with domain lenses (Kubernetes, Helm,
Argo, CI pipelines, Compose, Ansible). Priority: readability and
confidence over feature count. Web UI first, desktop app (Wails) later.

## Proposed features

- **YAML core:** friendly parse errors, multi-document split, file-type
  detection, semantic outline, cursor path breadcrumbs + copy path
  (yq / JSONPath / `.Values`), jq-style query, format, k8s "neat",
  anchor resolution, convert (JSON, flatten, `--set`), semantic diff,
  inline secret decode, embedded content handoff to v1 tools, inventory
  (kinds, images, refs).
- **Lint/validate:** yamllint-style rules, JSON Schema (k8s, CRDs,
  SchemaStore), k8s best practices, API deprecations.
- **Helm:** render with layered values, values provenance (which file
  set each value), template ↔ output mapping, undefined/unused values.
- **Kubernetes:** resource relationship graph, broken-reference checks.
- **Argo:** parameter resolution (runtime values as placeholders),
  `templateRef` inlining, DAG graph. Original brief:
  `../../argoview-design.md` (outside this repo).
- **Later:** CI pipeline graphs, Compose, Ansible playbook outline.
- **Everywhere:** command palette, tabs, file watch, CLI parity
  (`codec yaml|helm|k8s ...`), exit code `2` for findings/diffs.

## Proposed architecture

```
internal/codec      v1 transforms (unchanged)
internal/yamlkit    engine: parse, detect, outline, paths, diff, lint
internal/kube       engine: k8s knowledge
internal/helm       engine: values merge + provenance
internal/argo       engine: params, templateRef, DAG
internal/runner     adapter: runs helm/kustomize binaries
internal/workspace  adapter: sandboxed local file access
internal/cli, internal/web
```

## UI direction

Collapsible left rail grouping tools (Encode/Decode, Logs, YAML).
YAML workbench: files | editor | lens panel (Outline, Rendered, Values,
Problems, Graph) + status bar. Dark/light, keyboard-first, system
fonts, small frontend budget (~500 KB).

## Open decisions

1. Helm: shell out to `helm` (leaning) vs embed Helm SDK.
2. YAML library: `goccy/go-yaml` (leaning) vs `go.yaml.in/yaml/v3`.
3. Frontend: Preact + htm + CodeMirror 6, vendored, no build step
   (leaning) vs vanilla JS.
4. Local file access via the server (needs Host/Origin checks + session
   token) vs paste/drag-drop only.
5. Schemas: fetch-and-cache vs bundled offline set.
6. Lens priority (assumed: Kubernetes, Helm, then Argo).

## Proposed release slices

- `v2.0.0-alpha`: `/v2` module path, new UI shell, v1 tools moved in,
  YAML core basics.
- `v2.0.0`: diff, query, Helm render + provenance, workspace, CLI parity.
- `v2.1`: lint/schema/k8s checks. `v2.2`: Argo lens.
  `v2.3`: CI/Compose/Ansible lenses. `v3.0`: Wails desktop app.
