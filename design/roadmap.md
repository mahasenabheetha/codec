# Roadmap and backlog

codec 2.0 is complete ([history.md](history.md)). New work — features,
improvements, known gaps — is listed here first, so everyone (people and
agents) picks it up the same way. Bugs go to GitHub issues; a bug that
needs a design change gets an item here too.

## How to add an item

Add a row to the right table, then a short section under
[Items](#items) using this template:

```markdown
### <short name>
- **Problem:** what is hard or impossible today, for whom.
- **Result:** what the user sees when it is done (UI and CLI).
- **Scope:** packages likely touched; what is explicitly out of scope.
- **Done when:** checks that prove it (tests, a screenshot on the sample, docs updated).
- **Decisions:** existing ones it relies on; new ones it needs (add to decisions.md when agreed).
```

Pick up an item by following [workflow.md](workflow.md#adding-a-feature).
When it ships, delete its section, move the row to "Done" with the
version, and add a CHANGELOG entry.

## Open checks

| Check | Owner |
|---|---|
| Run codec natively on macOS (arm64) before tagging | user |
| Confirm no open issues labelled for 2.0 | user |
| Merge `feature/mab/yaml-tools` and tag `v2.0.0` | user |
| Make the GHCR package public (first release with an image) | user |
| Turn on GitHub Pages: Settings → Pages → `main`, `/docs` | user |

## Releases

Planned order (agreed 2026-10-03). Semantic Versioning: new features
are minor releases (2.x.0); patch releases (2.x.1) are bug fixes only.
Each item gets its own short branch from `main` (decision 72) and its
CHANGELOG line under "Unreleased"; a version is tagged when its items
are merged, following [releases.md](releases.md). After a release, copy
the new binary to the local launcher folder.

| Version | Items, in build order | Notes |
|---|---|---|
| 2.1.0 | [Compare: clear](#compare-clear) → [Compare: paste as a source](#compare-paste-as-a-source) → [Compare: pick files from the Explorer](#compare-pick-files-from-the-explorer), [Reload prompt after a server restart](#reload-prompt-after-a-server-restart), then [Find in Files](#find-in-files) | Find in Files is last: if it runs long, 2.1.0 ships without it and it moves to the next release |
| 2.2.0 | [Ansible log analyzer: whole runs](#ansible-log-analyzer-whole-runs) → [Ansible playbook map](#ansible-playbook-map) | The map's run colours need the analyzer; if the map runs long, release the analyzer as 2.2.0 and the map next |
| 2.3.0 | [Utilities: Encode & hash](#utilities-encode--hash) → [Utilities: Time](#utilities-time) → [Utilities: Regex](#utilities-regex) | Encode & hash first: it sets up the Utilities heading and sub-tabs |
| later | The rest of "Next" | Picked up after 2.3.0, or slotted in when it fits |

Before starting 2.1.0, merge the branch that added these plans.

## Next

| Item | Size | Notes |
|---|---|---|
| [Expand matrix jobs in the pipeline graph](#expand-matrix-jobs-in-the-pipeline-graph) | S | A matrix job calling a reusable workflow shows as one node |
| [Reload prompt after a server restart](#reload-prompt-after-a-server-restart) | S | Today requests fail until the page is reloaded |
| [Kustomize helmCharts](#kustomize-helmcharts) | M | Helm SDK is already embedded |
| [Window long lists](#window-long-lists) | S | Problems and Resources cards, like TreeView |
| [Compare: paste as a source](#compare-paste-as-a-source) | M | Third source per side; works with no folder open |
| [Compare: pick files from the Explorer](#compare-pick-files-from-the-explorer) | M | Drag and drop, plus a new Explorer context menu |
| [Compare: clear](#compare-clear) | S | Per-side × and a Clear button |
| [Find in Files](#find-in-files) | M | Text search across the workspace; read-only, no replace |
| [Helm environment matrix](#helm-environment-matrix) | M | All profiles of a chart at once: what differs per environment |
| [Kubernetes resource totals](#kubernetes-resource-totals) | S | Requests and limits × replicas, summed |
| [RBAC view](#rbac-view) | M | Who can do what; flags wildcards and cluster-admin |
| [NetworkPolicy view](#networkpolicy-view) | M | Which pods can talk to which |
| [Utilities: Time](#utilities-time) | M | Timestamp and Cron (explainer + generator) in one sidebar entry |
| [Utilities: Encode & hash](#utilities-encode--hash) | S | URL, hex, hash/HMAC, secrets & UUID, htpasswd |
| [Utilities: Regex](#utilities-regex) | M | Tester and explainer; Go (RE2) and Python/.NET/JS styles |
| [Ansible log analyzer: whole runs](#ansible-log-analyzer-whole-runs) | L | Whole pipeline logs: runs, plays, tasks, hosts; resilient to mixed output |
| [Ansible playbook map](#ansible-playbook-map) | M | Diagram of plays, roles, includes, handlers; coloured by a loaded run |

## Later

| Item | Notes |
|---|---|
| Schema-driven form for any kind | Phase 13 stretch; uses the schema cache |
| `codec lsp` | Provider concepts already mirror LSP |
| User-defined lint rules (CEL) | Per-user rules in settings, never in repos |
| v3 desktop app (Wails) | The frontend talks to the backend only through `lib/api`; swap the transport |
| Optional AI assist | Must stay opt-in and local-first |
| Several folders open at once | e.g. an app and an infra repository; search and compare across them |
| References across folders | e.g. an Argo Application pointing at a chart in another local folder; needs the item above |

## Done

| Version | Items |
|---|---|
| 2.0.0 | Everything in [history.md](history.md) |

## Items

### Expand matrix jobs in the pipeline graph
- **Problem:** a GitHub job with a `matrix` that calls a local reusable
  workflow is drawn as one node; the CLI already knows it runs × N.
- **Result:** one node per combination (or a badge "× 3" with the list
  on click), consistent with plain matrix jobs.
- **Scope:** `internal/ci` graph building, the Pipeline view.
- **Done when:** the sample's `ci.yml` shows three `test` runs; ci tests cover it.

### Reload prompt after a server restart
- **Problem:** each `codec serve` run has a new token, so a page left
  open fails with "can't reach the server" until reloaded.
- **Result:** the UI detects the token mismatch (401 with a known body)
  and shows "codec restarted — Reload".
- **Scope:** `internal/web` (distinguish token mismatch), `frontend/src/lib/api/client.ts`, shell.
- **Done when:** restarting codec shows the prompt; reload restores tabs.

### Kustomize helmCharts
- **Problem:** kustomizations using `helmCharts` can't be built; codec
  says so and points to the Helm view.
- **Result:** charts vendored in the repository are inflated with the
  embedded Helm SDK; remote charts stay unsupported (decision 3's spirit:
  never download).
- **Scope:** `internal/kube` (kustomize plugin hook), `internal/helm`.
- **Done when:** a kustomization with a local chart builds like `kubectl kustomize --enable-helm`.

### Window long lists
- **Problem:** the Problems view and Resources cards render every row;
  a repository with thousands of findings or objects gets slow.
- **Result:** only visible rows are in the DOM, like TreeView (decision 70).
- **Scope:** `frontend/src/features/lint`, `features/kube`, possibly a shared list component.
- **Done when:** 5,000 findings scroll smoothly; `scripts/perf.sh` notes it.

### Compare: paste as a source
- **Problem:** Compare only takes workspace files and Helm renders, and
  needs a folder open. Comparing a live object (`kubectl get -o yaml`),
  two API responses or two snippets means saving them as files first.
- **Result:** each side's source is File · Helm render · **Paste**; a
  Paste side is an editor box. Both sides pasted, or one pasted against
  a file or render. Comparing two pasted sides works with no folder open,
  reachable from Home and Tools ("Compare text") as well as the Compare tab.
  - Compared by structure (as today) only when both sides parse to a
    mapping or list; YAML and JSON mix freely (JSON is YAML). Anything
    else — plain text, logs, XML, ini, a lone scalar — gets the text diff,
    so two paragraphs don't show as one "value changed".
  - "Compare as: Auto · Structure · Text" (the API's `text` flag already
    forces a text diff); "Ignore whitespace" and "Ignore case" for text.
  - A "Live object noise" ignore preset: `metadata.managedFields`,
    `resourceVersion`, `uid`, `creationTimestamp`, `generation`, `status`.
- **Scope:** `internal/web/compare_api.go` (a `paste` side kind; no
  workspace needed when neither side reads one), `internal/textdiff`
  (whitespace/case options), `features/compare` (SidePicker, paste editor),
  Home and Tools entries, docs `compare.html`. Out of scope: per-format
  parsers (XML, ini); saving pasted text.
- **Done when:** compare_api tests cover paste × paste with no folder,
  paste × file, the scalar-to-text rule and the forced modes; the sample's
  Deployment against a pasted live copy with the noise preset shows only
  real changes; docs updated.
- **Decisions:** 19 (pasted text is never persisted, so it is gone after
  a reload — agreed), 44 (semantic diff identity). New: Paste is a third
  source per side, not a separate screen (agreed 2026-10-02).

### Compare: pick files from the Explorer
- **Problem:** a side's file can only be chosen through the quick-open
  search; the Explorer tree can't feed Compare.
- **Result:** drag a file from the Explorer onto the Left or Right side;
  the side shows "Choose a file… or drop one here". A new Explorer
  context menu (right-click) has "Select for compare", "Compare with
  selected", and with two files Ctrl+clicked, "Compare selected". The
  search stays. No second tree inside Compare, and a plain click in the
  Explorer still opens the file.
- **Scope:** `features/workspace` (Explorer/TreeView: context menu,
  drag source, Ctrl+click selection), `features/compare/SidePicker.svelte`
  (drop target), a shared context-menu component in `lib/components`,
  docs `compare.html` and `workspace.html`, `shortcuts.html`. Out of
  scope: other menu entries (copy path, reveal, lint this file) — the
  menu is built so they can be added later.
- **Done when:** dragging and each menu entry open the right comparison on
  the sample; the menu works with the keyboard (Shift+F10 / context-menu
  key, arrows, Esc); docs updated.
- **Decisions:** agreed 2026-10-02: drag and drop plus a context menu,
  one Explorer tree only, click keeps meaning "open".

### Compare: clear
- **Problem:** resetting Compare means re-picking both sides by hand.
- **Result:** an × on each side clears that side; "Clear" next to Swap
  empties both sides and the result. Saved ignore patterns stay. No
  confirmation; when a pasted side is cleared, a brief "Cleared · Undo"
  toast brings it back (memory only, decision 19).
- **Scope:** `features/compare` (CompareView, SidePicker, compare.svelte.ts).
- **Done when:** both buttons reset the view; Undo
  restores pasted text; docs `compare.html` updated.
- **Decisions:** none new.

### Find in Files
- **Problem:** text can only be found inside one open file (Ctrl+F).
  Quick open matches file names, and Query (jq) sees YAML values only —
  not comments, keys as text, or non-YAML files (README, scripts, raw
  templates).
- **Result:** a Search view in the sidebar beside the Explorer
  (Ctrl+Shift+F, also in the palette). A word, phrase or regular
  expression; match case and whole word; include/exclude globs
  (`charts/**`, `!**/tests/**`). Results are grouped by file with the
  matching line and the match highlighted; clicking opens the file at
  that line. Optional CLI: `codec search "phrase" [path]`.
  - Read-only: no replace (decision 3).
  - Searches the files the Explorer lists (same skipped folders and
    .gitignore), skips binaries and files over the size limit, caps the
    results ("showing first N") and cancels when the query changes.
- **Scope:** a new `internal/search` package (pure: files in, matches
  out), `internal/web` (`POST /api/v2/search`), a new
  `features/search` view, the sidebar and palette, optionally
  `internal/cli/search.go`; docs `workspace.html`, `shortcuts.html`,
  `cli.html`. Out of scope: replace, an index (plain scanning is enough
  at the 8 MB-per-file limit), searching outside the open folder.
- **Done when:** search tests cover case, whole word, regex, globs,
  binary skip and the result cap; finding a phrase on the sample opens
  the right file and line; a large repository (`scripts/perf.sh`) returns
  first results quickly; docs updated.
- **Decisions:** 3 (read-only). New: Find in Files sits beside Query —
  text vs structure — rather than replacing it.

### Helm environment matrix
- **Problem:** Compare shows two renders at a time; seeing how dev, test
  and prod drift apart means many pairwise compares.
- **Result:** for a chart, render it with every saved profile (and the
  defaults) and show one table: objects present per profile, and the
  fields that differ (replicas, images, resources, env, hosts), one
  column per profile. A cell opens the pairwise Compare for those two.
- **Scope:** `internal/helm` (render each profile, reuse the semantic
  diff), `internal/web`, `features/helm`; docs `helm.html`. Out of scope:
  values files that aren't saved profiles; rendering in parallel beyond
  what the Helm view already does.
- **Done when:** the sample chart with two or more profiles shows the
  expected differences; helm tests cover the table; docs updated.
- **Decisions:** 44 (semantic diff identity); profiles stay in the user
  profile, never in the repository.

### Kubernetes resource totals
- **Problem:** how much a release asks of the cluster (CPU, memory) is
  scattered across containers and replica counts.
- **Result:** in Resources (and per Helm render): requests and limits
  per workload multiplied by replicas, summed per namespace and overall,
  with init containers counted as Kubernetes does (max, not sum) and
  containers without values listed as "not set". HPA min/max shown when
  present.
- **Scope:** `internal/kube` (quantity parsing and sums), `features/kube`;
  docs `kubernetes.html`.
- **Done when:** kube tests cover quantities (`100m`, `1.5`, `512Mi`,
  `1G`), replicas, init containers and missing values; the sample shows
  correct totals.
- **Decisions:** none new.

### RBAC view
- **Problem:** what a ServiceAccount, user or group may do is spread over
  Roles, ClusterRoles and their bindings.
- **Result:** a table per subject: verbs × resources × namespace, with
  the binding and role it comes from. Flags `*` verbs or resources,
  `cluster-admin` bindings and bindings to roles not in the repository.
- **Scope:** `internal/kube` (graph already links objects), `features/kube`,
  possibly lint rules; docs `kubernetes.html`. Out of scope: aggregated
  ClusterRoles from the cluster, built-in roles beyond a known list.
- **Done when:** kube tests cover Role vs ClusterRole bindings and
  wildcards; the sample shows its ServiceAccount's rights.
- **Decisions:** none new.

### NetworkPolicy view
- **Problem:** NetworkPolicy YAML is hard to read: which pods can reach
  which, on which ports, is not obvious.
- **Result:** per namespace, a diagram and a table: for each workload,
  allowed ingress and egress (pods, namespaces, CIDRs, ports), and
  workloads not selected by any policy (allow all). States clearly that
  it shows the repository's policies, not a cluster's.
- **Scope:** `internal/kube` (selector matching exists), `features/kube`;
  docs `kubernetes.html`. Out of scope: CNI-specific policies (Calico,
  Cilium CRDs).
- **Done when:** kube tests cover podSelector, namespaceSelector, ipBlock,
  ports and default deny; the sample has a policy to show.
- **Decisions:** none new.

### Utilities (applies to the three items below)
- A new rail heading **Utilities** with three entries — Time, Encode &
  hash, Regex — each with sub-tabs (`SegmentedControl`, as in Compare).
  The last sub-tab is remembered (a UI preference, decision 19).
- Each sub-tab is in the command palette and on Home by its own name
  ("Cron" opens Time on the Cron tab). `lib/tools.ts` gains optional
  sub-tabs per tool so the rail, Home and palette stay generated from it.
- Same layout as the Base64 and JWT tools: input, options, output, copy.
  Base64, JSON and JWT stay where they are.
- Logic is Go, as for the existing tools (decision 18): a package in
  `internal/` per area, exposed through the API and as CLI commands.
  Input and output are never persisted (decision 19).
- Smart paste learns to recognise epoch numbers, URL-encoded text and
  cron expressions and opens the right sub-tab.

### Utilities: Time
- **Problem:** epoch timestamps and cron schedules are read by hand or
  in a browser tab elsewhere.
- **Result:** rail entry **Time** with two sub-tabs.
  - **Timestamp:** epoch seconds or milliseconds (detected) to local,
    UTC, ISO 8601 and relative time; a date back to epoch; a **Now**
    button with the current Unix timestamp.
  - **Cron:** explainer and generator in one: an expression shows plain
    words ("Every 15 minutes, 02:00–06:59, Monday to Friday") and the
    next 10 runs; a form (every N minutes, hourly at, daily at, weekdays,
    day of month) builds the expression live, and a pasted expression
    fills the form where it can. Standard 5 fields plus `@daily`-style
    shortcuts, with a time-zone choice (UTC for GitHub/Azure; a CronJob's
    `timeZone`). Quartz (6–7 fields, `?`) and Jenkins `H` are recognised
    and named as unsupported.
  - Hover on a schedule in the editor (CronJob `schedule`, GitHub
    `on.schedule.cron`, Azure `schedules.cron`) shows the explanation and
    next runs.
- **Scope:** `internal/timeutil` (or similar) and `internal/cron`
  (parser, next runs, describe, build), `internal/web`, CLI
  `codec time` / `codec cron`, `features/time`, the providers' hover;
  docs `tools.html`, `cli.html`.
- **Done when:** cron tests cover ranges, steps, lists, names (`MON`),
  shortcuts, day-of-month vs day-of-week, DST changes and the dialect
  messages; timestamp tests cover s/ms detection and time zones; hover
  works on the sample's CronJob and workflow; docs updated.
- **Decisions:** agreed 2026-10-03: standard 5 fields + shortcuts,
  explainer and generator as one tool, hover tips in the editor.

### Utilities: Encode & hash
- **Problem:** small encode/hash jobs need other tools or websites,
  which is risky with secrets.
- **Result:** rail entry **Encode & hash** with sub-tabs:
  - **URL:** encode/decode a value or a whole URL; a query string as a
    table.
  - **Hex:** hex ↔ text.
  - **Hash:** MD5 (marked "checksums only"), SHA-1, SHA-256, SHA-384,
    SHA-512 of text, and HMAC with a key.
  - **Secrets & UUID:** UUID v4; random secrets by length and character
    set, as text, hex or base64.
  - **htpasswd:** bcrypt `user:hash` lines for ingress basic auth.
- **Scope:** `internal/codec` (or a sibling package), `internal/web`,
  CLI commands (`codec url`, `codec hash`, …), `features/encode`; docs
  `tools.html`, `cli.html`. Out of scope: generic encrypt/decrypt (no
  interoperable format — agreed); Ansible Vault belongs to the Ansible work.
- **Done when:** tests use published test vectors for each hash and
  HMAC; bcrypt output verifies; docs updated.
- **Decisions:** agreed 2026-10-03: no generic encryption tool; bcrypt
  via `golang.org/x/crypto` (new dependency).

### Utilities: Regex
- **Problem:** regexes from Ansible, pipelines and tooling are tested by
  trial and error.
- **Result:** rail entry **Regex**, one screen: the pattern, flags as
  toggles, test lines with matches highlighted live, capture groups in a
  table, a replace preview (`$1`, `${name}`), and an explainer tree that
  describes each part in plain words; hovering a part highlights it in
  the pattern and the matches.
  - Two styles: **Go (RE2)** with the standard library, and
    **Python / .NET / JavaScript** with `dlclark/regexp2` (lookarounds,
    backreferences, `(?<name>)` and `(?P<name>)`). Each test has a time
    limit, so a pattern that backtracks badly stops with a message.
- **Scope:** `internal/regex` (run, explain: `regexp/syntax` for RE2,
  a small parser for the extra features), `internal/web`, CLI
  `codec regex`, `features/regex`; docs `tools.html`, `cli.html`.
- **Done when:** tests cover both styles, named groups, flags, replace,
  the time limit and the explainer for each construct; docs updated.
- **Decisions:** agreed 2026-10-03: logic in Go with two styles, tester
  and explainer as one tool; `dlclark/regexp2` (new dependency).

### Ansible log analyzer: whole runs
- **Problem:** the log tool reads one task block (the first result line
  and its JSON). Whole logs from pipelines — CI timestamps, wrapper
  prefixes, bash and other tools' output, several playbook runs — are
  not understood, and stray text can break the parse.
- **Result:** paste or drop a whole log (tens of MB; never persisted).
  - Summary bar: runs, hosts, failed/changed counts, duration (when the
    log has timestamps), and the "other output" blocks.
  - Left: an outline Run → Play → Task with per-host status and counts,
    filters (Failed · Changed · Skipped · All), a host picker and search;
    the first failure is selected on open; windowed for long logs.
  - Right: today's single-task view as the detail of the selected task
    and host (cause, fields, command/stdout/stderr, "Defined in").
  - PLAY RECAP as a table per host.
  - Text outside Ansible runs is kept as collapsed "other output" blocks
    (line count, error/warning count, existing line colouring), so a
    failure outside Ansible is visible ("Ansible succeeded; the step
    failed after it").
- **How (four stages, each degrading instead of failing):**
  1. Clean: strip ANSI and `\r` redraws; strip CI line prefixes
     (Azure DevOps / GitHub / GitLab timestamps, `##[group]`,
     `section_start:`), keeping timestamps for durations; learn wrapper
     prefixes (Packer `azure-arm:`, Compose `web  | `) from the prefix
     seen before an Ansible anchor, instead of hard-coding them.
  2. Segment: only Ansible's own anchors decide what is Ansible —
     `PLAY [`, `TASK [`, `RUNNING HANDLER [`, `PLAY RECAP`, result lines
     (`ok/changed/skipping/failed/fatal: [host]`), `included:`,
     `...ignoring`, `[WARNING]:`, `[DEPRECATION WARNING]:`, `ERROR!`. A run
     starts at the first PLAY and ends after its recap; several runs per
     log. Unrecognised lines inside a task (`-vvv` SSH debug, interleaved
     output) attach to the task as "other lines".
  3. Read results: find the end of a JSON payload by brace matching that
     respects strings; else try YAML (`yaml` stdout callback); else keep
     the raw text. Loops (`item=`), retries, ignored, rescued, unreachable,
     `no_log`. The `json` stdout callback (one document) is read exactly.
  4. Model: Run → Plays → Tasks → host results (status, items, retries,
     payload, other lines) + recap + durations.
- **Scope:** a new log package beside `internal/codec/ansible.go` (the
  single-task parser stays for the detail view and Smart paste),
  `internal/web` (transform or a new endpoint), CLI
  (`codec ansible log <file>`: summary and failures), `features/ansible`;
  docs `ansible.html`, `tools.html`, `cli.html`. Out of scope:
  interpreting non-Ansible output beyond line colouring; guessing
  without an anchor; special handling for `strategy: free` (results
  attach to the latest task header; odd cases get a note); mini tools
  (idempotency check, timeline, grid, Vault).
- **Done when:** a fixture set of synthetic logs passes — default
  output, `-v` to `-vvv`, `yaml` and `json` callbacks, Packer and Compose
  prefixes, Azure/GitHub/GitLab timestamps, interleaved bash, two
  playbooks, `ERROR!` before a run, unreachable, loops, retries,
  `include_tasks`, handlers, rescue, ignore_errors; a fuzz test shows the
  parser never panics and never drops lines; a 50 MB log stays
  responsive; docs updated.
- **Decisions:** 18 (structured data from the API), 19 (never
  persisted). Agreed 2026-10-03: four stages; "other output" kept and
  collapsed; no interpretation of non-Ansible output; generic formats,
  not tailored to one pipeline.

### Ansible playbook map
- **Problem:** the Ansible view lists the execution order; the shape
  of a playbook — which roles and files it pulls in, and how — is hard
  to see.
- **Result:** a diagram per playbook (layout like the CI pipeline graph):
  play → roles (with `meta` dependencies) → included task files →
  nested includes, and handlers off the tasks that notify them. Static
  `import_*` edges are solid; dynamic `include_*` edges are dashed,
  resolved and marked "assumed" when the variable has exactly one
  definition, otherwise listing the candidates. External roles
  (Galaxy, collections) are marked. Clicking a node opens its file.
  - With a run loaded in the log analyzer, nodes are coloured by that
    run: failed, changed, ok, never ran (agreed).
- **Scope:** `internal/ansible` (graph from the existing execution-order
  analysis), `internal/web`, `features/ansible` (reuse the pipeline
  graph component); docs `ansible.html`. Out of scope: run replay on
  the execution list; Jinja evaluation.
- **Done when:** the sample's `site.yml` map shows roles, includes and
  handlers; ansible tests cover static vs dynamic includes, the
  single-definition resolution and external roles; colours match a
  sample log; docs updated.
- **Decisions:** agreed 2026-10-03: V2 only for visuals, with run
  colours; no Jinja evaluator.
