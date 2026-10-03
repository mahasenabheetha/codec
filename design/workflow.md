# Workflow

How to change codec so the result fits what is there. Written for any
contributor or AI agent; follow it step by step. Background:
[architecture.md](architecture.md), [conventions.md](conventions.md),
[design-language.md](design-language.md), [decisions.md](decisions.md).

## Ground rules

- codec **never writes to the folders it opens** [3]. Edits are what-if.
- No settings or files in user repositories [4]; settings live in the
  user profile (`internal/config`).
- Engine packages are pure: no files, network, clock or environment.
  Input/output belongs to adapters (`web`, `cli`, `workspace`, `config`,
  `schemacache`, `check`, `sample`).
- No runtime calls to third parties except schema downloads; nothing
  pasted is persisted [19].
- Don't reopen settled decisions; propose a change to the user instead.
- Branch from `main` (`fix/<topic>` or `feature/mab/<topic>`); never
  commit to `main` [72]. Commit messages end with the attribution
  trailer the session specifies.
- Run tests only for packages you touched while working; the full
  suite once before the pull request. The user does UI testing.

## Find where a problem lives

1. **Reproduce with the CLI** if the feature has one (`codec yaml lint`,
   `codec helm values --provenance`, `codec ci job`, `codec argo resolve`,
   `codec k8s refs`, …). Same wrong answer → the bug is in the engine.
   Right answer in the CLI, wrong in the UI → `internal/web` or the frontend.
2. **Use the sample repository** (`internal/sample/files`, or
   `codec serve --sample`) to reproduce; add files there only if the
   sample should demonstrate the case (it is also a test fixture:
   `internal/sample/sample_test.go` expects only `k8s/legacy/` to have
   findings).
3. **Map the symptom to a package:**

| Symptom | Package |
|---|---|
| Parse errors, positions, expressions, paths, fmt/convert, diff, merged-file origins | `internal/yamlkit` |
| Wrong file type, outline, generic hover/definition/completion | `internal/provider` |
| Type-specific hover/definition/diagnostics | the lens package's `provider.go` |
| Lint finding, wording, levels | `internal/lint` (rules in `rules.go`) |
| Schema choice or error mapping, schema completion | `internal/schema` |
| Schema download, cache, offline | `internal/schemacache` |
| Lint + schema combination, Kustomize patches | `internal/check` |
| Helm render, provenance, values checks | `internal/helm` |
| Kubernetes cards, graph, inventory, neat, Kustomize | `internal/kube` |
| Argo resolution, depends, when, Argo CD | `internal/argo` |
| GitHub/GitLab/Azure pipelines | `internal/ci` |
| Compose merge, variables, checks | `internal/compose` |
| Ansible order, roles, precedence, inventories, log tasks | `internal/ansible` |
| Starters, clone, snippets | `internal/scaffold` |
| jq query | `internal/query` |
| Find in Files matches, globs, result cap | `internal/search` |
| Files missing, reading, watching, classification | `internal/workspace` |
| Settings file | `internal/config` |
| API shape, request handling, security | `internal/web` (`<feature>_api.go`, `security.go`) |
| CLI flags or output | `internal/cli` |
| Anything on screen | `frontend/src/features/<view>`; shared parts in `frontend/src/lib` |

## Fixing a bug

1. **Reproduce** and write down the exact input and wrong output.
2. **Write a failing test** in the package that owns the logic: a named
   case in the existing table-driven test (see `lint_test.go`,
   `kube_test.go`, `parse_test.go` for the style). Use the smallest YAML
   that shows it; real-world fixtures go in `testdata/`, trimmed and
   anonymised.
3. **Fix at the source.** Match the surrounding code: comments explain
   why, small functions, existing helpers (`names.DidYouMean`,
   `yamlkit` node helpers) over new ones. No new dependency without a
   reason stated in the pull request.
4. **Frontend bugs:** check the browser console first (Svelte errors
   such as `each_key_duplicate` stop a view from rendering). Fix in the
   feature folder; keep tokens and shared components.
5. **Verify:** `go test ./internal/<pkg>/...`; `npm --prefix frontend
   run check` for UI changes; reproduce again in the UI on the sample.
6. **Record:** a line under `### Fixed` in `CHANGELOG.md` `[Unreleased]`;
   update `docs/` if the documented behaviour changes.
7. **Commit** with a message that names the symptom
   ("Fix the Resources view when an object is defined twice").

## Adding a feature

1. **Plan it in [roadmap.md](roadmap.md)** with the item template
   (problem, result, scope, done when, decisions). Agree scope with the
   user before building anything large.
2. **Decide.** If a new choice is needed (a dependency, a behaviour
   rule, a limit), append a numbered decision to decisions.md.
3. **Engine.** Pure logic in the owning package, or a new
   `internal/<name>` package. Table-driven tests alongside.
   - New file type → a provider (`Detect` + `Symbols`, optional
     capabilities) registered in `provider.Default`.
   - New project-wide view → a lens (see architecture.md "Lenses").
   - New lint rule → `internal/lint/rules.go` (ID, group, title,
     default level, why) and a check with message and fix.
4. **Adapters.** API endpoint in `internal/web/<feature>_api.go`,
   registered in `server.go`, documented in architecture.md "Web API";
   CLI command in `internal/cli` with `--json` and exit codes per
   conventions.md.
5. **UI.** API calls in `frontend/src/lib/api/<feature>.ts`; view in
   `features/<feature>/`; route in `lib/stores/router.svelte.ts`; tab
   title in `lib/shell/TabBar.svelte`; palette entries; empty, loading
   and error states; keyboard access. Follow the review checklist in
   design-language.md.
6. **Performance.** Anything that walks every file or document must stay
   within the budgets (architecture.md "Performance"); run
   `scripts/perf.sh` if in doubt.
7. **Document** (see the matrix below), then move the roadmap row to
   "Done" when it ships.

## What to update

| Change | Update |
|---|---|
| Any user-visible change | `CHANGELOG.md` `[Unreleased]` |
| New or changed feature | Its page in `docs/` (+ screenshot on the sample repo); README "What it does" if new |
| New CLI command or flag | `docs/cli.html`, the feature page's CLI section, README CLI section |
| New shortcut | `lib/shell/ShortcutsDialog.svelte`, `docs/shortcuts.html` |
| New setting | Settings view, `docs/settings.html` |
| New package, API or lens | `design/architecture.md` |
| New UI pattern, token or component | `design/design-language.md`, `docs/design.html` |
| New rule or limit | `design/decisions.md` |
| New work planned or finished | `design/roadmap.md` |

### Editing the docs site

`docs/` is plain HTML with `docs/assets/site.css` — no build step, served
by GitHub Pages from `/docs`. Each page repeats the top bar, the sidebar
and previous/next links: copy an existing page as a template, set
`aria-current="page"` on its own sidebar link, add the new link to the
sidebar of every page, and fix the previous/next links of its
neighbours. Diagrams are inline SVG using the classes in `site.css`
(`box`, `line`, `label`, …). Screenshots are 1400×820 PNGs of the
sample repository in `docs/images/`; never show real repositories,
usernames or paths.

## Definition of done

- [ ] Tests for the changed logic, passing (`go test ./internal/<pkg>/...`).
- [ ] `go vet ./...` clean; `npm --prefix frontend run check` clean for UI changes.
- [ ] Works in the UI on the sample repository (and the CLI where relevant).
- [ ] Read-only and privacy rules still hold.
- [ ] Docs updated per the matrix; CHANGELOG entry.
- [ ] One focused commit (or a few), on a branch, ready for a pull request.

## Releasing

See [releases.md](releases.md): merge to `main`, move `[Unreleased]` to
a version with the date, tag `vX.Y.Z`; the release workflow publishes
binaries, checksums and the Docker image.
