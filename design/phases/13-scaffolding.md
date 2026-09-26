# 13 — Scaffolding

**Goal:** speed up creating new YAML. Output is always text to copy
(or a what-if tab) — codec never writes files.
**Depends on:** 10, 11, 12 · **Branch:** `feature/mab/p13-scaffolding`

## Scope (in priority order)

1. **Clone-with-rename:** pick a file/doc, give a new name → renames
   `metadata.name`, common labels, template/job names and internal
   references; highlights fields that usually need changing (images,
   hosts, secret refs).
2. **Snippets** in the editor: container, probe, resources, DAG task
   (with dependency picker), GitHub step/job, GitLab job, Ansible task.
3. **Built-in starters** (embedded, with placeholders + small form):
   Deployment+Service(+Ingress), ConfigMap/Secret, Helm chart skeleton
   (multi-file; copy per file or browser download as zip), Argo
   WorkflowTemplate/CronWorkflow, ArgoCD Application, GitHub Actions
   workflow, GitLab pipeline, Azure pipeline, Compose service, Ansible
   playbook/role.
4. **Personal templates** in the user config dir (never in repos).
5. Stretch: schema-driven form for any kind (phase 07 schemas) with live YAML preview.

Every generated result runs through lint/render preview before copy.

## Acceptance

- [ ] Cloned WorkflowTemplate has no leftover references to the old name.
- [ ] Every starter passes phase 07 lint and schema validation.

**Go concepts:** `text/template`, embedded template sets.
