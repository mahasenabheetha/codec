# 09 — Kubernetes lens

**Goal:** understand a set of manifests at a glance — what's there, how
it connects, what's broken.
**Depends on:** 07 · **Branch:** `feature/mab/p09-kubernetes`

## Scope

- `internal/kube` provider: outline with meaning (`Container nginx ·
  nginx:1.25 · 2 ports`), resource cards view.
- Inventory for a folder or render: kinds count, images + tags, ports,
  ConfigMap/Secret refs, total requests/limits.
- Relationships: Service → pod templates (selector vs labels), Ingress
  → Service:port, workloads → ConfigMap/Secret/PVC/ServiceAccount, RBAC
  bindings, HPA targets. Broken-reference diagnostics within the same
  folder/render scope (soft warnings: targets may live elsewhere).
- Shared Graph component (pick a small layout lib, e.g. dagre; record
  in decisions.md) — reused by Argo and CI.
- Neat: strip `managedFields`, `status`, `uid`, `resourceVersion`,
  timestamps, last-applied annotation (display/copy only).
- Secrets: decode `data` inline (masked, reveal), reusing `internal/codec`.
- Kustomize: build overlays in-process (`sigs.k8s.io/kustomize/api`,
  in-memory filesystem fed by the workspace) → same render view as Helm.
- CLI: `codec k8s images|refs|neat`, `codec kustomize build <dir>`.

## Acceptance

- [ ] Service with non-matching selector flagged; matching one linked.
- [ ] Kustomize output matches `kubectl kustomize` on fixtures.
- [ ] Graph readable for ~50 resources.

**Go concepts:** graph modelling, label-selector matching, in-memory filesystems.
