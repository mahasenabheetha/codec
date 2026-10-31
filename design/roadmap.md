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

### v3.6.0 — Update check and Wellness
Agreed 2026-10-06 (decision 85). Wails has no updater and codec never
looks for new versions, so an installed copy stays old until the user
notices a release. codec can ask GitHub for the latest release and say
when a newer one exists; updating stays the installer run over the top
(it replaces the install in place and keeps settings).

The release also brings **Wellness**, a new sidebar section (decision
96, 2026-10-31, widening 91): a focus timer and break reminders for
long days at the screen, asked for "with the best UI possible". One
tool, **Focus & breaks**, with sub-tabs Focus, Breaks and Today. It is
frontend only except for system notifications in the desktop app
(the Wails notifications service, no new dependency), runs the same in
the browser UI and the desktop app, and keeps its settings, a running
timer (so a reload doesn't lose it) and today's totals in the browser's
storage, nothing else: no history beyond today, nothing sent anywhere.
Everything is off until started or switched on.

Order (agreed 2026-10-31): the Wellness tasks T3–T6 first, then the
update check T1–T2, then the release.

| # | Task | Main parts | Status |
|---|---|---|---|
| T1 | Check: engine + API + CLI | `internal/update`: latest release from the GitHub API (pre-releases only when running one), semver compare with `internal/version`, 5 s timeout, at most once a day (result cached in the config dir), dev builds never check; `updates.check` setting, off by default; `GET /api/v2/update`; `codec version --check`; tests against a fake server | Done: `internal/update` (`Checker` with the GitHub releases list, drafts dropped, pre-releases only for a pre-release build, its own SemVer order with build metadata ignored, a 5 s limit, the answer cached a day per version in `update-check.json` in the settings folder, a development build never asks); `config.Updates.Check`, off; `GET /api/v2/update` (asks nothing when off unless `?force=1`, a failed check reported in `error`, not as a failure) and `POST /api/v2/update/settings`; `Options.Updates` for tests; `codec version --check`. Tests: the semver.org precedence list both ways, rejected forms, the newest release per case, dev builds, the day's cache (forced, older, another version), a 403 and a server that never answers; the API off, forced and on, against a fake GitHub. Rules in AGENTS.md, workflow.md and the docs now name the release check as the second outbound request; security and CLI pages |
| T2 | Notice in the UI | Settings → General: "Check for updates" switch and "Check now"; "codec X.Y.Z is available" in the footer with a link to the release (the system browser in the desktop app), dismissable per version; docs (settings, security: the second network use, troubleshooting) | Done: `lib/api/update.ts`, `stores/update.svelte.ts` (loaded once when the app starts; the dismissed version kept as a display preference); Settings → General **Check for updates** (switch, Check now, and a line with the answer: up to date and when, newer with Release notes and download, a development build, or why it couldn't check); the status bar's "codec X is available" with the link (`openLink`: the system browser in the desktop app) and a dismiss; the desktop tray's "Get codec X…" from `Server.NewerRelease`, which reads the day's answer only (`Checker.Cached`) and never asks GitHub. Tests: `NewerRelease` before and after a check. Checked in the browser with a throwaway settings folder: the switch saving, Check now on a development build, and (with the answer stood in for in the page) the Settings line and the status-bar notice. Docs: settings, troubleshooting (blocked or rate-limited), security (T1), CHANGELOG |
| T3 | Wellness frame, clock and sounds | A **Wellness** group on the rail with **Focus & breaks** (sub-tabs Focus, Breaks, Today; Home and palette entries) and its own hue. `features/wellness/clock.svelte.ts`: one ticker for every timer, each an `endsAt` timestamp, so a throttled tab or a sleeping laptop can't make it drift; a timer long past its end offers a fresh start instead of a stale alarm. `sound.ts`: chimes made with Web Audio, no files (soft bell, two notes, wood block, rising, a gentle tick for the last seconds), volume, mute and a preview; the first click unlocks audio. Settings persisted per user (`persisted`) | Done: a Wellness rail group with **Focus & breaks** (violet; sub-tabs Focus, Breaks, Today, placeholders until T4 and T5); `clock.svelte.ts` with `Timer` (start, pause, resume, add, reset, snapshot and restore; `left` and `progress` reactive; `onEnd(late)` once, stale past 5 minutes) and `formatClock`; `sound.ts` with five Web Audio chimes and a tick; `wellness.svelte.ts` with sound preferences merged over defaults, `ring()` and `preview()`; a Sound pane (chime, Play, volume, mute) on the Focus tab. The Timer was checked in Node with Svelte's compiler against a fake clock: ends once, pause and resume, +5 minutes, a sleeping laptop (stale, no alarm), a throttled tab (no drift), restore after a reload, formatting; all pass. No CHANGELOG entry until a timer works |
| T4 | Focus timer | A large progress ring in the phase's colour (focus in the tool's hue, breaks in a calm green), the time in large figures, the round as dots (●●○○). Presets: Classic 25/5, Deep work 50/10, Flow 90/20, and Custom: focus, short break and long break lengths, rounds before the long break, auto-start the next phase. Start, pause, resume, +5 minutes, skip, reset; "What are you working on?" shown beside the clock, in the window title and the status bar (kept for the session only). A short end-of-phase card with the chime; Space starts and pauses on the tab. **Today**: focus sessions and minutes, breaks taken, a strip of the day's sessions; reset at midnight | Done: `focus.svelte.ts` (presets, custom lengths within limits, rounds, auto-start, skip counting a cut-short session's minutes but not the session, a stale end offering a fresh start, the run persisted and restored, the task in session storage) and `today.svelte.ts` (today's blocks, dropped when the date changes; reads never write). Focus tab: a phase pill, `FocusRing` (minute ticks, a track tinted by the phase, the arc and a knob, a glow while running), the time in large figures, round dots, the task line, an end-of-phase card (Start break / Skip break / Start focus), the controls around a round main button, a link to Today; a Timer pane (presets, steppers, auto-start) and the Sound pane. Breaks use a new `--rest` token in every theme (contrast-checked). Today tab: three stats, a day timeline in local whole hours (a +5:30 zone was off by half an hour with epoch hours, fixed), the list of sessions, Clear today. Space is a shortcut name now (`shortcuts.svelte.ts`). Checked in the browser: start, the run saved, a reload a few seconds before the end restoring and ending it, the card, Today recording it, custom lengths, Dark and Dusk; docs (shortcuts), CHANGELOG |
| T5 | Break reminders | Reminders that run on their own: **Eye break** (20-20-20: every 20 minutes, 20 seconds looking far away), **Stretch** (every 50 minutes, 3 minutes, a rotating set of simple stretches), **Water** and **Posture** (nudges), and your own (name, interval, length). Each: on/off, interval, length, sound and how it shows: a **full-window break** (dimmed backdrop, countdown ring, a guided breathing circle in a chosen rhythm: calm 4-6, box 4-4-4-4, 4-7-8; for the eye break a slow look-far animation; for stretches a drawn figure per move), a **corner card**, or a **toast**. Snooze 5 minutes, skip, "pause reminders" for 30 minutes, an hour or until tomorrow. Works with the focus timer: no reminder during a focus break, and an option to hold reminders until the focus session ends. Optional active hours (for example 9–18 on weekdays). Coming back after an absence longer than a break counts as one. Still under "reduce motion" | Done: `reminders.svelte.ts` (four built-ins merged over what's stored, custom ones; one scheduler on the Wellness clock, loaded with the shell through `BreakLayer` in `App.svelte`, so reminders run in any tool and the focus run is restored at start; one break at a time; a reminder due over 10 minutes ago or outside active hours starts its interval over; none during a focus break; held until a focus session ends when asked; a break taken or skipped covers shorter ones due; snooze, pause for 30 min / 1 h / until tomorrow, resume restarting intervals; a test from the settings leaves the schedule alone). `BreakLayer`: the full-window break (blurred backdrop, pill, guide, progress, time, Snooze, Skip, Esc) and the corner card; toasts carry Snooze. Guides: `Breathing` (three rhythms, words and seconds), `LookFar`, `StretchGuide` (a drawn figure, five moves paced over the break). Breaks tab: a status card (next reminder and countdown, Take a break now, Pause), a card per reminder (on/off, next-due bar, settings: message, every, lasts, show as, guide, sound, Try it, remove), Add your own, When to remind (wait for focus, active hours, breathing rhythm), Sound. Dropped: "coming back after an absence": a browser only knows codec is in the background, which is most of the working day, so it would reset reminders all the time; the Windows idle time stays under Later. Found and fixed: a timer read just after start could be a moment over its length (the clock's last tick), which broke the stretch guide; the clock now takes the real time when a listener joins and `left` is clamped. Checked in the browser: switching on schedules, a due eye break showing its card on time after a reload and rescheduling 20 minutes on, Take a break now opening the full-window stretch, Esc skipping, a custom reminder's Try it with breathing; the Node timer checks still pass |
| T6 | Everywhere | A status-bar widget (a small ring and the time left; a popover to start, pause, skip and snooze); palette commands (Start focus, Pause, Take a break now, Pause reminders); the countdown in the window title; system notifications when codec isn't in front (the browser's Notification API, asked for from Settings; in the desktop app the Wails notifications service and a tray menu entry: Start focus, Pause reminders); docs: a `wellness.html` page with screenshots, settings, shortcuts; checked by hand in both apps, including a minimised desktop window and a background browser tab | Done: `WellnessWidget` in the status bar (mini ring and time, or the next reminder, or Paused; a popover with focus controls, Break now, Pause 1 h / Resume, Open Focus & breaks), which also owns the global palette commands (the per-tab ones went), the countdown in `document.title`, and the tray's requests. Tray: `Server.Wellness(action)` publishes a `wellness` event (test `TestWellness`), the page's event stream turns it into a `codec:wellness` window event; tray entries Start or pause focus, Take a break now, Pause reminders for an hour, Resume reminders. Notifications: `notify.ts` with the web Notification API, also in WebView2, so no new dependency: the Wails notifications service needs `go-toast`, a module not in `go.sum` (a download and a new dependency), so it was left; off until switched on (Focus → Timer, Breaks → When to remind), only when codec is in the background, silent (the chime plays). Docs: `wellness.html` linked from every page's sidebar, between Everyday tools and Settings, a docs home card, settings, shortcuts. Checked in the browser: from the Network tool a tray-style request started focus, the widget showed the ring and time, the title the countdown, the popover its controls, the palette the Wellness commands. Not checked by hand: the desktop tray entries and a WebView2 notification (the desktop build compiles; launching it here would open a window on the user's desktop); no screenshots on the page yet (Edge headless crashes in this environment) |
| T7 | Find in inputs, Smart paste colours | Asked for 2026-10-31: Smart paste's input coloured as JSON, and find in the input as well as the output (Smart paste, JSON) | Done: `InputPane` gains the search button, Ctrl+F and the find bar, so every tool with an input pane has it; `findShortcut` (`shared/find.svelte.ts`) shared with `OutputPane`, catching Ctrl+F before the editor so CodeMirror's own panel never opens (also on an empty input); `CodeView.find` gains a `'mark'` step and `FindBar` a `stay` option, so typing in the input updates the marks and count without moving the cursor; `language="auto"` (JSON when it parses, else text) for Smart paste. Checked in the browser: Smart paste coloured and searched, typing keeping the cursor; JSON input and output searched side by side; Ctrl+F on an empty input; docs (shortcuts), CHANGELOG |
| — | Release | Full checks; CHANGELOG cut as 3.6.0 | |

Later: the Windows idle time, to count a break taken away from the
keyboard anywhere, not only from codec.

Later, after SignPath signing: "Update now" (download the installer,
verify it against the release checksums, run it silently, restart);
a winget package published by the release workflow.

### v3.7.0 — JSON and table tools
Agreed 2026-10-25 (decision 90). The two heavier tools: working with
structured data pasted from anywhere. The JSON tool grows sub-tabs and
YAML ⇄ JSON lives in it (a paste in one gives the other); CSV and TSV
join Text & data.

| # | Task | Main parts | Status |
|---|---|---|---|
| T1 | JSON tools: engine + API | `internal/jsonkit` over the existing scanner: JSON ⇄ YAML (key order kept), JSONC and JSON5 read, sort keys, flatten to paths and back, a diff by path (added, removed, changed, type changed), infer a JSON Schema from samples, jq filter (the query engine is there), large input read without a tree; `codec json` gains the subcommands; a fuzz target | |
| T2 | JSON tools: UI | The JSON tool grows sub-tabs: Format, Tree (search, copy a value's path, fold), Convert (JSON ⇄ YAML), Diff, Paths, Schema, Query; the tool is named JSON & YAML in the rail and palette; docs | |
| T3 | CSV and TSV | `internal/table`: delimiter and header guessed (override), quotes and embedded newlines; viewer with a windowed table (sort, filter, column types and counts); converts to JSON (typed or all text), Markdown table, YAML and TSV/CSV, and a JSON array of objects back to CSV; `codec csv`; a fuzz target | |
| T4 | Smart paste | Recognises CSV or TSV and JSON5 or JSONC, and offers the tool | |
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
| Mermaid in the Markdown preview | Loaded only when a file has a diagram; source and preview side by side with scroll sync (from 3.3.0) |

## Done

| Version | Items |
|---|---|
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
