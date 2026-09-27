# design/

Project context for contributors and AI agents: how codec is built and
how to change it. User documentation is in [`docs/`](../docs/index.html).
Start at the repository's [AGENTS.md](../AGENTS.md) and read only what
the task needs.

| File | Read when |
|---|---|
| [workflow.md](workflow.md) | Always before changing code: bug fixes, features, what to update |
| [architecture.md](architecture.md) | Adding packages, APIs, providers or lenses; performance budgets |
| [conventions.md](conventions.md) | Writing Go, frontend code, tests, commits |
| [design-language.md](design-language.md) | Any UI work: tokens, components, patterns, accessibility, writing |
| [decisions.md](decisions.md) | Before proposing a technical choice; append new ones |
| [roadmap.md](roadmap.md) | Planning work; open checks; the backlog |
| [history.md](history.md) | How v2 was built, phase by phase |
| [releases.md](releases.md) | Versioning, cutting a release, the Docker image |

When a fact or decision changes, update the file in the same pull
request. Keep these files short: facts and rules, not essays.
