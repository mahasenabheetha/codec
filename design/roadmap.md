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

### Routine for every version

1. Start from an up-to-date `main`: `git switch -c feature/mab/v2.X.0`.
2. Do the tasks in order. Each is finished before the next: code, tests
   for the packages it touches only, its docs page, its CHANGELOG line
   under "Unreleased". Commits: `2.X.0 T<n>: <what>`.
3. After each task, stop with a short summary and how to try it
   (`scripts/dev.sh run`); continue when the user says so.
4. Finish: `go vet ./...`, `go test ./...`, frontend build and
   `svelte-check`; docs pass over the changed pages; CHANGELOG
   "Unreleased" → `## [2.X.0] - <date>`; roadmap rows → "Done".
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

### v2.1.0 — Compare, reload prompt, Find in Files
Branch `feature/mab/v2.1.0`.

| # | Task | Main parts | Status |
|---|---|---|---|
| T1 | Compare: clear | × per side, Clear next to Swap; ignore patterns kept | Done |
| T2 | Compare paste: API | `paste` side kind; no folder needed; structure only for mappings/lists; whitespace and case options in `textdiff`; tests | Done |
| T3 | Compare paste: UI | Third source with an editor box; "Compare as"; "Live object noise" preset; "Compare text" on Home and in the palette; "Cleared · Undo"; docs | Done |
| T4 | Explorer: context menu + Ctrl+click | Shared context-menu component; multi-select; keyboard | Done |
| T5 | Explorer → Compare | Drag and drop onto a side; "Select for compare", "Compare with selected", "Compare selected"; docs | Done |
| T6 | Reload prompt | Server marks a stale token; "codec restarted — Reload" banner; docs | Done |
| T7 | Find in Files: engine + API + CLI | `internal/search`, endpoint, result cap; tests; `codec search` | Done |
| T8 | Find in Files: UI | Search panel in the Explorer's slot, Ctrl+Shift+F, results open at the line; docs | Done |
| T9 | Compare: side-by-side text diff | Changed characters marked, folded context, Unified toggle; each picker heads its column (feedback on T3) | Done |
| — | Release | Full checks passed; CHANGELOG cut as 2.1.0 | Released as v2.1.0 |

### v2.2.0 — Ansible log analyzer, playbook map
Branch `feature/mab/v2.2.0`. Large logs are dropped or picked as a file and sent as is, up to 64 MB (decision 76).

| # | Task | Main parts | Status |
|---|---|---|---|
| T1 | Analyzer stage 1: clean | ANSI, `\r`, CI timestamps/markers, learned wrapper prefixes; tests | Done |
| T2 | Stage 2: segment | Anchors, several runs, "other output", stray lines to their task; tests | Done |
| T3 | Stage 3: read results | Brace matching → YAML → raw; loops, retries, ignored, rescued, unreachable, `no_log`; `json` callback | Done |
| T4 | Stage 4: model + API + CLI | Run → plays → tasks → host results, recap, durations; `codec ansible log`; synthetic fixtures; fuzz test | Done |
| T5 | Analyzer UI | Summary bar, outline, filters, host picker, recap table, other-output blocks, file drop, first failure selected; existing task view as detail; docs | Done |
| T6 | Playbook map: graph | From the execution-order analysis; static/dynamic includes, "assumed", external roles; tests | Done |
| T7 | Playbook map: UI | Pipeline graph component, click to open; docs | Done |
| T8 | Run colours on the map | Failed, changed, ok, never ran | Done |
| — | Release | Full checks passed; CHANGELOG cut as 2.2.0 (the map shipped with the analyzer) | Released as v2.2.0 |

### v2.3.0 — Utilities
Branch `feature/mab/v2.3.0`. Decisions 78–80; the design notes are in this file's history.

| # | Task | Main parts | Status |
|---|---|---|---|
| T1 | Utilities setup | Sub-tabs in `lib/tools.ts`, "Utilities" rail heading, palette and Home entries per sub-tab; placeholders until each tool lands | Done |
| T2 | [Encode & hash](#utilities-encode--hash): engine + API + CLI | URL, hex, hash/HMAC, secrets & UUID, htpasswd (bcrypt); test vectors; `internal/encode`, `POST /api/v2/encode/{kind}`, decision 78 | Done |
| T3 | Encode & hash: UI | Five sub-tabs; docs | Done |
| T4 | [Time](#utilities-time): engine + API + CLI | Timestamps; cron parse, next runs, describe, build; dialect messages; DST tests; `internal/cron`, `internal/timeutil`, `POST /api/v2/time/{kind}`, decision 79 | Done |
| T5 | Time: UI | Timestamp with Now; cron explainer + generator; docs | Done |
| T6 | Cron hover | CronJob, GitHub and Azure schedules | Done (Argo CronWorkflow too; sample has one of each) |
| T7 | [Regex](#utilities-regex): engine + API + CLI | RE2 and regexp2, time limit, replace, explainer; tests; `internal/regex`, `POST /api/v2/regex`, decision 80 | Done |
| T8 | Regex: UI | Tester and explainer on one screen; docs | Done |
| T9 | Smart paste | Epoch numbers, URL-encoded text, cron expressions | Done (`codec.DetectUtility`; the clipboard watcher and `codec auto` are unchanged) |
| — | Release | Three parallel reviews (engines, API and CLI, frontend), fixes with regression tests and a fuzz test; full checks passed; CHANGELOG cut as 2.3.0 | Released as v2.3.0 |

### v2.4.0 — Light theme
Agreed 2026-10-03 (decision 81). Every colour is already a token in
`tokens.css` (no hard-coded colours in components), so this is a second
token set, a setting and a contrast pass.

| # | Task | Main parts | Status |
|---|---|---|---|
| T1 | Light palette | Light values for surfaces, borders, text, accent, status, syntax, diff, expression tints, graph groups and logos; every text pair ≥ 4.5:1; `design-language.md` gains the light column | Done |
| T2 | Theme switching | `[data-theme="light"]` in `tokens.css`, `color-scheme` per theme; a script in `index.html` sets the theme before first paint (no flash) | Done |
| T3 | Setting | System / Dark / Light in Settings and the palette, persisted as a display preference; System follows the OS live | Done |
| T4 | Pass over every view | Editor, compare, graphs, lens views, tools, dialogs and toasts in both themes; `CloneView` fallback colours to tokens; docs (`docs/design.html`, settings) | Done |
| — | Release | Full checks passed; CHANGELOG cut as 2.4.0 | Ready to merge and tag |

### v3.0.0 — Desktop app
Agreed 2026-10-03 (decision 82). Wails v3 (system tray needs it), one
repo, Windows first; macOS when a Mac is available. The desktop app is a
second entry point to the same server and UI, never a second copy of a
feature: features go in `internal/` and `frontend/` as before, and
desktop-only extras sit behind `frontend/src/lib/platform.ts` with a
browser fallback, so the web UI (`codec serve`) keeps working unchanged.

```
cmd/codec/            CLI + codec serve (unchanged)
cmd/codec-desktop/    window, tray, menus; mounts the same server
frontend/             same app; lib/platform.ts for desktop extras
desktop/              Wails config, icons, installer
```

| # | Task | Main parts | Status |
|---|---|---|---|
| T1 | Spike | `cmd/codec-desktop` loads the current app: the server's handler mounted in Wails (no port, no token) or, if SSE streaming fails there, a loopback server with the token; check live file events, clipboard, persisted settings, drag and drop, file drop for logs; confirm Wails v3 status | Done: Wails v3.0.0-beta.27; loopback server on 8769 (decision 83); live events, clipboard (secure context), persisted settings work; log file drop fixed in code, to try by hand |
| T2 | Window | Single instance (second launch focuses the window and passes its file), window size and position remembered, native menus mapped to palette commands, title bar follows the theme | Done, except the menu bar: none on Windows for now (the palette is the menu; Wails themes a menu bar only at start, so it would stay dark after switching to light); native menus come with macOS. WebView2 data in `%LOCALAPPDATA%\codec\webview` |
| T3 | Native dialogs | Open folder / file through `lib/platform.ts`; the web keeps its folder browser | Done: `workspace.chooseFolder()` everywhere Open folder is offered; `POST /api/v2/desktop/pick-folder`. File pickers (log file) already get the system dialog in WebView2; picking a file inside the workspace stays codec's list |
| T4 | Tray and auto-start | Tray menu (open, recent folders, quit); closing the window keeps codec in the tray; "Start at login" setting (per-user Run key) | Done: Settings → Desktop (Start at login runs `codec-desktop --hidden`; Keep running in the tray, `desktop.quitOnClose` in settings.json); `/api/v2/desktop/settings` |
| T5 | Installer | Per-user NSIS (`RequestExecutionLevel user`) into `%LOCALAPPDATA%\Programs\codec`: Start menu entry, uninstall entry in HKCU, "Open with codec" for `.yaml`/`.yml` in HKCU; no admin; a portable exe as well | Done: `scripts/desktop.sh`, `desktop/windows/` (installer.nsi, manifest, icon.ico); exe icon and version via a generated `.syso` (wails3 generate syso). Optional desktop shortcut; existing shortcuts are never overwritten, and uninstall removes only what it created. Uninstall keeps `%APPDATA%\codec` (shared with the CLI) |
| T6 | Release build and signing | Desktop build in the release workflow next to the CLI; version and product metadata on the exe; signing step through [SignPath](#code-signing) (off until approved) | Done without signing: `desktop` job in release.yml (windows-latest, after GoReleaser) uploads the portable exe, installer and checksums. The desktop package builds on Windows only (`//go:build windows`), so Linux vet/test/build skip it. Signing waits for the SignPath application (Open checks) |
| T7 | Docs | Install, desktop features, uninstall; README and docs site; `architecture.md` | Done: README Quick start, getting-started (Desktop app), settings, security, troubleshooting, architecture page; `design/architecture.md` (layout, runtime mode), design-language (`lib/platform.ts` pattern), releases.md |
| — | Release | Full checks; CHANGELOG cut as 3.0.0 | Review (16 findings fixed); pre-released as v3.0.0-rc.1 (release workflow and installer confirmed); rebuilt with Wails beta.28, full checks passed; CHANGELOG cut as 3.0.0. Released as v3.0.0 |

Later, with a Mac: build on a macOS runner (cgo, Xcode), Apple Developer
ID signing and notarization, macOS menu bar, Dock and window-close
behaviour.

#### Code signing
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
| Product name and version metadata on signed binaries | Set by the Windows build (T6) |
| Uninstaller; no undisclosed data collection or system changes | Installer has one; nothing collected |

The certificate names SignPath Foundation as the publisher, not the
author. SignPath signs Windows only; macOS needs Apple's program.

### v3.1.0 — Log viewer
Agreed 2026-10-22 (decision 86, replacing the Argo-only plan of
2026-10-10). Structured logs are slow to read in a terminal: JSON lines
are long, sources interleave, and the line that matters sits among
thousands. A Log viewer (Tools → Logs, beside the Ansible log; no open
folder needed; browser UI and desktop app) reads every common format
into one shape (time, level, message, source, logger, caller, error,
trace ID, other fields), and every view works on that shape. Argo is
one source among many; its step tree comes in 3.2.0.

What it reads, with no settings:

| Kind | Examples |
|---|---|
| ECS JSON | any ecs-logging library (Go `ecszap`, Java, .NET, Python, Node): `@timestamp`, `log.level`, `message`, `error.*`, `trace.id`; dotted or nested fields |
| Other JSON | Go `slog`, zap, logrus, zerolog; Bunyan and pino (numeric levels); `severity` (Google Cloud); Kibana and Elasticsearch exports (`_source`, `hits.hits`) |
| logfmt | `slog`'s text handler, logrus text, Argo's own containers |
| Text | Kubernetes `klog`; Go panics and goroutine dumps, Java and Python stack traces joined to their entry; any other line kept as a text entry |
| Prefixes, stripped and kept as the line's source | `argo logs` and `stern` pod prefixes, `kubectl logs --timestamps`, container-runtime lines (`… stdout F …`), `docker compose logs`, CI timestamps, colour codes |

Input: files opened or dropped (several at once, merged by time;
gzip read as is), pasted text, or a log file in the open folder
(Explorer → Open in Log viewer). The parsed log stays in the server's
memory, never on disk, and the browser asks only for what it shows
(pages, counts, a chart), so 100k lines stay smooth and 1M lines work.

| # | Task | Main parts | Status |
|---|---|---|---|
| T1 | Reader | `internal/logview`, pure: decode (UTF-8 and UTF-16 BOMs, CRLF, gzip), strip colour codes and prefixes (kept as source and prefix time), detect the format per line; map each JSON flavour, logfmt and klog to the common entry (level names and numbers; RFC 3339, epoch s/ms/ns, `2006-01-02 15:04:05,000`, klog without a year); stack traces and panics joined to their entry; JSON inside `message` read as fields; synthetic fixtures per format; fuzz test; benchmark `encoding/json` against `valyala/fastjson` | Done: own JSON scanner (order kept, ~11× `encoding/json`, fuzzed against it), so no new dependency; also Docker json-file, Serilog CLEF (templates rendered), Kibana/Elasticsearch documents, CRI split lines; 100k ECS lines read in ~0.25 s |
| T2 | Store and query | In-memory log: core fields per entry, other fields as dictionary-encoded columns; query `level>=warn service:api -logger:health status>=500 "connection reset" /time.?out/` (AND by default, `OR`, parentheses, `*` wildcards); time range; entries over time per level; the N entries around one, unfiltered; error groups (message with numbers, IDs, UUIDs, hex and quoted values masked); field stats (present in, distinct, top values; min, max, median, p95 for numbers); summary (levels, span, sources, first error, silent gaps); budgets in `scripts/perf.sh` (100k lines: load < 1 s, query < 100 ms) | Done: compact rows with keys and sources stored once (columns not needed: a full scan of 100k entries takes 1–35 ms); also `field:(a OR b)`, durations, array items, files merged by time, histogram axis kept while filtering; 100k ECS lines: build 0.38 s, field stats 94 ms; held memory ≈ 2.4× the input (`Log.Size`). Budgets are benchmarks here; `perf.sh` gets them with the API |
| T3 | API | `POST /api/v2/logs` (files, pasted text, or a workspace file through the root guard) → handle and summary; `/{id}/entries` (query, page), `/{id}/entries/{n}` (detail and context), `/{id}/fields`, `/{id}/groups`, `/{id}/export` (filtered entries as NDJSON, CSV or text); `DELETE`; limits on memory held (`Log.Size`, about 2.4× the input): 512 MB per log, 1 GB in all with the oldest dropped first, dropped after an idle hour; budgets in `scripts/perf.sh`; "expired" answered so the UI re-sends what it holds; tests | Done: limits are 256 MB of text per log (≈ 600 MB held) and 1 GB held in all; the last query is kept per log so paging doesn't rerun it; `at` finds the page holding an entry; a gzip or UTF-16 file in the open folder is read through the root guard (`workspace.ReadLog`); NDJSON export keeps field order. `perf.sh` on 100k lines: open 0.55 s, queries 3–27 ms, field stats 120 ms |
| T4 | Windowed list | Shared list component that keeps only visible rows in the DOM, with variable heights (wrapped lines, expanded context); also used by the Problems view and Resources cards (the backlog item "Window long lists") | Done: `lib/components/VirtualList.svelte`; Problems flattened into file and finding rows, Resources cards windowed a row at a time with as many cards as fit; 5,000 findings: 14–22 rows in the DOM, 0–6 ms of script per scroll step |
| T5 | Viewer UI | Tools → Logs → Log viewer: inputs (open, drop, paste; files as chips; Explorer "Open in Log viewer" for `.log`, `.ndjson`, `.jsonl`); overview (levels, span, sources, first error, top error groups); entry list (level colours, any field as a column, absolute or relative time, wrap, line numbers); query bar with chips that write the query; chart of entries over time, drag to set a range; docs (`docs/logs.html`) | Done: `features/logs` (viewer state, overview, chart, list on `VirtualList` with pages fetched as rows come into view, a basic entry panel T6 extends); a log the server dropped is sent again on its own (410); paste anywhere on the tool; display choices (time, wrap, columns) remembered; 100k pasted lines read in 0.64 s, any scroll position filled in ~70 ms. No screenshot yet in `docs/logs.html` |
| T6 | Investigate | Detail panel (fields as a tree, raw line, stack trace laid out, JSON in the message pretty-printed); per value: only this, hide this, add as column, follow (`trace.id`, request IDs); ±N entries around one in a filtered list; Errors tab (groups with count, first and last time); Fields tab (stats); export and copy (filtered entries, a summary; password, token and secret values and bearer tokens masked on copy); keyboard (next error, next match, `/` for the query); smart paste sends logs here; docs | Done: detail panel (`FieldTree`, `StackTrace`, `ValueActions`, ±5/20/50 around); Errors and Fields tabs; `group:N` in the query (`logview` groups now have ids); export as lines, NDJSON or CSV; `mask.ts` on every copy (checked with 10 cases in Node, as the frontend has no Vitest); keyboard; smart paste detects logs (`codec.DetectUtility`, not Ansible runs); `/` allowed in query field names (Kubernetes labels) |
| — | Feedback | After trying it | Done: search as you type with suggestions (field names, then values) and "?" examples, words marked in rows; rows wrap (or one line, scrolling sideways); a time of day alone, `_timestamp` and other time names read; a line that is only a time starts an entry and the plain lines after it from the same source are its message (Oracle's alert log inside an Argo run) |
| T7 | Several logs | Tabs inside the viewer, one log each with its own query, range, selection, pins and highlights; Open logs and Explorer → Open in Log viewer open a tab, Add logs merges into the current one; each tab's held size shown; recent queries back in the suggestions | Done: `LogTabs` and one `LogViewer` per tab, reached through Svelte context; every tab stays mounted (hidden when not shown), so its scroll, entry panel and Errors/Fields tab survive switching; pasted logs numbered across tabs, the sample named Sample; a tab that fails to read shows why and closes |
| T8 | View layer: engine + API | Over the matches, on the server so 100k entries stay smooth: context (N entries around each match, the rest dimmed, gaps as "⋯ 240 entries"); repeats folded (consecutive, same source; exact by default, or the same apart from numbers and IDs) as "×120, first–last"; runs of plain output between structured entries folded per source ("⋯ 12 lines of output from …"), expandable, nothing dropped; up to four highlight rules (queries) marked per row; a compact picture of where errors, highlights and pins are, for the scrollbar | Done: `logview.View` (context with gaps, output folded per source and broken by any structured entry or gap, runs of 3 or more; repeats exact or similar), `RowAt`, `Filter` for highlight rules, `Markers` (error, warning, pin and rule bits per bucket); the entries endpoint pages over the view (cached per log like the matches) and keeps counts, chart and groups on the matches; 100k entries: context 10 with output folding 5 ms, everything folded 22 ms; fuzzed |
| T9 | View layer: UI | Context 0 / 3 / 10, Repeats and Output folding switches, highlight rules with colours (edge per row), markers beside the scrollbar to jump to; counts, chart and Errors keep counting matches only | Done: Around Off/±3/±10, Fold: Output (on by itself for mixed logs) and Repeats (off, exact, similar), Highlight (four rules, errors shown per rule), the marker strip with where you are; the list pages over view rows; O opens or folds; selection follows its entry when a fold opens above it; context and repeat style remembered as preferences |
| T10 | Pins | Pin with P, a row button or the entry panel; a pins panel in time order with a note each; [ and ] between pins; copy as a report (text or Markdown, secrets masked); pins on the scrollbar; memory only | Done: a Pins tab (time order, a note each, go to, unpin, unpin all), P on the selected row, the pin on a row and in the entry, [ and ] from pin to pin (the query cleared if it hides one), pins on the marker strip; copy as text or Markdown through `maskSecrets`; cleared with a notice when the tab's files change, since pins point at entries by their place |
| T11 | Compare two runs | Compare with… another open tab: the first difference (the first entry whose message the other run never has), new messages and errors, missing ones (a step that didn't happen), changed counts and slower sources; messages matched with numbers, IDs and times ignored; each item opens its entry in either tab; engine, API, UI | Done: Compare with… another open tab, a Compare tab with both runs in numbers, the first difference in each, New here / Missing here / More or less often (half as often again or a third less, and by 2 or more) / Sources (entries, errors, time; slower marked); `logview.Compare` over fingerprints (any masked value matches any other, generated pod names masked), `POST /logs/{id}/compare`; a click opens the entry in either tab; copy as text, masked; stale once a tab's files change. Also fixed: a tab opened from the Explorer lost its folder file when files were dropped into it (folder files now go as `path` parts beside dropped ones) |
| — | Release | Full checks; CHANGELOG cut as 3.1.0 | Full checks passed (vet, all Go tests, frontend build, svelte-check; the 100k-entry log benchmarks within budget); docs pass (Log viewer page against the code, README, docs home, security); CHANGELOG cut as 3.1.0 |

Later: `codec logs` on the command line on the same engine (filters,
`-o json`, stdin, an exit code for gates); saved queries.

### v3.1.1 — Review and stabilisation
A whole-codebase pass after 3.1.0, so it covers the viewer too. Bug
fixes only, as a patch release.

| # | Task | Main parts | Status |
|---|---|---|---|
| T1 | Static checks | `staticcheck`, `govulncheck`, `go test -race`, `npm audit`; the useful ones join CI | Done: staticcheck 2026.2.2 found 10 (dead code in ansible, ci, web and yamlkit; a loop in compose; four API test lines that never checked the status), all fixed; govulncheck 1.9.0 found GO-2026-6809 (`crypto/x509`, fixed in Go 1.27.3): releases took whatever Go the runner had (2.4.1's binary says 1.27.1), so CI and release now ask for `stable` with `check-latest` (the `golang.org/x/crypto` openpgp advisory is not reached); `npm audit` found nothing; CI on Linux now runs the race detector, staticcheck and govulncheck (pinned); npm stays with Dependabot. No C compiler here, so the race detector first runs in CI |
| T2 | Fuzz tests | Every parser (YAML tools, logs, Ansible logs, encoders, cron, regex), for crashes and hangs | Done: 12 new fuzz targets beside the 6 there were: FuzzWorkbench (the editor's detection, analysis, lint, hover, definition and completion over mutated sample files), the YAML parser and formatter, paths, cron, smart paste, base64, hex, URLs, diffs, jq queries, Argo expressions, Neat; each run 1–5 min. Found and fixed: a nil dereference in CI hover; parser ranges ending before they start (stacked tags, templates in multi-line scalars; now clamped in `Parse`); Format returning text it can't read back (map keys), failing on comment-only text and dropping empty documents only on a second pass; Neat writing `metadata: # c` then `{}`, and odd anchors it can't write (now refused). Failing inputs kept in `testdata/fuzz` |
| T3 | Browser tests | Playwright against the sample workspace: open a folder, the main views, Compare, tools, settings; in CI | Skipped (2026-10-24): the user chose not to add Playwright; UI checks stay manual (the browser pane) |
| T4 | Review by area | Separate reviewers (a workflow, under 10 at a time): Go engines; server, API and security; frontend; desktop app and installer; release workflow; docs against behaviour; each finding verified before it is reported | Done as one lean pass instead of a workflow (the user's choice, 2026-10-24), over the riskiest areas. Server and security: the Host, token and Origin checks and the folder guard (`os.Root`) hold; v1 `/api/transform` needs no token but reads no files. Log viewer API: request values bounded, caches locked. Found: the browser re-sent an expired log once per page being fetched (several copies held); `RowAt` looked back only 64 rows for a fold. Desktop, release workflow and docs were not reviewed |
| T5 | Fixes | Each with a regression test | Done with T4: one shared re-send of an expired log (checked in a test build: two pages found it gone, one re-send), and `RowAt` looks back all the way (a test with a fold crossed by 80 sources fails on the old code). Then, from the user's report: the scrollbar marks went to where a click fell in a bucket of up to hundreds of rows (now `Markers` returns the row each mark stands for: the first error, else pin, highlight, warning); selecting or closing an entry redrew the list from the top (the list moves into or out of the split view; the row is brought back); `scrollToIndex` corrects by the row's real position; the where-box is placed by rows like the marks. Checked in a test build: four marks from either state each selected their error, on screen |
| — | Release | Full checks; CHANGELOG cut as 3.1.1 | Full checks passed (vet, all Go tests, staticcheck, govulncheck with the local Go 1.27.2 aside, frontend build, svelte-check); docs pass (the developer docs' two new rows); CHANGELOG cut as 3.1.1 |

### v3.2.0 — Argo workflows and live follow
Agreed 2026-10-22 (decision 87). Built on the Log viewer: an Argo run
shown as its steps, what each did, how long it took, what failed and
why, including steps that failed before they logged.

| # | Task | Main parts | Status |
|---|---|---|---|
| T1 | Steps from pods | Argo pod names become steps (or the `argo_step_name` and `argo_workflow_name` fields when the log carries them); `init` and `wait` container lines folded; per step: first and last time, errors, warnings | Done: `logview/argo.go` works out each entry's step after reading (its `argo_step_name`, else its pod's, else the pod name: `<wf>-<template>-<number>` with the generated suffix, or a workflow named in `argo_workflow_name`; older `<wf>-<number>` names are their own step; sourceless lines inherit), `Summary.Steps` and `Workflows`, `step:` in the query; executor lines (init/wait containers, `argo=true`) count as output; a Steps tab (start order, time bar, took, entries, errors, warnings, Show), the step in the list's source column and in the entry; checked in a test build with a made-up four-step run |
| T2 | Workflow status | `argo get <wf> -o yaml` or `-o json` (or `kubectl get wf`), whole or a snippet, opened or pasted beside the logs: the step tree (steps, DAG tasks, parallel groups, retries, loop items) with phase, times, exit code and message; failed reason = Argo's message + the step's first error | Done: `argo.ReadStatus` (YAML or JSON; a Workflow, a List or a bare status; nodes linked into a tree from the workflow's node, each once; pod names as Argo makes them, v2 `<wf>-<template>-<fnv32a(node name)>` or v1 node IDs), `POST /argo/status`; per-pod tallies in each log step (`PodStats`, with first error); Workflow status… (open or paste, memory only) and a chip with the run's phase; the Steps tab as the tree (parallel groups hidden), phase, when, took, exit code, the pod's own counts (by pod name; else the step by name or template, a retry's attempts leaving it to the retry node), failed reason = Argo's message + first error; steps only in the log listed after. Checked in a test build with a made-up run and a log whose pods carry the computed names |
| T3 | Steps and lanes | Step tree with failed steps expanded; one lane per source (pod, step, service) over time; a log that stops mid-run shows its last steps as still running | Done: the status tree folds (open where something failed or runs, a chevron flips it, a folded node says how many it holds); Running and Pending steps show a striped bar to the log's end; `Log.Lanes` (a lane per step, else per source, on the histogram's buckets, a red cell's entry its error) and `POST /logs/{id}/lanes`; Chart: Levels / Steps (or Sources) as a preference; a cell goes to its entry, a name filters. Without a status, a log that stops mid-run can't tell a running step from a finished one; live follow (T4) is where that shows |
| T4 | Live follow | A log file in the open folder followed as it grows (codec's file watching), e.g. `argo logs --follow > run.log` | Done: Follow for tabs of folder files; `GET /files/stat` polled once a second (the watcher's index skips files over 8 MB, which a followed log soon is, and polling also works on mounts); `refresh()` reads the files again keeping query, view, pins and selection, stays at the end if the list was there; the last step to log shows as running without a status. Checked in a test build: lines appended from outside arrived in about two seconds |
| T5 | Argo graph coloured by a run | When the open folder has the workflow, its graph in the Argo view is coloured by the run's step results, with durations; a node opens its step's log; docs (`argo.html`) | Done: the Argo view takes runs from Log viewer tabs with a status (best fit by step names, or picked); `runOf` rolls a step's attempts and items up; Graph's run states (ok, failed, changed for running, never); phase and time in the node, phase, exit code and message in its details; Open its log filters the tab to the step (by pod, else name) and switches to it; `logs/steps.ts` shared with the Steps tab |
| — | Release | Full checks; CHANGELOG cut as 3.2.0 | Full checks passed (vet, all Go tests, staticcheck, govulncheck apart from the local Go, frontend build, svelte-check; a fuzz target for the status parser); docs pass (logs, Argo, README, docs home); CHANGELOG cut as 3.2.0 |

### v3.3.0 — Markdown preview
Agreed 2026-10-05 (decision 84). READMEs and docs in a repository are
easier to read rendered than as raw syntax. `.md` files already open in
the editor (lang `markdown`); a Source / Preview switch in the file
toolbar shows them rendered, in the browser UI and the desktop app.

| # | Task | Main parts | Status |
|---|---|---|---|
| T1 | Safe rendering | `marked` (GitHub-flavoured: tables, task lists, strikethrough) and `DOMPurify`; raw HTML off, scripts and event handlers stripped; renders the editor's text, so what-if edits show; tests with hostile Markdown | Done: `features/markdown/render.ts`, `renderMarkdown(src)`; marked 18.1.1 and DOMPurify 3.4.17, pinned; raw HTML shown as text; DOMPurify strips scripts, handlers, unsafe links, frames, forms and styles; task boxes disabled, links get `rel=noopener noreferrer`, image addresses moved to `data-src` so nothing loads (for T3). 28 cases in `render.cases.ts` with `check()`, which reads the parsed elements; all pass in the browser (dev server console); a renderer that sanitises nothing fails 19. No frontend test runner yet: running them in CI needs a DOM library such as jsdom |
| T2 | Preview in the file view | Source / Preview switch in `FileView.svelte` toolbar and the palette; styles for headings, lists, tables, quotes and code in both themes; heading anchors | Done: `MarkdownPreview.svelte`; a Source / Preview `SegmentedControl` in the file toolbar for lang `markdown`, Show Markdown preview / source in the palette; the editor stays mounted under the preview (cursor, scroll and what-if edits kept), and a jump to a position switches back to Source. Heading ids as GitHub makes them (`user-content-` prefix via DOMPurify, so no id can shadow a page global), a `#` anchor on hover. The app routes by URL hash, so the preview takes every link click: `#heading` scrolls, http(s) and mailto open a new window, file links wait for T3. 31 cases pass; checked by hand in a test build (dark and light, what-if, palette, hostile HTML inert) |
| T3 | Links and images | A link to another file opens it in a codec tab, `#heading` scrolls, external links open in the system browser; images inside the workspace through a read-only route (images only, inside the root); remote images blocked with "Load images" | Done: `links.ts` resolves links and images against the file's folder (a leading / against the open folder, nothing above it, other schemes ignored); a file link opens its tab and hands over a #heading for when that file's preview shows, a folder link reveals it, a missing one says so. `GET /api/v2/files/image` (images only, bytes checked, 10 MB, sandbox CSP), fetched with the token and shown as blob URLs, SVG cleaned with DOMPurify first. Remote images counted and held back behind a "Load images" bar, allowed per file until codec closes (never stored). Web links through `platform.openLink`: a new tab in the browser, `POST /api/v2/desktop/open-url` (http, https, mailto only) to the system browser in the desktop app. Go tests for both routes; 17 link cases with `check()`; checked by hand in a test build. The desktop link opening is not tried yet |
| T4 | Code blocks and setting | YAML, JSON and shell blocks highlighted with the editor's highlighter; Settings → Editor: open Markdown as Preview or Source; docs (`workspace.html`, settings) | Done: the editor's tag colours moved to `lib/components/syntax.ts`, one list for CodeView's `HighlightStyle` and `highlightTo` (lezer `highlightCode` with `syn-*` classes, text nodes and spans only); yaml/yml, json/jsonc and sh/shell/bash/zsh/console blocks, others stay plain. "Open Markdown as" in Settings → General (where the editor settings are), `markdownView` persisted like `lensOpen`. Docs: workspace.html Markdown preview, settings.html, CHANGELOG |
| — | Release | Full checks; CHANGELOG cut as 3.3.0 | Full checks passed (gofmt, vet, all Go tests, staticcheck, frontend build, svelte-check, the 31 Markdown and 17 link cases); govulncheck and the race detector left to CI (no vulnerability database offline, no C compiler here); docs pass (workspace, settings, tools, README, docs home); decision 92; CHANGELOG cut as 3.3.0. Also in this release: find in tool outputs, Base64 decode pretty-printing JSON |

Later: Mermaid diagrams (loaded only when a file has one); side by side
with scroll sync.

### v3.4.0 — Network and text tools
Agreed 2026-10-25 (decision 90): the first of three releases of small
local tools in the style of 2.3.0, split so each is a whole. This one is
the daily-use half and mostly small: addresses, HTTP, text and time.
Each engine is a Go package with an API route, a CLI command where the
CLI makes sense and a fuzz target for what it parses; nothing a tool is
given is stored (19). New Utilities sub-tabs: **Network** (CIDR, IP,
URL, HTTP) and **Text & data** (text diff, case and names; the JSON and
CSV tools join it in 3.7.0, decision 94).

| # | Task | Main parts | Status |
|---|---|---|---|
| T1 | Commit SHA placeholders | Asked for 2026-10-31: Secrets & UUID gains a Commit SHA kind, random lowercase hex shaped like a git commit id (40, the 7-character short form, or 64 for SHA-256 repositories) for placeholder ids; `encode.CommitSHA`, `POST /api/v2/encode/sha`, `codec sha` | Done: a third Kind in Secrets & UUID with Full 40 / Short 7 / SHA-256 64, up to 100 at a time, Copy each or all; crypto/rand hex; `codec sha` (`--short`, `--sha256`, `-n`); tests for the engine and the route; docs (tools, CLI), palette keywords commit, sha, git |
| T2 | Utilities frame | Sub-tab groups Network and Text & data in `lib/tools.ts`, rail entries, palette and Home entries; placeholders until each tool lands | Done: two Utilities tools in `lib/tools.ts` after Encode & hash: **Network** (CIDR, IP, URL builder, HTTP; "URL builder" so it isn't confused with Encode & hash → URL) and **Text & data** (Text diff, Case & names), each a `TabbedTool`; rail, Home cards and palette entries come from the registry; `shared/PlannedTab.svelte` stands in for each sub-tab until its task lands. No CHANGELOG entry until a tool works |
| T3 | Network: engine + API + CLI | `internal/netutil`: CIDR (network, broadcast, first and last host, mask, wildcard, host count, contains, split into subnets, overlap between a list of ranges; IPv4 and IPv6); IP converter (integer, hex, binary, IPv6 expand and shorten, v4-mapped); URL parse and build (parts, query as editable pairs, rebuilt URL; builds on `encode.ParseURL`); `POST /api/v2/net/{kind}`; `codec cidr`, `codec ip`, `codec url` gains `build`; test vectors, a fuzz target | Done: `internal/netutil` on `net/netip` (`Details`, `Split` listing up to 1024, `Contains`, `Overlaps` up to 1000 ranges with "same" or "contains", `ParseIP`; range kinds from the IANA special-purpose blocks, counts as decimal strings for IPv6); `encode.BuildURL` beside `ParseURL`; `POST /api/v2/net/{cidr,overlaps,ip,url}`; `codec cidr` (one range, `--split`, `--contains`, several = overlaps, exit 2), `codec ip`, `codec url build` (`--set`, `--add`, `--del`). Fuzz targets for details, IP, overlaps and the URL query round trip; the IP one found 32-digit hex of `::1` read back as IPv4, fixed (the digit count now picks the family) |
| T4 | Network: UI | CIDR, IP and URL sub-tabs; a list of ranges with overlaps marked; the query-parameter table; docs | Done: one-column tabs. **CIDR**: Range (details with the kind as a badge, counts grouped, Copy per row; Subnets with a "split into /n" box starting two bits down, listing up to 1024; an address check) and Overlaps (a list with # names kept, each range marked same as / contains / inside by number, bad lines struck through). **IP**: every notation with Copy. **URL builder**: a pasted URL fills scheme, user, host, port, path, fragment and an editable query table; any edit rebuilds the URL (never written back to the input). Engine gained `Listed.Note` (the line's comment), also in `codec cidr`. Docs: tools.html Network section, CHANGELOG |
| T5 | HTTP: engine + API + CLI + UI | curl ⇄ fetch only: method, URL, headers, `-d`/`--data*` (JSON, form), `-u`, cookies, `--compressed` and `-L` noted; shell quoting, `$'…'` and line continuations read; what can't be converted (`-F` files, `@file`, proxies) said in words; status lookup (code, name, meaning, usual cause, whether to retry; searchable); `internal/httpkit`, `codec curl`, `codec http status`; a fuzz target for the curl reader | Done: `internal/httpkit`: a shell word splitter (quotes, `$'…'`, `\`, `^` and backtick continuations, stops at a pipe), the curl options that matter (`-X -H -d --data-* --data-urlencode --json -u --oauth2-bearer -b -A -e -F --form-string -G -I -m`), the rest noted (`-k`, proxies, `@file`, `-T`, certificates) or skipped (output flags); fetch out with `JSON.stringify` for JSON bodies, FormData for `-F` (a file as `fileInput.files[0], "name"`), `AbortSignal.timeout`; fetch back in with a small JS-literal reader (strings, templates kept as written, objects, `JSON.stringify`, `new Headers`, FormData appends); 61 status codes with cause and retry advice, searched by number, prefix or words. `POST /api/v2/http/{curl,fetch,status}`; `codec curl` (`--from-fetch`), `codec http status`. Fuzz: curl → curl and curl → fetch → back must match; it found seven real edge cases (invalid UTF-8, CR line endings, trailing spaces in header values, `-m 1E20` overflowing, control characters needing `$'…'`), all fixed and kept as seeds. UI: curl → fetch, fetch → curl (notes under the result, swap), Status codes; the API sends empty lists, not null |
| T6 | Text diff | UI over `internal/textdiff`: two boxes or dropped files, side by side and unified, word-level marks inside changed lines, ignore whitespace and case, next and previous change | Done: no new engine or route: `POST /api/v2/compare` already takes two pasted sides in text mode, and `compare/TextDiff.svelte` draws them. TextDiff now takes its options (layout, ignore flags) as a prop, the Compare view's by default, and gained next and previous change by block (arrows with a count, Alt+↓/↑, the current block marked), so Compare has them too; its bar wraps between items. `text/DiffTab.svelte`: Original and Changed panes (type, paste, open or drop a file read in the page, 5 MB, binary refused), ignore whitespace and case, swap, the diff below |
| T7 | Case and names | Camel, Pascal, snake, kebab, CONSTANT, dot, Title (acronyms kept) with the Kubernetes name (DNS-1123 label, 63 characters), environment-variable name and Helm release name rules; engine, API, UI; `codec case` | Done: in `internal/names` (case.go): `Words` (separators and case changes, acronyms whole, digits with their letters), `Cases` (ten forms; acronyms kept unless the input is all upper case), `Checks` (DNS-1123 label 63 and subdomain 253, label value, env var, Helm release 53), each with the problems in words and a fixed name; a fuzz target checks every suggestion passes its own rule. `POST /api/v2/text/case`; `codec case` (`--to` per line, `--check` exit 2). UI: one column, the words under the input, Cases with Copy beside Name rules with "Use …" and Copy. `ValueRows` moved to shared, now one aligned label column (subgrid); PlannedTab removed, every sub-tab is built |
| T8 | Time zones and timestamp formats | In the Time tool: a zone converter (a time in any zone shown in several at once, DST aware, an overlap strip for working hours) and a formats sheet (one instant as RFC 3339, RFC 2822, ISO week and ordinal date, Unix s, ms, µs and ns, and Go, Java, Python, .NET and JavaScript format strings, each with Copy); builds on `internal/timeutil` | Done: `timeutil.Instant` (also a bare 14:00 as today), `Zones` (time, date, abbreviation or offset, DST, day shift, the next change found to the minute, and the local minutes at each hour of the first zone's day), `Formats` (13 standard forms), `Patterns` (8 shapes with Go, Java, Python, .NET and JS, traps noted; a test checks each Go layout writes its example). `POST /api/v2/time/{zones,formats}`; `codec time --in`, `--formats`. UI: Zones (the base zone shared as a preference with a remembered list, add by name, a 24-hour strip marking 9–17 and the hours all share, the overlap named in the first zone's time) and Formats (rows with Copy, a card per shape, click to copy). Hours are whole hours of the base zone, so a half-hour edge shows from the next hour |
| T9 | Smart paste | Recognises a CIDR, an IP and a `curl` command and offers the tool; no bare-number guesses; `codec.DetectUtility` and `codec auto` follow | Done: `networkUtility` in `codec.DetectUtility`: a CIDR only as address/length, an IP only written as one (dotted IPv4, or with colons), a curl command only when it starts with `curl` and has one URL with a real host (`curl is a tool` is left alone); summaries from netutil and httpkit, notes as comments. Smart paste labels and "Open in CIDR / IP / HTTP"; the three Network tabs take the hand-over. `codec auto` gained the Utilities detection it never had (all kinds, before JSON, as the web route does), naming the command that does more |
| — | Release | Full checks; a fuzz run of the new parsers; docs (`tools.html`); CHANGELOG cut as 3.4.0 | Full checks passed (gofmt, vet, all Go tests, staticcheck, frontend build, svelte-check); govulncheck and the race detector left to CI (no vulnerability database offline, no C compiler here); a fuzz run of the seven new targets (netutil details, IP, overlaps; httpkit curl, fetch; names checks; encode URL build) came back clean; docs pass (tools, CLI, compare, README, docs home); decision 93; CHANGELOG cut as 3.4.0 |

### v3.5.0 — Comfort: bigger controls and the Dusk theme
Agreed 2026-10-31 (decision 94), asked for by the user: some icons are
too small (Copy, Clear), and they want a colourful dark theme that is
easy on the eyes, not high contrast, next to today's Dark and Light,
which stay exactly as they are. Every colour and size is already a
token (81), so this is new tokens, a setting for size, a third token
set and a pass over the views.

**Dusk**, the new theme: a muted blue-violet background instead of
near-black, soft off-white text rather than bright white (body text
around 9–11:1 instead of Dark's 16:1; muted text still at least
4.5:1), and pastel colours, light and low in saturation, so they read
as colour without glare. No pure white, no neon. Colour carries
meaning in three separate roles, and the status hues are never borrowed:
a **tool hue** (each tool has one; its rail icon, tabs and main button
use it), an **action tone** (Copy, Clear, Download, Swap, Open, Add,
Run each have one) and the **status colours** (error, warning, OK,
info), so a pink tab never reads as an error.

| # | Task | Main parts | Status |
|---|---|---|---|
| T1 | Colour roles as tokens | A `hue` per tool in `lib/tools.ts`, set as `--tool` on its rail entry, header and tabs; action tones (`--act-copy`, `--act-clear`, `--act-save`, `--act-swap`, `--act-open`, `--act-add`, `--act-run`) and a `tone` prop on `IconButton` and `Button`, given to the existing actions; in Dark and Light every new token resolves to today's colour, so nothing looks different yet | Done: seven pastel hues (`--hue-*`); a hue on every tool and on the rail's workspace views (`viewHue`); a `.hued` class works out `--tool-accent`, `-hover`, `-soft` and `--tool-icon` from `--tool` with `color-mix`, so the values are known where the tool sets its hue; tool pages, rail entries and Home cards use it, as do primary buttons and tab underlines inside a tool. Seven action tones (`--act-*`, drawn from the same hues; statuses keep theirs) and a `tone` prop on `IconButton` and `Button`, given to 62 existing buttons (copy, clear, save, swap, open, add, run; a close × and zoom have none). `--tint-tool` and `--tint-action` are 0% in Dark and Light: the colours computed in the browser match the old ones, and set to 100% they show the hues. Colour roles in `design-language.md`. No CHANGELOG entry: nothing visible yet |
| T2 | Interface size | Icon and control sizes as tokens (`--icon-sm`, `--icon-md`, `--control-h`, `--control-h-sm`) switched by a `data-size` attribute: Compact (today's 14/16 px icons), Comfortable (16/18 px, 28/32 px controls, the default) and Large; the icon sizes written into components (about 120) move to the tokens; Settings → General "Interface size", beside Theme, set before first paint like the theme; Copy, Clear and Download in output panes become icon-and-word buttons; docs (settings) | |
| T3 | Dusk palette and setting | `[data-theme='dusk']` in `tokens.css` (`color-scheme: dark`): surfaces, borders, text, accent, status, syntax, diff, expression tints, graph groups, logos, tool hues and action tones; contrast checked by a script against the rules above; Theme becomes System / Dark / Dusk / Light, and a "Dark style" choice (Dark or Dusk) for what System uses when the OS is dark; `index.html` applies it before first paint, the desktop title bar follows as dark; `design-language.md` gains the Dusk column | |
| T4 | Colour in Dusk | What only Dusk turns on: the active tab a soft pill in the tool hue and the others a small hue dot, rail icons in their hue always (a soft glow when selected), action icons in their tone, primary buttons tinted with the tool hue, a thin hue edge on result cards; a pass over every view in all three themes and both sizes (editor, compare, graphs, lens views, tools, logs, dialogs, toasts); docs (`design.html`, `settings.html`, a Dusk screenshot) | |
| — | Release | Full checks; CHANGELOG cut as 3.5.0 | |

### v3.6.0 — Update check and focus timer
Agreed 2026-10-06 (decision 85). Wails has no updater and codec never
looks for new versions, so an installed copy stays old until the user
notices a release. codec can ask GitHub for the latest release and say
when a newer one exists; updating stays the installer run over the top
(it replaces the install in place and keeps settings).

The release also holds a small focus timer (decision 91, 2026-10-31):
a Pomodoro clock and an eye-break reminder, so long days at the screen
get rests. Both add a Settings section, and both are small.

| # | Task | Main parts | Status |
|---|---|---|---|
| T1 | Check: engine + API + CLI | `internal/update`: latest release from the GitHub API (pre-releases only when running one), semver compare with `internal/version`, 5 s timeout, at most once a day (result cached in the config dir), dev builds never check; `updates.check` setting, off by default; `GET /api/v2/update`; `codec version --check`; tests against a fake server | |
| T2 | Notice in the UI | Settings → General: "Check for updates" switch and "Check now"; "codec X.Y.Z is available" in the footer with a link to the release (the system browser in the desktop app), dismissable per version; tray menu entry in the desktop app; docs (settings, security: the second network use, troubleshooting) | |
| T3 | Focus timer and break reminders | Frontend only, no new dependency, off until switched on in Settings → Focus & breaks. `features/focus/focus.svelte.ts`: one store, phase (focus, break, off) and an `endsAt` timestamp, the time left worked out from the clock so a throttled window or a sleeping laptop can't make it drift, a long-expired timer offering a new session instead of a stale alert. **Pomodoro** (focus 25, break 5, a long break of 15 every fourth round; lengths are settings; auto-start or wait) and an **eye break** (every 20 minutes, a 20-second look-away; the lengths are settings); any break resets the eye timer, so the two never fire together. A status-bar widget (icon and countdown, a popover with start, pause, skip, reset and snooze 5 minutes), palette commands (Start focus, Take a break now, Snooze). A break card in the corner with a CSS breathing circle (in 4 s, out 4 s) and the countdown, still under "reduce motion"; a soft two-note chime made with Web Audio (no files; volume and mute; the Start click unlocks audio); the countdown in the window title while it runs. Same code in the browser UI and the desktop app. Only the chosen lengths are remembered (`persisted`), never the running timer or any history; docs (`settings.html`, `shortcuts.html`, a short page on the timer); to check by hand in both apps, including a minimised desktop window and a background browser tab (late by up to a minute) | |
| — | Release | Full checks; CHANGELOG cut as 3.6.0 | |

Later for the timer: a system notification and a tray entry in the
desktop app (needs a look at what Wails offers), the Windows idle time
for a nudge across all apps; for now a system-wide tool such as
Stretchly or Workrave covers other applications.

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

## Done

| Version | Items |
|---|---|
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
