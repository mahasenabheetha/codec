package logsrc

// Excludes are the Find in Files globs a search from a log leaves out:
// logs themselves (the line would find its own copy) and the folders
// builds and package managers fill, which repeat the source or aren't
// it. Only what the folder lists is searched in any case.
var Excludes = []string{
	"!*.log", "!*.log.[0-9]*", "!*.min.js", "!*.min.css", "!*.map",
	"!node_modules", "!vendor", "!dist", "!build", "!target", "!out", "!bin", "!obj",
	"!coverage", "!.git", "!.next", "!.nuxt", "!.svelte-kit", "!.terraform", "!.gradle",
	"!.venv", "!venv", "!__pycache__", "!.tox", "!.pytest_cache",
}

// MaxFragment bounds a searched selection: a search is for a phrase
// from a line, not a paragraph.
const MaxFragment = 300
