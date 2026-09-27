# Contributing

Thanks for helping with codec. The short version:

1. Open an issue first for anything bigger than a small fix, so we can
   agree on the approach.
2. Branch from `main`, keep the change focused, and follow
   [design/workflow.md](design/workflow.md) — it explains where things
   live, how to fix a bug (failing test first), how to add a feature,
   and which docs to update.
3. Before opening a pull request:
   ```bash
   npm --prefix frontend ci && npm --prefix frontend run check && npm --prefix frontend run build
   go vet ./... && go test ./...
   ```
4. Describe what changed and why; include a screenshot for UI changes
   (use the sample repository: `codec serve --sample`).

Ground rules that every change keeps: codec never writes to the folders
it opens, stores nothing in users' repositories, and makes no network
calls except downloading public schemas. UI changes follow
[design/design-language.md](design/design-language.md).

By contributing you agree that your contribution is licensed under the
[MIT License](LICENSE).
