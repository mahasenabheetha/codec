# Changelog

All notable changes to codec are documented here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and codec uses
[Semantic Versioning](https://semver.org/).

## [Unreleased]

### Added

- CLI: `codec url`, `hex`, `hash`, `secret`, `uuid` and `htpasswd` for
  percent-encoding, hex, digests and HMACs, random secrets, UUIDs and
  bcrypt basic-auth lines. Passwords and HMAC keys are never taken as
  arguments.
- CLI: `codec time` reads epoch numbers (seconds to nanoseconds, told
  by size) and dates in any zone; `codec cron` explains an expression in
  plain words, field by field, with its next runs across daylight-saving
  changes, and names Quartz and Jenkins expressions.
- CLI: `codec regex` lists matches with their groups, replaces
  (`--replace`) or explains a pattern part by part (`--explain`), in Go's
  RE2 or the Python / .NET / JavaScript style (lookarounds,
  backreferences; stopped after 2 seconds when it backtracks badly).
- Encode & hash, under a new Utilities heading: URL (with the parts and
  query as a table), hex, hashes and HMAC with an expected-digest check,
  secrets and UUIDs, and htpasswd lines with a check. Tools can have
  sub-tabs; Home and the palette list each one.
- Time, under Utilities: Timestamp (epoch units told by size, dates,
  any zone, relative time, Now) and Cron (plain words, field by field,
  next runs, presets and a form that builds the expression and fills
  from it).
- Hover a cron schedule in a Kubernetes CronJob, an Argo CronWorkflow,
  a GitHub workflow or an Azure pipeline to read it in plain words, with
  the zone it runs in and the next runs. The sample has one of each.
- Regex, under Utilities: matches highlighted as you type, capture
  groups per match, a replace preview, and an explanation of the pattern
  part by part; hovering a part finds it in the pattern and a group
  lights up what it captured. Go (RE2) or Python / .NET / JS style.
- Smart paste reads epoch timestamps, URL-encoded text and cron
  expressions, and opens them in Timestamp, URL or Cron with one click.

## [2.2.0] - 2026-10-03

The Ansible log tool reads whole pipeline logs, from one failed task to
several playbook runs among other output, and the Ansible lens draws a
playbook map that a loaded log colours by what ran.

### Added

- `codec ansible log <file|->` reads a whole pipeline log — CI
  timestamps, Packer or Compose prefixes, colour codes, shell output,
  several playbook runs — and prints each run's failures with their
  reason and defining file, the recap, and other output with errors.
  Rescued and ignored failures, unreachable hosts and runs cut short
  are told apart. `--all` lists every task with its duration.
- Ansible log tool reads whole pipeline logs: paste one or drop a file
  (up to 64 MB). A summary and verdict, an outline of runs, plays and
  tasks with the output around them, filters (Failed, Changed, Skipped),
  a host picker and search, the first failure selected; each host's
  result in the single-task view, and the recap as a table. One task's
  output still opens in the detailed single-task view; the pasted text
  stays in view beside the result.
- Ansible lens: a playbook map of plays, roles, task files and handlers;
  imports solid, includes dashed, templated include files resolved when
  their variable has one definition. With a log loaded, the map is
  coloured by a run: failed, changed, ok, rescued, never ran.

### Changed

- Roles listed in a requirements file show as external (installed when
  the playbook runs) instead of not found.

### Fixed

- Ansible "Defined in" no longer matches another role's file with the
  same name (every role has a tasks/main.yml).

## [2.1.0] - 2026-10-03

Compare takes pasted text and files picked in the Explorer, Find in
Files searches the text of every file, and a page left open across a
restart offers a reload.

### Added

- Find in Files (Ctrl+Shift+F, or Search in the sidebar): text across
  every listed file as you type, with match case, whole word, regex and
  include/exclude globs; results grouped by file open at the match.
  `codec search <text> [folder]` does the same on the command line.
- Compare: paste as a third source per side, next to File and Helm render
  — a live object against the repo file, or two pasted texts with no
  folder open ("Compare text" on Home and in the palette). Pasted text is
  never saved.
- Compare as Auto, Structure or Text. Auto compares by structure only
  when both sides are YAML/JSON mappings or lists, otherwise line by line;
  text diffs can ignore whitespace and case.
- Compare: text diffs show side by side, each changed line beside its
  replacement with the changed characters marked, unchanged runs folded,
  and where the first difference is ("character 48"). Unified view and
  "Copy as patch" stay one click away. Each side's picker now heads its
  own column.
- Compare: "Add live object noise" fills in the fields a cluster adds
  (`managedFields`, `resourceVersion`, `uid`, `status`, …) as ignored paths.
- Explorer: a right-click menu (also Shift+F10) with Open, "Select for
  compare", "Compare with …" and, for two Ctrl+clicked files, "Compare
  selected". Files can be dragged onto a side of the Compare view.
- A "codec restarted — Reload" banner when the page outlives the server
  run that served it, instead of failing requests.
- Compare: an × on each side clears it, and Clear empties both sides and
  the result. Ignored paths stay; cleared pasted text can be undone.

## [2.0.1] - 2026-09-28

### Changed

- New app icon for codec 2 (a YAML key with nested key: value rows), with a
  maskable icon so the installed app fills its tile. The installed app is
  named "codec 2", so it sits apart from a v1 install.

## [2.0.0] - 2026-09-27

codec 2.0: the read-only YAML workbench for Kubernetes, Helm, Kustomize,
Argo, CI pipelines, Compose and Ansible, with lint, schemas, compare,
query and starters, on top of the v1 tools.

### Added (release)

- Settings view (Ctrl+,), replacing Lint settings: the editor "Open in"
  launches, the outline panel, Kubernetes version, schemas, rule levels,
  saved Helm profiles (forget one, or all of a chart that is gone),
  ignored diff paths, codec's folders and the recent folders list.
- Sample repository built into the binary: **Open the sample** on the
  home screen or in the explorer, the palette, or `codec serve --sample`
  writes it to codec's cache folder and opens it on a rendered chart.
  First run shows "try the sample" and "open a folder".
- Palette entries for the file toolbar (open in editor, copy path,
  reveal, compare, query, clone, reset, copy diff).
- `scripts/perf.sh` times a 10k-file repo, a 5 MB file and a
  500-document chart against the performance budgets.
- Documentation site in `docs/` (GitHub Pages): a guide for every view,
  the CLI reference, shortcuts, troubleshooting, architecture and flow
  diagrams, security and the design language, with screenshots.
- MIT licence, contributing guide, security policy, issue and pull
  request templates.

### Changed (release)

- Faster on big inputs: documents of a file parse in parallel (a 5 MB
  file in about 0.3 s instead of 1 s), long trees (explorer, outline)
  draw only the rows in view, and outlines of huge files keep fewer
  levels. Files up to 8 MB open (was 2 MB).
- The Helm view shows a render at once and adds schema findings when
  the schemas have downloaded; schemas needed together download
  together.
- Muted text and code comments are brighter, for 4.5:1 contrast.
  Loading states are placeholders rather than text.

### Fixed (release)

- Kustomize patch files are no longer flagged for the required fields
  of the complete object.
- Hosts and groups without settings in YAML inventories aren't flagged
  as empty values.
- The Resources view no longer fails to open when the same object is
  defined twice (a base and an overlay's patch).
- Settings described diff ignore patterns wrongly: `*` matches any
  characters, dots included.

### Added (Scaffolding)

- New view: starters for Deployment + Service (+ Ingress), ConfigMap,
  Secret, a Helm chart, Argo WorkflowTemplate and CronWorkflow, Argo CD
  Application, GitHub Actions, GitLab CI and Azure pipelines, a Compose
  service and an Ansible playbook and role. A small form fills them in;
  the result is linted, schema-checked and (charts) rendered with
  helm template before you copy a file or download a zip. Edits in the
  preview are what-if only.
- Personal starters in the `templates` folder of codec's settings
  folder: a folder per starter with an optional `starter.yaml`, or
  single files whose fields are the `<% .name %>` placeholders they use.
- Clone with a new name: a copy of a file or one document, renaming
  every whole-word use of the old name (labels, template and job names,
  internal references, keeping comments and layout). References to
  objects outside the copy, Secrets, images and addresses are listed as
  suggestions; images, hosts and Secret references as things to check.
- Editor snippets: container, probe, resources (Kubernetes, Helm, Argo),
  DAG task after the last one (Argo), step, action and job (GitHub),
  job (GitLab), task (Ansible), service (Compose). In Argo DAGs,
  `depends:`, `dependencies:` and `template:` complete from the tasks
  and templates around.
- CLI: `codec new [starter] --set name=value`, `codec clone <file> --to
  <name>`.

### Added (Compose and Ansible)

- Compose view: the project's files merged with the Compose rules
  (ports and volumes merge, commands replace, `!reset`/`!override`,
  `extends`, `include`), every line of a service saying which file set
  it; `${VAR}` from `.env` and what-if values (never saved, shell
  environment not read); services graph with start order, networks and
  volumes; ports and volumes tables; other Compose files in the folder
  can be added as layers.
- Compose checks: unknown services in depends_on/links, undeclared
  networks, volumes, secrets and configs, dependency loops, unset and
  required variables, host port clashes. Editor: outline of services,
  hover and go to definition for `${VAR}` (to `.env`), services
  (including `extends` into another file), networks and volumes.
- Ansible view: plays in execution order (facts, pre_tasks, roles with
  their dependencies, tasks, post_tasks, handler flushes), roles and
  import/include_tasks/role opened in place, handlers, variables with
  every definition ranked by Ansible's precedence, YAML inventories as
  groups and hosts.
- Ansible roles found via roles/, `roles_path` in ansible.cfg, folders
  above, or another repository of the open folder; roles installed at
  run time are info (naming the requirements file that lists them).
- Ansible editor: outline in execution order, Jinja tints, hover and
  go to definition for `{{ variables }}` and `when:` expressions, roles,
  handlers (`notify` → handler or `listen`), included files, templates.
- Ansible log tool: "Defined in" links a failed task to its definition
  in the open folder (by task path, else by name).
- CLI: `codec compose services|config`, `codec ansible
  plays|vars|task|inventory`.

### Fixed

- Lint no longer flags GitHub Actions events written without settings
  (`workflow_dispatch:`) as empty values.
- Merged keys in effective configurations (GitLab, Compose) now show
  the line of the file that set them.

### Added (CI pipelines)

- Pipeline view for GitHub Actions, GitLab CI and Azure Pipelines: jobs
  as a graph in real execution order (needs, stage order, dependsOn)
  or as a list by stage; for each job its steps, what it waits for,
  rules/conditions and its effective configuration, each line saying
  where it came from (the job, `extends .base`, `default`, global
  variables, `!reference`, a template file).
- GitLab: `include: local` (globs too) followed, project/remote/
  template/component includes listed as not read; anchors and merge
  keys, `!reference`, `extends` and `default:`/`inherit:` merged the
  way GitLab does; `parallel: matrix` jobs named; child pipelines.
- GitHub: matrix preview with every combination, include/exclude
  applied (and marked); local reusable workflows run in the caller's
  place, with the inputs it passes checked; local composite actions
  show their steps.
- Azure: local `template:` files inserted with their parameters,
  `${{ if }}`/`${{ each }}`/parameters evaluated (what-if parameters
  in the view), `extends:` templates, deployment jobs.
- Editor: jobs, stages and steps in the outline; go to definition and
  hover for needs, extends, `!reference`, local includes, `uses:`,
  `template:`, dependsOn and `${{ }}` contexts (inputs, env, secrets
  by name only, matrix, needs outputs, steps); diagnostics for unknown
  jobs and templates, missing files, needs across stages, undeclared
  inputs and more; Azure `$(var)` and `$[ ]`, GitLab `$[[ inputs ]]`
  highlighted.
- `codec ci jobs <file> [-p name=value] [--json]` (exit 2 on problems),
  `codec ci job <file> <job>` (effective config with origins) and
  `codec ci graph <file> --format mermaid|dot`.

### Added (Argo)

- Argo view for Workflows, WorkflowTemplates, ClusterWorkflowTemplates,
  CronWorkflows and the workflows Argo Events sensors submit: every
  step with the template it runs (templateRef followed across files)
  and the inputs it gets, parameters resolved from arguments, call
  sites, defaults and loop items. Values only known at run time (step
  outputs, workflow.uid, event data) stay as written and are marked.
- Steps and DAGs as graphs: parallel step groups, `dependencies` and
  `depends` expressions (edges labelled Failed, AnySucceeded…),
  drill into nested templates, click a step for its details and its
  template with the values filled in; `when` conditions are evaluated
  once their values are known ("skipped with these values").
- What-if parameters (or a parameter file), never saved; enum values
  are checked.
- Helm-packaged workflows: an Argo tab on the Helm view resolves the
  render; raw templates work too, with Helm's `{{ "{{" }}` escapes
  understood.
- Editor: templates outlined by type with the entrypoint marked; go to
  definition and hover for template names, templateRef (other files,
  raw Helm templates included), depends terms and parameters;
  diagnostics for missing templates, bad depends expressions, unknown
  tasks, undeclared inputs and missing or unused arguments.
- Argo CD Applications and ApplicationSets: source, value files,
  parameters, destination, sync policy and generators; "Render this
  app" opens the chart in the Helm view with the app's values.
- `codec argo resolve [files…] [-w workflow] [-p name=value]
  [--parameter-file f] [--json]` (exit 2 on problems) and
  `codec argo graph … [--template t] --format mermaid|dot`.

### Added (Kubernetes)

- Resources view for a folder, a Helm render or a Kustomize build:
  objects as cards (replicas, containers, ports, keys, selectors, who
  uses what), a relationship graph, an inventory (kinds, images and
  tags, ports, ConfigMaps/Secrets used, requests/limits totals) and
  reference problems.
- Relationships: Service → pods (selector vs labels, named target
  ports), Ingress → Service:port, workloads → ConfigMaps, Secrets, PVCs
  and ServiceAccounts, RBAC bindings, HPA and PDB targets. References
  that point outside the scope are warnings (they may live elsewhere).
- Kubernetes files get an outline that says what things are
  ("Container app · nginx:1.27 · 1 port", "Env DB_PASSWORD · from Secret
  db/password"); hovering a Secret's data shows it decoded; Secret cards
  mask values until you reveal them.
- Neat: copy manifests without status, managedFields, uid,
  resourceVersion, timestamps or last-applied annotations.
- Kustomize built in-process — the same output as `kubectl kustomize`,
  with what-if edits, lint and schema problems, and the resources view.
- `codec k8s images|refs|neat` and `codec kustomize build <dir>`.

### Added (compare and query)

- Semantic diff: key order and list order don't count; Kubernetes
  documents pair up by kind/namespace/name and list items (containers,
  env, ports, volumes…) by name, so a renamed container is one change.
  Changes are listed by path; command/args lists keep their order.
- Compare view: any two files, what-if edits vs disk, one document vs
  another, or a chart rendered with two profiles (dev vs prod) — change
  list by path, both sides with changed lines marked, click to jump.
  Ignore noisy paths with patterns (saved in your settings).
- Query view: jq expressions (gojq) over the workspace, one file or a
  Helm render; results jump to their line.
- `codec yaml diff a b [--ignore p] [--text] [--json]` (exit 2 when
  different) and `codec yaml query '<expr>' [files|folders…]`.

### Added (lint and schemas)

- Lint rules with a why and a fix for every finding: YAML 1.1 traps
  (yes/no/on/off, octal-looking numbers), duplicate keys, indentation
  consistency, empty values, trailing spaces, and optional document
  start and line length.
- Kubernetes checks: unpinned images, missing requests/limits, missing
  readiness probe, privileged containers, hostPath, hostNetwork, running
  as root; removed and deprecated APIs for a target Kubernetes version
  (1.30–1.37, default 1.34).
- Schema validation for Kubernetes (per version), CRDs (Argo, Argo CD,
  cert-manager, … from the CRD catalog), GitHub Actions, GitLab CI,
  Azure Pipelines, Compose and Kustomization — errors on the right line,
  with "did you mean" for misspelled fields. Schemas download on first
  use and are cached; offline mode and a custom schema folder.
- Editor: schema key and value completion (also mid-edit, with docs),
  field descriptions on hover, a badge naming the schema in use; problems
  show why they matter and can turn their rule off.
- Workspace problems view (lint every YAML file), Lint settings view
  (levels per rule, Kubernetes version, schema options; kept in your
  settings), and lint + schema findings on Helm's rendered output.
- `codec yaml lint [paths] [--k8s-version] [--offline] [--no-schemas]
  [--json]`: exit 2 when there are warnings or errors.

## [2.0.0-alpha.1] - 2026-09-26

First 2.0 pre-release: the read-only YAML workbench (workspace, editor,
Helm) on top of the v1 tools, as native binaries and a Docker image.

### Added (release)

- Docker image `ghcr.io/mahasenabheetha/codec` for linux/amd64 and
  linux/arm64 (distroless, non-root): mount a repository read-only at
  `/work`; file changes are detected by polling. Version tags on every
  release, `latest` on stable ones only.
- README rewritten for 2.0 with screenshots, Docker quick start and
  notes for Windows SmartScreen and macOS Gatekeeper.

### Added (YAML)

- `codec yaml identify|outline|path|fmt|convert|flatten`: detect 13 file
  types (Kubernetes, Helm chart/template/values, Kustomize, Argo
  Workflows, Argo CD, GitHub Actions, GitLab CI, Azure Pipelines,
  Compose, Ansible playbook/inventory), outline, cursor path in five
  notations, comment-preserving re-indent, YAML↔JSON, flatten/unflatten,
  anchor/merge-key resolution.
- Template-aware parsing: Helm/Go-template, Jinja, GitHub and Argo
  expressions are recognised instead of breaking the parse.
- Plain-English syntax errors with fix hints (tabs, bad indentation,
  unquoted colons, unclosed quotes) and duplicate-key detection.

### Added (Helm)

- Render charts with the embedded Helm 4.3 SDK — byte-for-byte the same
  output as `helm template`, no helm install, no cluster, no downloads
  (dependencies must be vendored in `charts/`, `.tgz` or folders).
- Helm view per chart: values layers (add, reorder, toggle), `--set`,
  release/namespace/Kubernetes version, saved profiles (kept in your
  settings), rendered manifests grouped by template, NOTES, problems
  with jump-to-line, and live re-render including what-if edits from
  open tabs.
- Values provenance: every merged value says which layer set it and what
  it overrode (hover in the Values view, or on `.Values.*` in a
  template); `null` deletions, subchart defaults and globals included.
- Checks: `.Values` paths used but set nowhere (unguarded uses only),
  defaults no template uses, template errors mapped to file:line, and a
  note where `lookup` returns empty without a cluster.
- `codec helm render <chart> -f … --set …` and
  `codec helm values <chart> -f … --provenance`.
- Helm templates get template-aware syntax highlighting.

### Added (editor)

- YAML files open in an editor: Helm/Jinja (template-time) and Argo/
  GitHub (runtime) expressions tinted, errors squiggled with plain-English
  hints, hover cards (path, type, value, anchors, YAML 1.1 gotchas,
  expression meaning), go to definition for aliases (F12, Ctrl/⌘+click),
  alias completion after `*`, go to line (Ctrl+G).
- Outline and Problems panel synced with the cursor; status-bar path
  breadcrumb that copies the path as dot, yq, JSONPath, Helm or `--set`;
  document switcher for multi-document files.
- What-if edits: change anything, never saved; Reset, Copy content and
  Copy diff (a patch that `git apply` accepts, keeping CRLF/BOM). A disk
  change during edits asks whether to reload or keep them.
- Open the file at the cursor in VS Code or Cursor.
- Friendly error for a line indented under a key that already has a
  value (the parser used to blame the wrong line).

### Added (workspace)

- Open a repository folder read-only (Ctrl+O, recent folders, folder
  browser with drives on Windows, or `codec serve --root`): file tree
  that honours `.gitignore`, file-type logos and a type filter, Ctrl+P
  fuzzy Go to file, files in read-only tabs that refresh on disk change.
- `codec serve --host --root --poll --open`. Native file watching with a
  polling fallback (automatic inside containers).
- Security for the local server: Host-header check (DNS rebinding),
  per-run token on `/api/v2`, Origin check on state-changing requests,
  and file access confined to the opened folder (symlink escapes too).
- Settings (recent folders) live in the user profile, never in a repo.

### Fixed

- A link straight to a file (`#/file/…`) opened on a first visit no
  longer lands on the home page.
- "File not found" errors name the path instead of the raw OS message.
- Hover on a Helm template expression no longer mentions Ansible.
- A YAML file using Go templates without Helm's objects (e.g.
  `.goreleaser.yaml`) is no longer labelled a Helm template.

### Changed

- Go module path is now `github.com/mahasenabheetha/codec/v2`; requires Go 1.26+.
- Building from source needs Node.js for the web UI (see README).

### Changed (web UI)

- Brand-new dark web UI replaces the v1 page: collapsible tool sidebar,
  tabs that keep state, Ctrl+K command palette, live transform-as-you-type
  in syntax-highlighted editors, resizable panes, and a status bar.
- JWT view shows claims as a table with expiry/validity at a glance;
  Ansible view adds line filtering and collapsible sections.
- New shortcuts: Alt+S (use output as input), Ctrl+B (sidebar), ? (help).
  Base64 swap also flips encode/decode.

### Added

- `/api/transform`: optional `indent` for json-pretty; structured `jwt`
  field in responses. `/app/` redirects to `/`.
- CI runs on Windows as well as Linux.

## [1.0.0] - 2026-09-26

First tagged release: codec as it stood before the 2.0 work began.

### Added

- CLI commands: `auto` (detect and transform), `b64 encode|decode` with
  `--url` for the URL-safe alphabet, `json pretty|min|validate`, and
  `jwt decode`. Input comes from an argument or stdin.
- `--copy` / `-c` global flag to also place output on the clipboard.
- `watch` mode: transforms recognizable clipboard content in place.
- Ansible `-vv` task log analysis: status, probable-cause diagnosis,
  summary chips, prettified commands, and severity-colored output.
- `serve`: local web UI on `127.0.0.1` with explicit modes,
  transform-on-paste, click-to-jump JSON errors, and PWA install support.
- `version` command, `--version` flag, and `GET /api/version`; the web
  UI footer shows the running version.
- Release pipeline: pushing a `v*` tag publishes binaries for Linux,
  Windows and macOS (amd64/arm64) to GitHub Releases via GoReleaser.

[Unreleased]: https://github.com/mahasenabheetha/codec/compare/v2.2.0...HEAD
[2.2.0]: https://github.com/mahasenabheetha/codec/compare/v2.1.0...v2.2.0
[2.1.0]: https://github.com/mahasenabheetha/codec/compare/v2.0.1...v2.1.0
[2.0.1]: https://github.com/mahasenabheetha/codec/compare/v2.0.0...v2.0.1
[2.0.0]: https://github.com/mahasenabheetha/codec/compare/v2.0.0-alpha.1...v2.0.0
[2.0.0-alpha.1]: https://github.com/mahasenabheetha/codec/compare/v1.0.0...v2.0.0-alpha.1
[1.0.0]: https://github.com/mahasenabheetha/codec/releases/tag/v1.0.0
