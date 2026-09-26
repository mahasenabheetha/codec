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
lints the whole folder; **Lint settings** sets rule levels, the target
Kubernetes version and schema options. Ctrl+Space completes keys and
values from the file's schema.

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
