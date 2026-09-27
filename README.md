# codec

[![CI](https://github.com/mahasenabheetha/codec/actions/workflows/ci.yml/badge.svg)](https://github.com/mahasenabheetha/codec/actions/workflows/ci.yml)

A read-only YAML workbench for the files a DevOps engineer lives in —
Kubernetes, Helm, Argo, CI pipelines, Compose, Ansible — plus the small
encode/decode tools you reach for all day. One binary, a local web UI and
a CLI. It sits next to VS Code: open a repository, understand it, try
changes as what-if, jump back to your editor.

codec **never writes to your files** and nothing leaves your machine.

![Helm view: merged values, and where each value came from](docs/images/helm.png)

## What it does

- **Workspace.** Open a repository folder read-only: a file tree that
  honours `.gitignore`, every YAML file labelled by type (Kubernetes,
  Helm, Kustomize, Argo Workflows/CD, GitHub Actions, GitLab CI, Azure
  Pipelines, Compose, Ansible), Ctrl+P to jump to a file, tabs that
  refresh when the file changes on disk.
- **YAML editor.** Template and runtime expressions (`{{ }}`, `${{ }}`,
  `{% %}`) understood instead of reported as errors; syntax errors in
  plain English with a fix; hover for path, type and value; go to
  definition for anchors; outline and problems; the path at the cursor
  as dot, yq, JSONPath, Helm or `--set`.
- **What-if edits.** Change anything to see its effect — it is never
  saved. Copy the result, or copy a diff that `git apply` accepts.
- **Lint and schemas.** YAML 1.1 traps, risky Kubernetes settings,
  removed APIs for your target Kubernetes version, and validation
  against the published schema (Kubernetes, CRDs, GitHub Actions,
  GitLab CI, Azure Pipelines, Compose) — each finding says why it
  matters and how to fix it. The same schemas drive key completion and
  field docs in the editor.
- **Kubernetes and Kustomize.** See a folder, a render or a Kustomize
  build as objects: cards, a relationship graph (Service → pods,
  Ingress → Service, workloads → ConfigMaps and Secrets…), images,
  ports and resource totals, and references that point nowhere. Build
  Kustomize overlays without kubectl; copy "neat" manifests.
- **Argo.** Read a workflow as it would run: every step with the
  template it runs (templateRef across files) and the values it gets,
  steps and DAGs as graphs (`depends` expressions included), what-if
  parameters. Run-time values stay marked, never guessed. Argo CD apps
  show where they deploy from and to, and render with their values.
- **CI pipelines.** GitHub Actions, GitLab CI and Azure Pipelines as
  they run: jobs in execution order as a graph, every GitHub matrix
  combination, and each job's effective configuration after includes,
  `extends`, `!reference`, templates and reusable workflows — every
  line saying where it came from.
- **Compose.** A project as `docker compose config` sees it: the
  override (or any files you pick) merged with the Compose rules, every
  line saying which file set it; `${VAR}` filled from `.env` and what-if
  values; services as a graph with networks and volumes; ports and
  volumes tables; unknown services, undeclared networks and port clashes.
- **Ansible.** Playbooks in the order Ansible runs them — pre_tasks,
  roles (dependencies first), tasks, post_tasks, handler flushes — with
  roles and includes opened in place; hover a `{{ variable }}` to see
  where it is defined, highest precedence first; inventories as groups
  and hosts. The Ansible log tool links a failed task to its definition.
- **Compare and query.** Diff two files, a what-if edit against disk, or
  a chart's dev and prod renders by meaning — key and list order don't
  count, containers and env pair up by name. Run jq over the workspace,
  a file or a render and jump to each result.
- **Helm.** Render charts with the embedded Helm 4 SDK — byte-for-byte
  what `helm template` produces, no helm install, no cluster. Pick
  values files and `--set`, save them as profiles, and see for every
  merged value which file set it and what it overrode.
- **Tools.** Smart paste (detects JSON, base64, JWT, Ansible logs),
  base64, JSON pretty/minify/validate, JWT claims with expiry, Ansible
  `-vv` failure analysis.
- **CLI.** The same engine from the terminal and in CI (see [CLI](#cli)).

![Editor: a Helm template, hover shows the value it rendered with](docs/images/editor.png)

## Quick start

### Native binary (Windows, macOS, Linux)

Download the archive for your platform from
[GitHub Releases](https://github.com/mahasenabheetha/codec/releases),
unpack it, and run:

```bash
codec serve --open --root path/to/your/repo
```

No installer and no admin rights; `codec version` shows what you have.
The UI is at http://localhost:8765 and only this machine can reach it.

- **Windows:** the binary isn't code-signed, so SmartScreen may say
  "Windows protected your PC" — choose *More info → Run anyway*, or
  unblock it once with `Unblock-File .\codec.exe` in PowerShell.
- **macOS:** Gatekeeper blocks unsigned downloads. Remove the
  quarantine flag once: `xattr -d com.apple.quarantine codec`.

### Docker

```bash
docker run --rm -p 127.0.0.1:8765:8765 -v "$PWD:/work:ro" ghcr.io/mahasenabheetha/codec:2.0.0-alpha.1
```

Then open http://localhost:8765. The repository is mounted read-only at
`/work`; file changes are picked up by polling. In PowerShell use
`-v "${PWD}:/work:ro"`. Keep `127.0.0.1:` in `-p` so the port isn't
open to your network. To keep saved Helm profiles and recent folders
between runs, add `-v codec-home:/home/nonroot`. Images are published
for linux/amd64 and linux/arm64; `latest` points at the newest stable
release (pre-releases only get their version tag).

### From source

Go 1.26+ and Node.js 22+:

```bash
git clone https://github.com/mahasenabheetha/codec.git
cd codec
npm --prefix frontend ci && npm --prefix frontend run build
go build -o codec ./cmd/codec
```

## Web UI

```bash
codec serve                     # http://localhost:8765
codec serve --open              # …and open it in the browser
codec serve --root ~/repos/app  # open a folder right away
codec serve --port 9000 --poll  # other port; poll for file changes
```

Open a folder with Ctrl+O (recent folders and a folder browser) or
`--root`. Files open as YAML-aware editors; **Render** on any chart file
(or **Helm: render …** in the palette) opens the Helm view. "Open in VS
Code/Cursor" jumps to the same file and line. **Problems** in the sidebar
lints the whole folder, **Compare** diffs files or Helm profiles by meaning,
**Query** runs jq; **Lint settings** sets rule levels, the target
Kubernetes version and schema options. Ctrl+Space completes keys and
values from the file's schema. **Argo** on a workflow or application
file (or the Argo tab of a Helm render) opens the workflow as a graph
with its parameters resolved. **Pipeline** on a GitHub Actions, GitLab
CI or Azure Pipelines file shows its jobs in execution order.
**Compose** on a Compose file shows its services with the override
merged; **Ansible** on a playbook, task file or inventory shows it in
execution order (or as groups and hosts).

| Shortcut | Action |
|---|---|
| Ctrl+K | Search tools and actions |
| Ctrl+P · Ctrl+O | Go to file · open folder |
| Ctrl+Shift+E | Toggle the explorer |
| F12 / Ctrl+click | Go to definition (YAML alias → anchor) |
| Ctrl+G · Ctrl+F | Go to line · find in file |
| Ctrl+Enter | Run the transform (tools) |
| Alt+C · Alt+S | Copy output · use output as input |
| Ctrl+B | Toggle the sidebar |
| ? | Show all shortcuts |

On macOS, Ctrl is ⌘. Nothing you paste is stored. Settings (recent
folders, Helm profiles) live in your user profile —
`%AppData%\codec`, `~/Library/Application Support/codec` or
`~/.config/codec` — never in the repository.

**Install as an app:** in Edge, menu → Apps → *Install this site as an
app* (Chrome: the install icon in the address bar) for its own window
and taskbar icon. `codec serve` must be running.

**Security.** The server binds to `127.0.0.1`, rejects requests for other
host names (DNS rebinding), requires a per-run token on its API, and
confines file access to the opened folder (symlinks too), so web pages
you visit can't use it.

## CLI

Data goes to stdout, messages to stderr; exit code 1 on invalid input,
so every command works as a CI gate. Files are never modified.

### YAML

```bash
codec yaml identify deploy.yaml             # file type, documents, problems (file:line:col)
codec yaml outline deploy.yaml              # key/item tree with line numbers
codec yaml path deploy.yaml --line 26 --col 12 --style all
#   dot       spec.template.spec.containers[0].image
#   yq        .spec.template.spec.containers[0].image
#   jsonpath  $.spec.template.spec.containers[0].image
#   helm      (index .Values.spec.template.spec.containers 0).image
#   set       spec.template.spec.containers[0].image=
codec yaml path values.yaml --get image.tag # value and its position
codec yaml fmt messy.yaml --k8s-order       # re-indent, comments kept
codec yaml fmt anchors.yaml --resolve       # expand anchors and << merges
codec yaml convert values.yaml --to json    # key order kept (and --to yaml)
codec yaml flatten values.yaml              # path: value lines; --reverse to undo
```

### Lint

```bash
codec yaml lint                              # the current folder (honours .gitignore)
codec yaml lint charts/ deploy.yaml --k8s-version 1.31
codec yaml lint . --offline --json           # cached schemas only; machine-readable
#   deploy.yaml:6:3: error: Unknown field "replica" in spec [schema]
#       fix: Did you mean "replicas"?
```

Exit status 2 when there are warnings or errors, so it works as a CI
gate. Rule levels (error/warning/info/off), the target Kubernetes
version and schema options are set in the web UI's **Lint settings** and
shared with the CLI. Schemas are downloaded on first use and cached in
your user cache folder; with **Offline** on, only the cache and your
custom schema folder are used.

### Kubernetes and Kustomize

```bash
codec k8s images deploy/                     # images, tags, who runs them
codec k8s refs deploy/                       # how objects connect; exit 2 on broken references
helm template ./chart | codec k8s refs       # also from stdin
kubectl get deploy api -o yaml | codec k8s neat
codec kustomize build overlays/prod          # like kubectl kustomize, no kubectl needed
```

### Argo

```bash
codec argo resolve wf.yaml templates/ -p env=prod   # each step, its template, its inputs
#   build(0:linux) → compile · script · golang:1.26
#       target = linux-amd64
#   diagnose → collect-logs · container
#       from = {{tasks.test.outputs.parameters.report}}   (run time)
helm template ./chart | codec argo resolve -w deploy-wf
codec argo graph wf.yaml --template main --format mermaid   # or dot
```

### CI pipelines

```bash
codec ci jobs .gitlab-ci.yml                 # stages and jobs in execution order
#   stage test
#     test  · golang:1.26 · runner docker · × 2
#         after build
#         extends .base → .tester
codec ci job .gitlab-ci.yml build            # effective config, where each line comes from
#   image: golang:1.26  # ← extends .base  ci/templates.yml:2
#   tags:               # ← default
#     - docker          # ← default
codec ci jobs .github/workflows/ci.yml --json
codec ci graph azure-pipelines.yml -p env=prod --format dot | dot -Tsvg > ci.svg
```

Includes, templates and actions are read from the repository (the
folder holding `.git`, or `--root`); remote ones are listed, never
fetched.

### Compose

```bash
codec compose services                       # compose.yaml + its override, here
#   web  · build ./web
#     after db (healthy)
#     port 8080 → 80/tcp  (compose.yaml)
#     port 9229 → 9229/tcp  (compose.override.yaml)
codec compose config                         # merged services, which file set each line
#   command: npm run dev   # ← compose.override.yaml:3
#     - 8080:80            # ← compose.yaml:13
codec compose services compose.yaml compose.prod.yaml -e TAG=1.2 --json
```

`${VAR}` comes from `.env` (or `--env-file`) and `-e`; your shell's
environment isn't read.

### Ansible

```bash
codec ansible plays playbooks/site.yml       # plays and tasks in execution order
#   play Web servers  · hosts web
#     (gathering facts)
#     pre_tasks:
#       Update cache  · apt  → notifies web restart
#     (run notified handlers)
#     roles:
#       role common
codec ansible vars playbooks/site.yml app_port   # where it is defined, highest precedence first
#   play vars (Web servers)    8080  playbooks/site.yml:5
#   group_vars (web)           9090  group_vars/web.yml:1
codec ansible task "nginx : restart nginx"   # a task from a log → its definition
codec ansible inventory inventory/hosts.yml
```

### Diff and query

```bash
codec yaml diff dev.yaml prod.yaml           # by meaning; exit 2 when different
#   Deployment api
#     ~ spec.replicas: 1 → 3
#     ~ spec.template.spec.containers[name=app].image: nginx:1.16 → nginx:1.27
codec yaml diff a.yaml b.yaml --ignore 'metadata.labels.helm.sh/chart'
codec yaml query '.spec.template.spec.containers[].image' charts/   # file:line: value
codec yaml query 'select(.kind == "Ingress") | .spec.rules[].host' . --json
```

### Helm

Dependencies must be vendored in `charts/` — codec never downloads.

```bash
codec helm render ./charts/app -f values-prod.yaml --set image.tag=1.27   # like helm template
codec helm values ./charts/app -f values-prod.yaml --provenance           # merged values, annotated
#   replicaCount: 3  # ← values-prod.yaml:1 (overrides values.yaml:6)
```

### Encode, decode, logs

```bash
codec auto '{"name":"mahasen"}'        # detect and transform (also reads stdin)
codec b64 encode 'any text at all'     # --url for the URL-safe alphabet
codec b64 decode aGVsbG8=              # tolerates missing padding
codec json pretty '{"a":{"b":1}}'      # --indent to customize
codec json validate '{"a":}'           # exit 1 + "line 1, column 6"
codec jwt decode "$TOKEN"              # does NOT verify the signature
codec auto < failed-task.log           # Ansible -vv: probable cause, rc/msg, highlighted stderr
codec watch                            # transform recognizable clipboard content in place
```

`-c` on any command also copies the output. Windows PowerShell 5.1
strips inner double quotes from native arguments; pipe instead:
`'{"a":1}' | .\codec.exe auto` (PowerShell 7+ is fine).

## Development

```bash
npm --prefix frontend ci && npm --prefix frontend run build   # UI, embedded by go build
go vet ./... && go test ./...
scripts/dev.sh ui     # build, serve, and the hot-reload UI at http://localhost:5173/
```

Architecture, conventions, decisions and the roadmap live in
[design/](design/README.md); contributors and AI agents start at
[AGENTS.md](AGENTS.md). Changes go through pull requests. Releases:
[CHANGELOG.md](CHANGELOG.md) and [design/releases.md](design/releases.md).

## Acknowledgements

This project was developed as a hands-on Go learning journey by
[Mahasen Abheetha](https://github.com/mahasenabheetha), with development
assistance from **Claude (Anthropic)** used as a pair-programming and
teaching tool. Design decisions, code review, and explanations were
AI-assisted; the learning was not.
