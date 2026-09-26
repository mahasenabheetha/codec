# 07 — Lint + schemas

**Goal:** catch mistakes before they reach a cluster or pipeline, and
give schema-aware completion and hover docs.
**Depends on:** 06 · **Branch:** `feature/mab/p07-lint`

## Scope

- Style rules (configurable in user settings): indentation consistency,
  trailing spaces, duplicate keys, truthy words (`yes/no/on/off`),
  octal-looking numbers, empty values, document start, line length (off
  by default). Each finding: severity, range, why, how to fix.
- JSON Schema validation (`santhosh-tekuri/jsonschema`), errors mapped
  to ranges. Sources: Kubernetes (selectable version), CRDs catalog
  (Argo, ArgoCD, cert-manager, …), SchemaStore (GitHub Actions, GitLab,
  Azure Pipelines, Compose). Fetched on demand, cached in
  `os.UserCacheDir()/codec/schemas`; offline mode + custom schema dir
  setting. Honour `HTTPS_PROXY`.
- Schema-driven completion (keys, enums) and hover (descriptions) in
  the editor via the phase 04 endpoints.
- Kubernetes checks (warnings with explanations): `:latest`/no tag, no
  requests/limits, no probes, privileged, `hostPath`, `hostNetwork`,
  missing `runAsNonRoot`.
- API deprecations: small embedded table, target K8s version setting.
- Problems panel: live for open files; "lint workspace" on demand;
  also runs on Helm render output.
- CLI: `codec yaml lint [paths] [--k8s-version]`, exit 2 on findings.

## Acceptance

- [ ] Fixtures per rule and per check; schema errors land on the right line.
- [ ] Works offline after first fetch; clear message when a schema is unavailable.
- [ ] Completion suggests valid keys inside a Deployment spec.

## Notes

Ask the user whether the company network needs a proxy before
implementing fetching.

**Go concepts:** HTTP client + caching, embedding data, rule-engine design.
