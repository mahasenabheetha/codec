# 09 — Kubernetes lens

**Goal:** understand a set of manifests at a glance — what's there, how
it connects, what's broken.
**Depends on:** 07 · **Branch:** `feature/mab/yaml-tools`

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

- [x] Service with non-matching selector flagged; matching one linked
  (`internal/kube/kube_test.go`, also named-targetPort and Ingress port checks).
- [x] Kustomize output matches `kubectl kustomize` on fixtures (goldens from
  kubectl 1.36 / kustomize v5.8.1: base + overlay with namePrefix, labels,
  component, images, replicas, patches, generators).
- [x] Graph readable for ~50 resources (49-object fixture; fit keeps a
  readable zoom, pan for the rest).

**Go concepts:** graph modelling, label-selector matching, in-memory filesystems.
