# 10 — Argo lens (Workflows + ArgoCD)

**Goal:** read Argo workflows as what they really do: resolved
parameters, stitched templates, and a visual DAG.
**Depends on:** 09 · **Branch:** `feature/mab/yaml-tools`

## Scope

- `internal/argo` provider for Workflow, WorkflowTemplate,
  ClusterWorkflowTemplate, CronWorkflow; outline of templates by type
  (container/script/dag/steps/resource/suspend), entrypoint marked.
- Parameter resolution: `workflow.parameters` (from `spec.arguments` +
  user params panel/file), `inputs.parameters` via call-site
  arguments, `{{item}}` per-item expansion. Runtime-only values
  (`steps.*`, `tasks.*`, outputs, `workflow.uid`…) stay as clearly
  marked placeholders — never guessed. `{{=expr}}` highlighted.
- `templateRef` / `clusterScope` resolution across the workspace
  index: go-to-definition, inline view, broken-ref diagnostics.
- DAG/steps graph (shared Graph component): edges from `dependencies`
  and parsed `depends` expressions; parallel step groups; click node →
  jump to YAML.
- Helm-packaged Argo: the lens runs on Helm render output (phase 05),
  so `{{ "{{" }}`-escaped syntax is already unescaped.
- ArgoCD Application/ApplicationSet (basic): show source (repo, path,
  chart, values files, parameters) and destination; if the chart is in
  the workspace, "Render this app" opens the Helm view preloaded.
- CLI: `codec argo resolve <file>`, `codec argo graph <file> --format mermaid|dot`.

## Acceptance

- [ ] Real (anonymized) workflow from the user's repo resolves with
      runtime values marked, not invented.
- [ ] templateRef across files resolves and inlines.
- [ ] DAG with `depends` expressions renders correctly.

## Notes

The original argoview brief (Path A/B/C) is covered by this phase +
phase 05. Ask the user for a sample workflow before starting.

**Go concepts:** small expression parser (`depends`), cross-file indexes.
