# shop: a sample repository

A small made-up service with the YAML a real repository collects. codec
extracted it to its cache folder so you can try things without opening
your own repo. Nothing here is deployed anywhere.

Things to try:

- **Helm:** open `charts/shop` in the Helm view. Switch between
  `values.yaml` and `values-prod.yaml`, hover a value to see which file
  set it, or change `replicaCount` in the editor and watch the render.
- **Kubernetes:** open the `k8s` folder in the Resources view, or build
  `k8s/overlays/prod` with Kustomize.
- **Problems:** run "Lint workspace". `k8s/legacy/` holds a removed API
  version, a Service that selects no pods and a container without
  resource limits.
- **Argo:** open `argo/build.yaml` for the DAG, with a template taken
  from `argo/common.yaml`. `argo/shop-app.yaml` renders the chart as
  Argo CD would.
- **CI:** `.github/workflows/ci.yml` (a matrix and a reusable
  workflow), `.gitlab-ci.yml` (includes and extends) and
  `azure-pipelines.yml` (templates and parameters).
- **Schedules:** hover the cron schedule in `k8s/base/cleanup.yaml`,
  `argo/nightly.yaml`, `.github/workflows/ci.yml` or
  `azure-pipelines.yml` to read it in plain words with its next runs.
- **Compose and Ansible:** `compose.yaml` with its override file and
  `.env`; `ansible/site.yml` with a role and an inventory.
- **Compare:** compare `charts/shop/values.yaml` with
  `values-prod.yaml`, or two Helm profiles.
