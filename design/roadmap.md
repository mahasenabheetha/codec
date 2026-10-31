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

### v3.7.0 — JSON and table tools, work tabs
Agreed 2026-10-25 (decision 90). The two heavier tools: working with
structured data pasted from anywhere. The JSON tool grows sub-tabs and
YAML ⇄ JSON lives in it (a paste in one gives the other); CSV and TSV
join Text & data. Asked for 2026-10-31 (decision 97): Smart paste,
Base64 and Compare get tabs like the Log viewer, several pastes or
comparisons open at once; it comes first.

| # | Task | Main parts | Status |
|---|---|---|---|
| T1 | Work tabs | A tab strip like the Log viewer's for Smart paste, Base64 and Compare: each tab its own input (or sides) and result; "+" for a new one, close (×, middle click, Ctrl+W), arrow keys between tabs, double-click to rename; names from what's in it (the detected kind, the mode, the two sides). One shared component and store, which the Log viewer moves onto too, so all four behave alike. Tabs last for the window: pasted text is never stored (19); docs | Done: `shared/worktabs.svelte.ts` (`WorkTabs`: list, current, add, step, close, optional keep-one and Undo for tabs holding pasted text; `workTabShortcuts` with Alt+T and Alt+W, since Ctrl+T and Ctrl+W are the browser's) and `WorkTabStrip.svelte` (the Log viewer's look; select, ×, middle click, arrow keys, double-click rename, a badge snippet, a failed state); `ToolLayout` gains a `tabs` slot. Smart paste and Base64: today's component is the tab (`SmartTab`, `Base64Tab`) with its state in a tab object, a small root renders the strip and every tab, the current one shown. Compare: `Comparison` per tab; `compareTabs` (exported as `comparison` too) opens files, renders and pastes in the current tab when empty, else a new one; the ignored paths and the picked file are shared; `TextDiff` takes the tab's options. Log viewer: `LogTabs` extends `WorkTabs`, its strip is the shared one. Names: detected kind (Smart paste), direction and number (Base64, never content), sides or Pasted n (Compare). Checked in the browser: two Smart paste tabs (JWT, base64), rename, close and Undo; two Base64 tabs with their own direction; two Compare tabs with their own results, Alt+T and Alt+W, Compare selected from the Explorer opening a new tab beside a pasted one; the Log viewer's sample tab with its count and closing to the empty state. Docs (tools, compare, logs, shortcuts, design language), CHANGELOG |
| T2 | JSON tools: engine + API | `internal/jsonkit` over the existing scanner: JSON ⇄ YAML (key order kept), JSONC and JSON5 read, sort keys, flatten to paths and back, a diff by path (added, removed, changed, type changed), infer a JSON Schema from samples, jq filter (the query engine is there), large input read without a tree; `codec json` gains the subcommands; a fuzz target | Done: `internal/jsonkit`: its own reader for JSON, JSONC and JSON5 (comments, trailing commas, unquoted keys, single quotes, hex, +, .5 and 5., escapes such as \x41 and line continuations; Infinity and NaN refused, having no JSON form; repeated keys: the last wins in the first's place; 1000 levels at most; errors with line and column in characters; the dialect reported), `Value` keeping key order and number text, `Encode` writing strict JSON; `SortKeys`; `Flatten`/`Unflatten` with jq-style paths (odd keys in brackets, gaps filled with null, indexes up to 100 000); `Diff` by path (added, removed, changed, type; numbers by value); `Infer` (draft 2020-12: types, properties in first-seen order, required, items, string formats); `Query` on gojq (no environment, 1000 results); `ToYAML`/`FromYAML` through yamlkit; `Scan` counting tokens of strict JSON with flat memory. `POST /api/v2/json/{format,to-yaml,from-yaml,flatten,unflatten,diff,schema,query,stats}` (422 with line and column; jq stopped after 5 s); `codec json` pretty and min read JSONC/JSON5 with `--sort`, validate says when text is JSONC or JSON5, plus yaml, from-yaml, flatten, unflatten, diff (exit 2), schema, query, stats. Tests for every function and the API; a fuzz target (Parse then Encode reads back as strict JSON and the same value, no diff with itself, flatten round trip, sorting stable) found an identity escape followed by invalid UTF-8 slipping through, fixed and kept as a seed; 6.3 million runs clean after |
| T3 | JSON tools: UI | The JSON tool grows sub-tabs: Format, Tree (search, copy a value's path, fold), Convert (JSON ⇄ YAML), Diff, Paths, Schema, Query; the tool is named JSON & YAML in the rail and palette; docs | |
| T4 | CSV and TSV | `internal/table`: delimiter and header guessed (override), quotes and embedded newlines; viewer with a windowed table (sort, filter, column types and counts); converts to JSON (typed or all text), Markdown table, YAML and TSV/CSV, and a JSON array of objects back to CSV; `codec csv`; a fuzz target | |
| T5 | Smart paste | Recognises CSV or TSV and JSON5 or JSONC, and offers the tool | |
| — | Release | Full checks; a fuzz run of the new parsers; a review pass over the engines and the UI; docs; CHANGELOG cut as 3.7.0 | |

### v3.8.0 — Generators and checks
Agreed 2026-10-25 (decision 90). Things that make something: checksums,
identifiers, passwords, test data, QR codes and colours. Nothing made is
stored. The QR library is the one new dependency of the three releases
and is agreed before it is added.

| # | Task | Main parts | Status |
|---|---|---|---|
| T1 | Checksum and UUID | In Encode & hash: **Checksum** (hash a dropped file, or a file in the open folder, streamed on the server and never kept; compare with an expected value pasted as hex, base64 or a `sha256sum` line, or verify several files against a `checksums.txt`); **UUID** (v4, v7, v5 and v3 from a namespace and name, parse one for version, variant and time, bulk up to 1000, formats) | |
| T2 | Password generator | Rules: length, classes with a minimum each, exclude ambiguous or chosen characters, presets such as "14+ characters, 4 classes"; a passphrase mode with an embedded word list (the EFF list is CC-BY, so its attribution goes in the docs); entropy shown; `crypto/rand` with no modulo bias; shown once, never stored; `codec secret` gains the rules | |
| T3 | Test data and lorem | `internal/fake`: lorem ipsum (words, sentences, paragraphs), and rows of fake data (names, emails on reserved example domains, IPs from documentation ranges, UUIDs, dates, numbers, choices) from columns you pick, as JSON, NDJSON, CSV, YAML or SQL `INSERT`; a seed for repeatable output; `codec fake` | |
| T4 | QR code | Generator (text, URL, Wi-Fi, TOTP URI; error correction level; PNG and SVG to save) and reader (an image dropped or pasted, decoded in the browser; no camera); browser-only | |
| T5 | Colour | HEX, RGB, HSL, HSV and a CSS name; WCAG contrast ratio with AA and AAA; a picker and shades; browser-only arithmetic | |
| T6 | Smart paste | Recognises a UUID or ULID and a hex colour and offers the tool | |
| — | Release | Full checks; a fuzz run of the new parsers; a review pass; docs (`tools.html`, `security.html`: nothing generated is stored); CHANGELOG cut as 3.8.0 | |

Later: Terraform plan viewer; certificate and CSR decoder; semver and
version-constraint tester; chmod and umask calculator; Kubernetes
quantity converter; a "Copy as…" menu (JSON path, `yq`, `kubectl patch`).

### Later
The rest of "Next", picked up when it fits.

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
| `codec logs` | The Log viewer's engine on the command line: filters, `-o json`, stdin, an exit code for gates; saved queries (from 3.1.0) |
| Browser tests | Playwright against the sample workspace in CI: open a folder, the main views, Compare, tools, settings (skipped in 3.1.1) |
| Idle time for Wellness | Count a break taken away from the keyboard anywhere, from the Windows idle time, not only from codec (from 3.6.0) |
| Update now | After SignPath signing: download the installer, verify it against the release checksums, run it silently, restart; a winget package published by the release workflow (from 3.6.0) |
| Wails notifications | Windows toasts from the Wails notifications service instead of the web Notification API; needs the `go-toast` module (from 3.6.0) |
| Mermaid in the Markdown preview | Loaded only when a file has a diagram; source and preview side by side with scroll sync (from 3.3.0) |

## Done

| Version | Items |
|---|---|
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
