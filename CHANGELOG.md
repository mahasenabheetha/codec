# Changelog

All notable changes to codec are documented here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/), and codec uses
[Semantic Versioning](https://semver.org/).

## [Unreleased]

## [3.6.0] - 2026-10-31

Wellness, a new sidebar section: a focus timer with presets or your own
lengths, today's sessions on a timeline, and break reminders for eyes,
stretches, water and posture with guided breathing and stretch
animations, calm chimes and system notifications, from any tool. Also
an opt-in update check, and find in tool inputs.

### Added

- Find in a tool's input as well as its output (the search button, or
  Ctrl+F over or in the input), in Smart paste, JSON, Base64 and every
  tool with an input pane; typing keeps your place while the matches
  follow. Smart paste's input is coloured as JSON when it holds JSON.

- Update check: Settings → General → "Check for updates" (off by
  default) asks GitHub once a day for codec's latest release; a newer
  one shows in the status bar with a link to its page (dismiss it until
  the next release) and in the desktop app's tray menu. "Check now" asks
  once either way, and `codec version --check` does the same on the
  command line. It sends nothing about you or your files.

- Wellness, a new sidebar section, with **Focus & breaks**. The Focus
  tab is a focus timer on a large ring: Classic 25/5, Deep work 50/10,
  Flow 90/20 or your own lengths and rounds, a long break after the
  last round, start, pause, +5 minutes, skip and reset (Space starts
  and pauses), "What are you working on?" beside the clock, and a calm
  chime (five to choose from, with volume and mute) when a phase ends.
  It keeps running in any tab and after a reload; if the laptop slept
  through the end, it offers a fresh start rather than a late alarm.
  The Today tab shows today's sessions, minutes focused and breaks on a
  timeline of the day, kept only until midnight.
- Break reminders, in Focus & breaks → Breaks: an eye break (20-20-20),
  stretches, water, posture and your own, each switched on separately
  with its own interval, length, message and sound, shown as a
  full-window break, a card in the corner or a toast. Breaks can guide
  you: a breathing circle (calm 4-6, box 4-4-4-4 or 4-7-8), rings for
  looking into the distance, a drawn figure doing five stretches, a
  glass filling with water, or a figure sitting up at a desk with three
  posture checks.
  Snooze 5 minutes, skip (Esc), take a break now, or pause reminders for
  30 minutes, an hour or until tomorrow. They wait for a focus session
  to end if you like, never interrupt a focus break, can keep to active
  hours, and a break taken covers shorter ones that came due. They run
  whichever tool is open; one due while the laptop slept is skipped.
  With codec open in several tabs or windows, they keep your choices in
  step and one of them reminds, so each reminder comes once. "Last
  chime", under Sound and in the status-bar popover, says what rang.
- Wellness from anywhere: a status-bar widget (a small ring and the time
  left, or when the next reminder is due, with controls in a popover),
  the time left in the window title, palette commands (start or pause
  focus, skip, add 5 minutes, take a break now, pause and resume
  reminders), tray menu entries in the desktop app, and an optional
  system notification when codec is in the background. A docs page,
  Focus and breaks.

## [3.5.0] - 2026-10-31

Comfort: bigger icons and controls with an Interface size setting, and
two soft, colourful dark themes, Dusk and Midnight, where each tool and
each kind of action has its own colour. Dark and Light look as before.

### Added

- Interface size in Settings → General and the palette: Compact,
  Comfortable or Large. Comfortable is the new default, with bigger
  icons (16 and 18 px instead of 14 and 16) and taller buttons and rail
  entries; Compact keeps the old sizes.

- Dusk, a third theme beside Dark and Light (Settings → General, and
  "Use the Dusk theme" in the palette): soft and colourful, for long
  days. A muted blue-violet ground, off-white text at about 10:1 rather
  than Dark's 16:1, pastel colours of one lightness; each tool's icon,
  header, main button and selected tab (a soft pill) in its own colour,
  a soft glow on the selected sidebar entry, a thin coloured line on
  result cards, and Copy, Clear, Open, Run and other actions in their
  own colours. Dark and Light are unchanged.
- Midnight, a fourth theme: Dusk's colours for icons, buttons, tabs and
  text on a deep navy background ("Use the Midnight theme" in the
  palette). With System, "When the system is dark" picks Dark, Dusk or
  Midnight.

### Fixed

- Settings: a setting's description no longer shrinks to one word a
  line when the page is narrow (beside the explorer); the control moves
  below it instead.

### Changed

- Copy in output panes and Clear in input panes show a word beside the
  icon, so they are easier to find and to hit.

## [3.4.0] - 2026-10-31

Network and text tools under Utilities: CIDR ranges and overlaps, IP
addresses, a URL builder, curl ⇄ fetch and HTTP status codes; a text
diff and a name in every case with the Kubernetes, Helm and env-var
rules; time zones with a working-hours overlap and date formats for
every language; placeholder commit SHAs; and smart paste for all of
them.

### Added

- Secrets & UUID makes placeholder git commit ids: Commit SHA gives
  random lowercase hex in the shape of a commit id, 40 characters, the
  7-character short form, or 64 for SHA-256 repositories; also
  `codec sha` (`--short`, `--sha256`, `-n`).
- Network tool, under Utilities: **CIDR** shows a range in detail (IPv4
  and IPv6: network, broadcast, hosts, mask, wildcard, the kind of
  range), splits it into subnets and checks an address; **Overlaps**
  marks the ranges of a list that share addresses, with their # names;
  **IP** shows an address as integer, hex and binary, IPv6 expanded,
  and its reverse-DNS name; **URL builder** takes a URL apart into
  fields and a query table to edit, and puts it back together; **HTTP**
  turns a curl command (browser "Copy as cURL" included) into fetch and
  a fetch call into curl, saying in a note what can't be carried over,
  and explains status codes with their usual cause and whether to
  retry. On the command line: `codec cidr` (`--split`, `--contains`,
  several ranges for overlaps), `codec ip`, `codec url build` (`--set`,
  `--add`, `--del`), `codec curl` (`--from-fetch`) and
  `codec http status`.
- Text & data tool, under Utilities: **Text diff** compares two pasted
  or dropped texts, side by side with the changed words marked or as a
  unified patch, ignoring whitespace or case if asked; **Case & names**
  writes a name in every case (camel, Pascal, snake, kebab, CONSTANT and
  more, acronyms kept) and checks it against the Kubernetes name and
  label rules, an environment-variable name and a Helm release name,
  offering a name that passes. On the command line: `codec case`
  (`--to`, `--check`).
- Time tool: **Zones** shows a time in several zones at once (DST, a
  day ahead or behind, the next clock change) with a strip of working
  hours that marks when everyone is at work; **Formats** writes one
  moment in RFC 3339, RFC 2822, ISO week and ordinal dates, the HTTP
  date and Unix s/ms/µs/ns, and gives the format string for common
  shapes in Go, Java, Python, .NET and JavaScript. On the command line:
  `codec time --in` and `codec time --formats`.
- Smart paste recognises a CIDR range, an IP address and a curl
  command, shows what it found and opens it in the Network tool (a bare
  number is never taken for an address). `codec auto` now does what
  smart paste does for every utility kind (timestamps, URL encoding,
  cron, logs, ranges, addresses, curl), naming the command that does
  more.
- Compare and Text diff: next and previous change (arrows, or Alt+↓ and
  Alt+↑ in the diff).

### Fixed

- `scripts/dev.sh` and `scripts/desktop.sh` no longer empty
  `frontend/node_modules` when `npm ci` fails (a registry that can't be
  reached): they reinstall only when `package-lock.json`'s content
  changes, not its timestamp, and put the old packages back if the
  install fails.

## [3.3.0] - 2026-10-31

Markdown preview: READMEs and docs in a repository read rendered, as on
GitHub, with links between files and the repository's own images, and
nothing in a file able to run. Also find in tool outputs, and Base64
decode pretty-prints JSON, as it said it did.

### Added

- Find in the output of the encode and decode tools (Smart paste, JSON,
  Base64 and the others): a Find button or Ctrl+F over the output; every
  match is marked, with a count and Enter for the next, Shift+Enter for
  the previous.
- Markdown preview: a Source / Preview switch on `.md` files (also in the
  palette) shows them rendered, GitHub-flavoured with tables, task lists
  and strikethrough, in both themes. It renders the editor's text, so
  what-if edits show. The file is treated as untrusted: raw HTML shows as
  text, nothing in it can run. A link to another file opens it in a
  codec tab (with its #heading when that file shows its preview), a
  folder link shows it in the explorer, web links open in the browser
  (the system browser in the desktop app). Images in the folder show;
  remote images stay blocked until "Load images", since loading them
  tells another server the file was opened. YAML, JSON and shell code
  blocks are coloured as in the editor. Settings → General → Open
  Markdown as picks Source or Preview for new tabs.

### Fixed

- Base64 decode pretty-prints decoded JSON, as its description and smart
  paste always did; a Pretty JSON toggle (on by default) shows the exact
  bytes when off.

## [3.2.0] - 2026-10-24

Argo workflows in the Log viewer: a run read step by step, beside its
status from `argo get` (the whole step tree, steps that never logged,
why a step failed), lanes over time, and a log followed as it grows.
The Argo view colours a workflow's graph by the run.

### Added

- Log viewer: an Argo run is read step by step. Steps come from the
  `argo_step_name` field, the pod's other entries or its name; the list
  shows the step in place of the pod, `step:` filters by it, and a Steps
  tab lists each step with when it ran, how long it took, its entries,
  errors and warnings. Argo's executor lines fold with the output.
- Log viewer: Workflow status… adds a run's status (`argo get -o
  yaml|json` or `kubectl get wf`, opened or pasted) beside its log. The
  Steps tab then shows the whole step tree with each node's phase,
  times and exit code, steps that never logged too, and why a failed
  step failed: Argo's message and its first logged error. Pods are
  matched to the log by name, so a retry's attempts each show their own.
  The tree folds, with the parts that failed or still run open; running
  steps show a striped bar to the end of the log.
- Log viewer: Follow reads a log file from the open folder again as it
  grows (`argo logs --follow > run.log`), keeping the search, view and
  pins and staying at the end; it checks every second, at any size.
- Log viewer: lanes, a second chart over time with a lane per source
  (per step in an Argo run): when each logged and where its warnings
  and errors fall; a cell goes to its entry.
- Argo view: a workflow's graph is coloured by a run whose status a Log
  viewer tab holds: each step's result and duration, Argo's message in
  its details, and Open its log.

## [3.1.1] - 2026-10-24

Fixes from a review and stabilisation pass: static checks, fuzz tests
of every parser and a review of the server and the Log viewer. CI now
also runs the race detector, staticcheck and govulncheck.

### Fixed

- Hovering over some unfinished lists in a CI pipeline file failed.
- Format YAML no longer hands back text that doesn't read as YAML (a
  map used as a key: it now says it can't), fails on a file of only
  comments, or drops an empty document only when run a second time.
- A neat copy of an object whose metadata had only a comment left after
  the server fields came out as invalid YAML; the comment now stays on
  the `metadata: {}` line.
- A problem could be underlined from the wrong place in files with
  stacked tags or templates inside a value over several lines.
- Log viewer: a log codec had let go of (idle for an hour, or codec
  restarted) was sent again once for every page being fetched at that
  moment, holding several copies of a big log; it is now sent once.
- Log viewer: going to an entry inside a fold of one source's output
  could fail when lines from many other sources crossed the fold.
- Log viewer: a click on a mark beside the scrollbar went to wherever
  in its stretch of the list the click fell, often far from the error;
  it now goes to the entry the mark stands for and selects it. The box
  showing where you are now lines up with the marks.
- Log viewer: selecting an entry (which opens it beside the list) or
  closing it threw the list back to the top; it keeps its place, and a
  jump far down the list lands on the entry, not a few rows off.

### Security

- Releases are built with the newest Go patch release. They were built
  with whatever Go the build machine had, which could lag behind its
  security fixes (2.4.1 was built with Go 1.27.1, for one).

## [3.1.0] - 2026-10-23

A Log viewer: structured logs of any common format read into one
shape, to filter, chart and read, several logs at once, and two runs of
a job compared. Built for what `argo logs`, `kubectl logs` and
pipelines print; the log is held in codec's memory only, never written
to disk.

### Added

- Log viewer (Tools → Log viewer), with no settings: ECS and other JSON
  (Go's slog, zap, logrus, zerolog; Bunyan, pino, Serilog; Docker log
  files; Kibana exports), logfmt, klog and text with stack traces,
  behind `argo logs`, `kubectl logs`, container or CI prefixes; times
  of day without a date (logback's default) too. Open, drop or paste
  one or more logs (merged by time; gzip and UTF-16 are fine), or
  right-click a log file in the Explorer; Smart paste opens a pasted
  log there. Several logs open at once, a tab each, every tab keeping
  its own search, range and selection.
- Search: an overview with the first error, repeated errors and
  silences; a query language (`level>=warn service.name:api
  -logger:health status>=500 "connection reset"`) that filters as you
  type, suggests field names and their values, marks the words found
  and explains itself under "?"; a chart to drag a time range on;
  field columns; rows wrap so nothing is cut off, or stay on one line
  and scroll sideways.
- Around a match: entries around each match (±3, ±10) with the skipped
  ones marked; runs of plain output and repeated lines folded into one
  row each (a click or `O` shows them); up to four highlight rules in
  colour; a strip beside the scrollbar showing where errors, warnings,
  highlights and pins are.
- An entry in full: JSON in its message laid out, its stack trace with
  framework frames folded, its fields as a tree, the entries around
  it; any value filters the log, becomes a column or, for trace and
  request IDs, is followed across it. Errors (grouped, `group:N` in the
  query) and Fields (top values, number statistics) tabs.
- Pins (`P`, `[` and `]`) with a note each, copied as a report in text
  or Markdown.
- Compare two runs (Compare with… another tab): the first entry where
  they part, messages new or missing in this run, ones logged much
  more or less often, and each source's time, matched with numbers,
  IDs, times and generated pod names masked; any item opens its entry
  in either tab.
- Export as lines, NDJSON or CSV; whatever is copied has likely secrets
  masked; keyboard (`J`/`K`, `E` for the next error, `/`).

### Changed

- The Workspace problems view and the Resources cards draw only the
  rows in view, so folders with thousands of findings or objects
  scroll smoothly.

## [3.0.0] - 2026-10-22

codec as a desktop app on Windows: its own window, a tray icon, start
at login and "Open with" for YAML files, installed per user without
admin rights. The browser UI (`codec serve`) and the CLI are unchanged.
The desktop files are not yet signed, so Windows warns about an unknown
publisher the first time. Pre-released as 3.0.0-rc.1 on 2026-10-05.

### Added

- Desktop app (`codec-desktop`, Windows): codec in its own window, with
  the same features as the browser UI. One instance: launching it again
  brings the window forward, and a folder or file given to it opens
  there (a file inside the open folder keeps that folder). The window
  comes back where it was, and its title bar follows the theme. Open
  folder uses the system folder dialog, starting in the open folder.
  A tray icon opens the window, a recent folder, or quits; closing the
  window keeps codec in the tray. Settings → Desktop: Start at login
  (in the tray, per user, no admin) and Keep running in the tray.
- Windows installer (`codec-setup.exe`): installs for the current user
  without admin rights, with a Start menu entry, an optional desktop
  shortcut and codec in Explorer's "Open with" for `.yaml` and `.yml`;
  uninstall from Apps. A portable `codec-desktop.exe` as well.

### Changed

- The log drop zone works in the desktop window too.
- Since 3.0.0-rc.1: the desktop app is built with Wails v3.0.0-beta.28;
  Helm SDK 4.3.1, `golang.org/x` and frontend dependency updates.
  Kustomize stays at 0.21.2 until a Kubernetes release supports its
  newer dependencies.

## [2.4.1] - 2026-10-04

### Changed

- Dependency updates: Go modules, `@types/node` and GitHub Actions.
  Dependabot holds TypeScript at 6 until svelte-check supports 7.

## [2.4.0] - 2026-10-04

A light theme, for bright rooms and side-by-side work with a light
editor. Every view gets it, with the same contrast rules as dark.

### Added

- Light theme: Settings → General → Theme offers System, Dark and
  Light; System follows the operating system as it changes. Also in the
  palette ("Toggle light and dark theme", "Use the system theme"). Dark
  stays the default, and the saved theme applies before the first paint.

### Changed

- Project: Dependabot opens grouped update PRs, weekly for Go modules
  and frontend packages, monthly for GitHub Actions.

## [2.3.0] - 2026-10-03

Utilities: a new sidebar heading with Encode & hash, Time and Regex, for
the small jobs that used to send you to a website — with secrets and
passwords never leaving your machine. Cron schedules also explain
themselves on hover in CronJobs and pipelines.

### Added

- Encode & hash: URL encode and decode, with a URL's parts and query
  as a table; hex; SHA-256, SHA-384, SHA-512, SHA-1 and MD5 digests and
  HMACs, with a check against an expected digest; random secrets by
  length and character set, and UUIDs; bcrypt htpasswd lines for basic
  auth, and a check of a password against one.
- Time: Timestamp reads epoch numbers (seconds to nanoseconds, told by
  size) and dates in any zone, with relative time and Now; Cron explains
  an expression in plain words, one cell per field, with its next runs
  across daylight-saving changes, builds one from a form or a preset
  (any months), notes when both day fields are set (either one runs),
  and names Quartz and Jenkins expressions as unsupported.
- Regex: matches highlighted as you type, capture groups per match, a
  replace preview, and an explanation of the pattern part by part;
  hovering a part finds it in the pattern and a group lights up what it
  captured. Go (RE2) or Python / .NET / JS style, the latter stopped
  after 2 seconds when a pattern backtracks badly.
- Hover a cron schedule in a Kubernetes CronJob, an Argo CronWorkflow,
  a GitHub workflow or an Azure pipeline to read it in plain words, with
  the zone it runs in and the next runs. The sample has one of each.
- Smart paste reads epoch timestamps, URL-encoded text and cron
  expressions, and opens them in Timestamp, URL or Cron with one click.
- Tools can have sub-tabs: Home and the command palette list each one
  by name ("Open Cron"), and a tool reopens on the one you used last.
- CLI: `codec url`, `hex`, `hash`, `secret`, `uuid`, `htpasswd`, `time`,
  `cron` and `regex`. Passwords and HMAC keys are never taken as
  arguments, so they stay out of shell history.

### Changed

- The docs page "Encode, decode and logs" is now "Everyday tools".

### Fixed

- An internal error while computing an editor request (hover,
  definitions, analysis) or reading an Ansible log is reported as an
  error instead of stopping `codec serve`.

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

[Unreleased]: https://github.com/mahasenabheetha/codec/compare/v3.6.0...HEAD
[3.6.0]: https://github.com/mahasenabheetha/codec/compare/v3.5.0...v3.6.0
[3.5.0]: https://github.com/mahasenabheetha/codec/compare/v3.4.0...v3.5.0
[3.4.0]: https://github.com/mahasenabheetha/codec/compare/v3.3.0...v3.4.0
[3.3.0]: https://github.com/mahasenabheetha/codec/compare/v3.2.0...v3.3.0
[3.2.0]: https://github.com/mahasenabheetha/codec/compare/v3.1.1...v3.2.0
[3.1.1]: https://github.com/mahasenabheetha/codec/compare/v3.1.0...v3.1.1
[3.1.0]: https://github.com/mahasenabheetha/codec/compare/v3.0.0...v3.1.0
[3.0.0]: https://github.com/mahasenabheetha/codec/compare/v2.4.1...v3.0.0
[2.4.1]: https://github.com/mahasenabheetha/codec/compare/v2.4.0...v2.4.1
[2.4.0]: https://github.com/mahasenabheetha/codec/compare/v2.3.0...v2.4.0
[2.3.0]: https://github.com/mahasenabheetha/codec/compare/v2.2.0...v2.3.0
[2.2.0]: https://github.com/mahasenabheetha/codec/compare/v2.1.0...v2.2.0
[2.1.0]: https://github.com/mahasenabheetha/codec/compare/v2.0.1...v2.1.0
[2.0.1]: https://github.com/mahasenabheetha/codec/compare/v2.0.0...v2.0.1
[2.0.0]: https://github.com/mahasenabheetha/codec/compare/v2.0.0-alpha.1...v2.0.0
[2.0.0-alpha.1]: https://github.com/mahasenabheetha/codec/compare/v1.0.0...v2.0.0-alpha.1
[1.0.0]: https://github.com/mahasenabheetha/codec/releases/tag/v1.0.0
