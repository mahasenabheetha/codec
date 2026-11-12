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
| Run the CLI and `codec serve` natively on macOS (arm64); not needed for the Windows-only desktop app | user |
| Make the GHCR package public (first release with an image) | user |
| Turn on GitHub Pages: Settings → Pages → `main`, `/docs` | user |
| Try Wellness in the desktop app: the tray entries (Start or pause focus, Take a break now, Pause and Resume reminders, Get codec X) and a notification from WebView2 with codec in the background (3.6.0; the build compiles, not run here) | user |
| Screenshots for the Focus and breaks docs page (`docs/wellness.html`): Focus, Breaks and a full-window break, 1400×820 (Edge headless crashed here) | user or agent |
| Apply to SignPath Foundation for Windows code signing (3.0.0 shipped unsigned; see [Code signing](#code-signing)) | user |

## Releases

Agreed 2026-10-03 (decision 73). Semantic Versioning: new features are
minor releases (2.x.0); patch releases (2.x.1) are bug fixes only.

A release's plan lives here while it is being built. Once it is
released, the plan is removed: the CHANGELOG, the [Done](#done) table
and git history keep the record, and decisions.md says why things
were built as they were.

### Routine for every version

1. Start from an up-to-date `main`: `git switch -c feature/mab/v2.X.0`.
2. Do the tasks in order. Each is finished before the next: code, tests
   for the packages it touches only, its docs page, its CHANGELOG line
   under "Unreleased". Commits: `2.X.0 T<n>: <what>`.
3. After each task, stop with a short summary and how to try it
   (`scripts/dev.sh run`); continue when the user says so.
4. Finish: `go vet ./...`, `go test ./...`, frontend build and
   `svelte-check`; docs pass over the changed pages; CHANGELOG
   "Unreleased" → `## [2.X.0] - <date>`; a row for the version in
   "Done", and its plan removed from this file (the release commit
   keeps it in git history).
5. The user pushes, opens the PR and merges; then tags `main`
   (`git tag -a v2.X.0 -m "codec v2.X.0"`, `git push origin v2.X.0`);
   the release workflow publishes ([releases.md](releases.md)). Copy the
   new binary to the local launcher folder.
6. A bug in a released version meanwhile: `fix/<topic>` from `main`,
   released as a patch (2.X.1); then merge `main` into the open version
   branch.

Work as the code already does: follow [workflow.md](workflow.md),
[conventions.md](conventions.md) and
[design-language.md](design-language.md); reuse existing components;
keep the UI consistent and polished. Work efficiently: read only what a
task needs and run only the tests for packages touched (the full suite
runs once, at step 4).

### 3.10.2: the interface refresh, the views

The second half of the refresh asked for 2026-11-12 (decision 105).
3.10.1 gave every screen its foundation (tokens, shared parts, panes
that resize, the shell); 3.10.2 goes through the views, one area at a
time, on those parts. Frontend only, no engine or API change, nothing
persisted that decision 19 rules out. Left over from 3.10.1 and done
with the view they belong to: each view's own field rules deleted
where the shared base covers them; the Helm, file and workspace
problems headers onto `PanelHeader`; loading states that are still
blank; Regex and Commands onto `ToolLayout` (they then get the
input-over-output button); the docs screenshots of Home, New, Settings,
Compare, Query and Problems.

**3.10.2 — the views** (`feature/mab/v3.10.2`)

| Task | What |
|---|---|
| T1 | Safety and reach: Escape no longer clears while typing or skips a break; "Cleared · Undo" on every clear, unpin, close and delete; every view's actions in the palette |
| T2 | Files: a toolbar with one main action per file type and the rest grouped; the explorer shows problems and what-if edits per file and folder, with a menu per kind |
| T3 | Graph: selection, keyboard pan and zoom, tooltips for cut labels, a full legend, a minimap, the view kept across what-if edits, status by icon as well as colour, stage lanes for pipelines |
| T4 | Lens views on one frame (Argo and CI onto `LensGrid`/`LensView`): one what-if indicator with reset, one error and loading pattern, sections that fold, Problems first |
| T5 | Logs and compare: controls that collapse; a histogram with a floor for rare errors, ticks and a brush window over the whole log; levels told apart; diff with a sign column, a coloured unified view, a minimap and gaps opened in steps |
| T6 | Tools: Regex and Commands on `ToolLayout`; a copy per value in JWT and URL; "Send to…" on every output (the hand-off store); one wording and one icon per action; options remembered where they are display choices |
| T7 | Settings and Wellness: themes as a swatch grid, a nav that follows the scroll, a Wellness section; breaks that trap focus, stop under reduced motion and read at card size |
| T8 | Accessibility pass (focus rings, names, tree and grid patterns, pane-width layouts), design language, docs and screenshots; release |

Not in these two: the new features the review suggested (what-if diff
on the graph, run replay, trace a value, bookmarks in the Log viewer,
cron on the zone strip). They go to "Later".

### Later
The rest of "Next", picked up when it fits.

Ideas from the 3.10.1 review: what-if diff on a graph (nodes added,
removed or skipped by a parameter); a run replayed over the Argo or
Ansible graph; one value traced from Argo CD to Helm values to the
rendered manifest; a pin in the Log viewer that restores its query,
range and highlights; a cron schedule on the time-zone strip; recipes
from chained tools.

Ideas from the 3.8.0 plan: Terraform plan viewer; certificate and CSR decoder; semver and
version-constraint tester; chmod and umask calculator; Kubernetes
quantity converter; a "Copy as…" menu (JSON path, `yq`, `kubectl patch`).

Ideas from the 3.10.0 plan: command packs for terraform, az, argo, gh,
PowerShell, crictl and kubectx; Copy as from the Commands tool to a
script or a Makefile target.

## Next

| Item | Size | Notes |
|---|---|---|
| [Expand matrix jobs in the pipeline graph](#expand-matrix-jobs-in-the-pipeline-graph) | S | A matrix job calling a reusable workflow shows as one node |
| [Kustomize helmCharts](#kustomize-helmcharts) | M | Helm SDK is already embedded |
| [Helm environment matrix](#helm-environment-matrix) | M | All profiles of a chart at once: what differs per environment |
| [Kubernetes resource totals](#kubernetes-resource-totals) | S | Requests and limits × replicas, summed |
| [RBAC view](#rbac-view) | M | Who can do what; flags wildcards and cluster-admin |
| [NetworkPolicy view](#networkpolicy-view) | M | Which pods can talk to which |

## Later

| Item | Notes |
|---|---|
| Schema-driven form for any kind | Phase 13 stretch; uses the schema cache |
| `codec lsp` | Provider concepts already mirror LSP |
| User-defined lint rules (CEL) | Per-user rules in settings, never in repos |
| Optional AI assist | Must stay opt-in and local-first |
| Several folders open at once | e.g. an app and an infra repository; search and compare across them |
| References across folders | e.g. an Argo Application pointing at a chart in another local folder; needs the item above |
| Desktop app on macOS | A macOS runner (cgo, Xcode), Apple Developer ID signing and notarization, menu bar, Dock and window-close behaviour (from 3.0.0) |
| `codec logs` | The Log viewer's engine on the command line: filters, `-o json`, stdin, an exit code for gates; saved queries (from 3.1.0); `codec logs source` for log to source (from 3.9.0) |
| Browser tests | Playwright against the sample workspace in CI: open a folder, the main views, Compare, tools, settings (skipped in 3.1.1) |
| Idle time for Wellness | Count a break taken away from the keyboard anywhere, from the Windows idle time, not only from codec (from 3.6.0) |
| Update now | After SignPath signing: download the installer, verify it against the release checksums, run it silently, restart; a winget package published by the release workflow (from 3.6.0) |
| Wails notifications | Windows toasts from the Wails notifications service instead of the web Notification API; needs the `go-toast` module (from 3.6.0) |
| Mermaid in the Markdown preview | Loaded only when a file has a diagram; source and preview side by side with scroll sync (from 3.3.0) |
| Logs: source to log | Logging calls in the Files view show how often they appear in the open logs; a click filters the log to them (from 3.9.0) |
| Logs: pipeline logs by run URL | Fetched with a token the user gives (Azure DevOps, GitHub, GitLab); needs a network and secrets decision (from 3.9.0) |
| Logs: pattern miner | Templates with typed slots; new, rare and vanished patterns against a baseline; a pattern library file; a noise and cost report (from 3.9.0) |

## Done

| Version | Items |
|---|---|
| 3.10.1 | The interface refresh, first part (decision 105): each tool and action in its own colour in every theme, with a Tool colours setting; text that grows with Interface size Large, tabular figures, rounder shapes; one look for fields, errors (with Try again), chips, view headers and the output pane, no spinners; panes you can size everywhere (lens views, Helm, Compare, the Source drawer; input over output per tool); pinned and recent tools on the rail and Home, Home's search, tabs that overflow, move and have a menu and shortcuts, a palette that remembers and lists every setting; fixes to graphs, the Helm panel, fold arrows and the log list |
| 3.10.0 | Commands (Utilities): command lines explained part by part and built with a form, two-way, for git, kubectl, helm, docker and compose and some 37 Linux tools, with risky parts flagged and a safer way; a cheat sheet searched by what you want to do; Smart paste and `codec explain`/`codec how`; Quote (a value written for bash, PowerShell, cmd, JSON, YAML, .env and more, both ways, in layers); Sanitize (secrets, emails, addresses, hosts, UUIDs, Azure names and user names out of a text or a log export, the same stand-in throughout; `codec sanitize`); breaks that rest outside active hours; fourteen sounds, a sound per moment, packs and focus sounds; the Paper, Forest, Rosé, Ocean and High contrast themes with a swatch menu |
| 3.9.0 | Log to source: file and line links, Find in workspace and Find source, in a Source drawer with a read-only peek; pipeline logs (Azure DevOps, GitHub Actions and its log zip, GitLab, Jenkins) in steps; build and infrastructure logs (BuildKit, Terraform, Helm, Maven, Gradle, npm, dotnet, go test, pytest); access logs, syslog, journald, Kubernetes events and audit logs; Smart paste for all of them; code colours in some 40 languages (CodeMirror's legacy modes, loaded per language) |
| 3.8.1 | File and folder icons from the Material Icon Theme (as in VS Code), with a Simple setting; top tabs in their tool's colour, as on the rail |
| 3.8.0 | Generators and checks: Checksum (files of any size against an expected value or a checksums file) and UUID (v4, v7, v5, v3, ULIDs read) tabs, a Password tab (rules or EFF passphrases, strength), Test data (made-up rows as JSON, CSV, YAML or SQL, lorem ipsum, seeds), a QR code tool (make and read; boombuler/barcode and gozxing), a Colour tool (formats, WCAG contrast, OKLCH shades); Smart paste for UUIDs, ULIDs and hex colours; brighter working hours with a hover column in Time zones |
| 3.7.0 | Work tabs in Smart paste, Base64, Compare, JSON & YAML and the Ansible log (one shared strip, the Log viewer's); JSON & YAML (JSONC and JSON5 read, format, tree, convert, diff by path, paths, schema, jq; `internal/jsonkit`, `codec json`); CSV & TSV in Text & data (`internal/table`, a windowed table, conversions, `codec csv`); Smart paste recognises CSV, TSV, JSONC and JSON5 |
| 3.6.0 | Wellness (focus timer with presets and custom lengths, Today, break reminders with full-window, card and toast breaks, breathing and stretch guides, chimes, status-bar widget, notifications, tray), an opt-in update check, find in tool inputs |
| 3.5.0 | Comfort: Interface size (Comfortable default, bigger icons), word buttons for Copy and Clear, the Dusk and Midnight themes with colour per tool and action, a contrast checker |
| 3.4.0 | Network tool (CIDR and overlaps, IP, URL builder, curl ⇄ fetch, status codes), Text & data (text diff, case and names), time zones and formats, commit SHA placeholders, smart paste for them |
| 3.3.0 | Markdown preview: rendered safely, links between files, the repository's images, code coloured as in the editor; find in tool outputs |
| 3.2.0 | Argo runs in the Log viewer: steps, the run's status as a step tree, lanes, live follow; the Argo graph coloured by a run |
| 3.1.1 | Review and stabilisation: static checks, fuzz tests of every parser, race detector, staticcheck and govulncheck in CI; releases on the newest Go patch |
| 3.1.0 | Log viewer: structured logs of any common format, filtered, charted and read; several logs in tabs, folds, highlights, pins and two runs compared |
| 3.0.0 | Desktop app for Windows (Wails v3): own window, tray, start at login, system folder dialog, "Open with" for YAML; per-user installer without admin rights and a portable exe |
| 2.4.0 | Light theme: System, Dark and Light in Settings and the palette |
| 2.3.0 | Utilities: Encode & hash (URL, hex, hash/HMAC, secrets & UUID, htpasswd), Time (timestamps, cron explainer and builder, schedule hover), Regex (tester and explainer); smart paste for their input |
| 2.2.0 | Ansible log analyzer: whole pipeline logs (UI and `codec ansible log`); Ansible playbook map coloured by a run |
| 2.1.0 | Compare: clear; Compare: paste as a source; Compare: pick files from the Explorer; Reload prompt after a server restart; Find in Files |
| 2.0.0 | Everything in [history.md](history.md) |

## Code signing
Unsigned builds show SmartScreen's "unknown publisher" warning. codec is
MIT-licensed and public, so it can apply to the SignPath Foundation for
free Windows signing. Their conditions, and what codec needs:

| Condition | codec |
|---|---|
| OSI licence, no proprietary components | MIT; dependencies are open source; WebView2 counts as a system library |
| Existing releases, functionality described on the download page | Releases since v2.0.0; README and docs site |
| Binaries built from source by CI, verifiably | Release workflow on GitHub Actions with SignPath's action |
| MFA on GitHub and SignPath for every team member | User to turn on |
| Roles: authors, reviewers, approvers; manual approval per release | The user holds all three |
| Code signing policy page on the homepage, with attribution and a privacy statement | A `docs/` page: "Free code signing provided by SignPath.io, certificate by SignPath Foundation"; codec sends no data anywhere |
| Product name and version metadata on signed binaries | Set by the Windows build (3.0.0) |
| Uninstaller; no undisclosed data collection or system changes | Installer has one; nothing collected |

The certificate names SignPath Foundation as the publisher, not the
author. SignPath signs Windows only; macOS needs Apple's program.

## Items

### Expand matrix jobs in the pipeline graph
- **Problem:** a GitHub job with a `matrix` that calls a local reusable
  workflow is drawn as one node; the CLI already knows it runs × N.
- **Result:** one node per combination (or a badge "× 3" with the list
  on click), consistent with plain matrix jobs.
- **Scope:** `internal/ci` graph building, the Pipeline view.
- **Done when:** the sample's `ci.yml` shows three `test` runs; ci tests cover it.

### Kustomize helmCharts
- **Problem:** kustomizations using `helmCharts` can't be built; codec
  says so and points to the Helm view.
- **Result:** charts vendored in the repository are inflated with the
  embedded Helm SDK; remote charts stay unsupported (decision 3's spirit:
  never download).
- **Scope:** `internal/kube` (kustomize plugin hook), `internal/helm`.
- **Done when:** a kustomization with a local chart builds like `kubectl kustomize --enable-helm`.

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
