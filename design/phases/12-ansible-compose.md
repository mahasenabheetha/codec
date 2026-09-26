# 12 — Ansible + Compose lens

**Goal:** make playbooks and Compose stacks readable and connected.
**Depends on:** 09 · **Branch:** `feature/mab/yaml-tools`

## Scope

Ansible:
- Playbook outline (plays → pre_tasks, roles, tasks, handlers); role
  layout awareness (`roles/*/tasks/main.yml`, defaults, vars).
- Jinja highlighting (template-time color); hover a variable → where
  it's defined (group_vars, host_vars, defaults, vars) — best effort.
- Inventory YAML as groups/hosts tree.
- Link from the v1 Ansible log tool: failed task name → its definition
  in the open workspace.

Compose:
- Services graph (`depends_on`, networks); ports and volumes tables.
- `${VAR}` interpolation from `.env` with what-if values.
- Multiple compose files merged with provenance (reuse a generic
  layered-merge from `yamlkit` if phase 05's merge was factored out).

## Acceptance

- [ ] Real playbook outline matches execution order.
- [ ] Log-tool link finds the task definition.
- [ ] Compose override merge shows which file set each field.

**Go concepts:** reusing generic packages across domains.
