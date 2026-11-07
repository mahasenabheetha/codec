package commands

import (
	"cmp"
	"slices"
	"strings"
	"sync"
	"unicode"
)

// Search by purpose (decision 104): "undo my last commit" finds
// git reset --soft HEAD~1 though the command has none of its words.
// Recipes, commands and flags are the documents. A query's words are
// cut to their roots and widened with a table of synonyms, and each
// document is scored by where its words matched: a title counts more
// than a summary, the word itself more than a synonym.

// Result is one thing the search found, with how the query matched it.
type Result struct {
	Kind    string  `json:"kind"` // recipe, command or flag
	Tool    string  `json:"tool"`
	Title   string  `json:"title"`
	Command string  `json:"command"` // the line to open in the builder
	Summary string  `json:"summary,omitempty"`
	Danger  *Danger `json:"danger,omitempty"`
	// Why says what matched: "commit", "undo ≈ reset", "pusj → push".
	Why   []string `json:"why,omitempty"`
	Score float64  `json:"score"`
}

// The weight of a match in each field of a document, and of each kind:
// a recipe answers a question; a flag is only part of an answer.
var (
	fieldWeight = [...]float64{3, 2.5, 2, 1} // title, other words, command, summary
	kindWeight  = map[string]float64{"recipe": 1.2, "command": 1, "flag": 0.8}
)

const (
	synonymWeight = 0.6
	prefixWeight  = 0.5 // "deploy" for "deployment"
	typoWeight    = 0.5
	toolBoost     = 1.5 // the query names the tool
	flagBonus     = 4.0 // the query has the flag itself: --force
	keepShare     = 0.25
	echoWeight    = 0.1 // a word matched in another field too
)

type doc struct {
	Result
	fields [len(fieldWeight)][]string // word roots
	flags  []string                   // a flag document's names
}

type index struct {
	docs  []doc
	vocab map[string]string // every root in a document, and a word it came from
	// tools are the words that name tools: their names and aliases, and
	// their groups ("kubernetes", "containers"), to the tools.
	tools map[string][]string
}

var loadIndex = sync.OnceValues(func() (*index, error) {
	tools, err := load()
	if err != nil {
		return nil, err
	}
	return buildIndex(tools), nil
})

func buildIndex(tools map[string]*Tool) *index {
	ix := &index{vocab: map[string]string{}, tools: map[string][]string{}}
	var list []*Tool
	for _, t := range tools {
		if !slices.Contains(list, t) {
			list = append(list, t)
		}
	}
	slices.SortFunc(list, func(a, b *Tool) int { return strings.Compare(a.Name, b.Name) })
	for _, t := range list {
		for _, n := range append([]string{t.Name, t.Group}, t.Aliases...) {
			if !slices.Contains(ix.tools[n], t.Name) {
				ix.tools[n] = append(ix.tools[n], t.Name)
			}
		}
		if t.Group == "kubernetes" {
			ix.tools["k8s"] = append(ix.tools["k8s"], t.Name)
		}
		for _, r := range t.Recipes {
			ix.add(doc{Result: Result{Kind: "recipe", Tool: t.Name, Title: r.Title, Command: r.Command, Summary: r.Note}},
				r.Title, strings.Join(r.Words, " "), purposeWords(r.Command), r.Note)
		}
		ix.addCommand(t.Name, &t.Command, []string{t.Name}, true)
	}
	return ix
}

// addCommand adds c and its flags, then its subcommands. The tool's own
// command isn't a result: the tool cards are.
func (ix *index) addCommand(tool string, c *Command, path []string, root bool) {
	line := strings.Join(path, " ")
	if !root {
		ix.add(doc{Result: Result{Kind: "command", Tool: tool, Title: line, Command: line + " ", Summary: c.Summary, Danger: c.Danger}},
			strings.Join(path[1:], " "), strings.Join(c.Aliases, " "), "", c.Summary)
	}
	for _, f := range c.Flags {
		name := longest(f.Names)
		if name == "-<n>" {
			continue
		}
		ix.add(doc{Result: Result{Kind: "flag", Tool: tool, Title: line + " " + name, Command: line + " " + name, Summary: f.Summary, Danger: f.Danger}, flags: f.Names},
			name, strings.Join(f.Names, " ")+" "+f.Value, strings.Join(path[1:], " "), f.Summary)
	}
	for _, s := range c.Commands {
		ix.addCommand(tool, s, append(slices.Clip(path), s.Name), false)
	}
}

func (ix *index) add(d doc, fields ...string) {
	for i, f := range fields {
		for _, w := range wordsOf(f) {
			r := stem(w)
			d.fields[i] = append(d.fields[i], r)
			if _, ok := ix.vocab[r]; !ok {
				ix.vocab[r] = w
			}
		}
	}
	ix.docs = append(ix.docs, d)
}

// purposeWords are the words of a recipe's command that say what it
// does: its subcommands and values, and its placeholders' names. Flags
// are documents of their own, and "--dry-run" in a command is no reason
// for it to answer "what is running".
func purposeWords(command string) string {
	var out []string
	for _, f := range strings.Fields(command) {
		if !strings.HasPrefix(f, "-") {
			out = append(out, f)
		}
	}
	return strings.Join(out, " ")
}

func longest(names []string) string {
	out := names[0]
	for _, n := range names[1:] {
		if len(n) > len(out) {
			out = n
		}
	}
	return out
}

// Search finds what does what the query asks, best first: at most
// limit results, and only those near the best. tool, when set, keeps
// to one tool. An empty query lists the recipes, for browsing.
func Search(query, tool string, limit int) ([]Result, error) {
	ix, err := loadIndex()
	if err != nil {
		return nil, err
	}
	return ix.search(query, tool, limit), nil
}

// term is a word of the query and what it may match.
type term struct {
	word string
	alts []alt
}

type alt struct {
	root   string
	weight float64
	why    string
}

func (ix *index) search(query, tool string, limit int) []Result {
	var flags []string
	for _, f := range strings.Fields(query) {
		if strings.HasPrefix(f, "-") && len(f) > 1 {
			flags = append(flags, f)
		}
	}
	var terms []term
	hinted := map[string]bool{} // tools the query names
	seen := map[string]bool{}
	onlyTools := true // "git", "docker compose": browse their recipes
	for _, w := range wordsOf(query) {
		// A tool's name is a hint, and a word too: "free disk space"
		// asks about space, not only about the free command.
		if ts, ok := ix.tools[w]; ok {
			for _, t := range ts {
				hinted[t] = true
			}
		} else if !stopword[w] {
			onlyTools = false
		}
		r := stem(w)
		if stopword[w] || seen[r] {
			continue
		}
		seen[r] = true
		terms = append(terms, ix.expand(w, r))
	}

	if len(flags) == 0 && (len(terms) == 0 || onlyTools && len(hinted) > 0) {
		var out []Result
		for _, d := range ix.docs {
			if d.Kind == "recipe" && (tool == "" || d.Tool == tool) && (len(hinted) == 0 || hinted[d.Tool]) {
				out = append(out, d.Result)
			}
		}
		return out
	}

	var out []Result
	for i := range ix.docs {
		d := &ix.docs[i]
		if tool != "" && d.Tool != tool {
			continue
		}
		score, matched, why := 0.0, 0, []string(nil)
		for _, t := range terms {
			// The best field counts; the others it matches in add a little,
			// so a recipe whose words say it again ranks above one whose
			// title only happens to have it.
			best, bestWhy, others := 0.0, "", 0.0
			for f, roots := range d.fields {
				fieldBest, fieldWhy := 0.0, ""
				for _, a := range t.alts {
					if w := fieldWeight[f] * a.weight; w > fieldBest && slices.Contains(roots, a.root) {
						fieldBest, fieldWhy = w, a.why
					}
				}
				if fieldBest > best {
					others += best
					best, bestWhy = fieldBest, fieldWhy
				} else {
					others += fieldBest
				}
			}
			best += echoWeight * others
			if best == 0 && len(t.word) >= 3 {
				root := stem(t.word)
				for f, roots := range d.fields {
					if w := fieldWeight[f] * prefixWeight; w > best && slices.ContainsFunc(roots, func(r string) bool { return strings.HasPrefix(r, root) }) {
						best, bestWhy = w, t.word+"…"
					}
				}
			}
			if best > 0 {
				score += best
				matched++
				why = append(why, bestWhy)
			}
		}
		for _, f := range flags {
			if slices.Contains(d.flags, f) {
				score += flagBonus
				matched++
				why = append(why, f)
			}
		}
		if score == 0 {
			continue
		}
		all := len(terms) + len(flags)
		score *= kindWeight[d.Kind] * (0.4 + 0.6*float64(matched)/float64(all))
		if hinted[d.Tool] {
			score *= toolBoost
		}
		r := d.Result
		r.Score, r.Why = score, why
		out = append(out, r)
	}
	slices.SortStableFunc(out, func(a, b Result) int {
		return cmp.Or(cmp.Compare(b.Score, a.Score), cmp.Compare(kindWeight[b.Kind], kindWeight[a.Kind]))
	})
	if len(out) > 0 {
		cut := out[0].Score * keepShare
		n := 0
		for n < len(out) && out[n].Score >= cut {
			n++
		}
		out = out[:n]
	}
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out
}

// expand is what a query word may match: its root, the roots of its
// synonyms, and, for a word no document has, the ones a typo away.
func (ix *index) expand(w, r string) term {
	t := term{word: w, alts: []alt{{r, 1, w}}}
	have := map[string]bool{r: true}
	for _, g := range synonymsOf[r] {
		for _, s := range synonymGroups[g] {
			if sr := stem(s); !have[sr] {
				have[sr] = true
				t.alts = append(t.alts, alt{sr, synonymWeight, w + " ≈ " + s})
			}
		}
	}
	if _, known := ix.vocab[r]; !known && len(t.alts) == 1 && len(r) >= 4 {
		var near []string
		for v := range ix.vocab {
			if len(v) >= 4 && oneEdit(r, v) {
				near = append(near, v)
			}
		}
		slices.Sort(near)
		for _, v := range near {
			t.alts = append(t.alts, alt{v, typoWeight, w + " → " + ix.vocab[v]})
		}
	}
	return t
}

// wordsOf splits text into lower-case words of letters and digits:
// "--force-with-lease" is force, with and lease.
func wordsOf(s string) []string {
	return strings.FieldsFunc(strings.ToLower(s), func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) })
}

// stem cuts a word to a root, so that change, changes, changed and
// changing meet. It is a rough cut, not English grammar: both sides of
// a match go through it, so they only need to agree.
func stem(w string) string {
	if len(w) <= 3 {
		return w
	}
	switch {
	case strings.HasSuffix(w, "sses"):
		w = w[:len(w)-2]
	case strings.HasSuffix(w, "ies") && len(w) > 4:
		w = w[:len(w)-3] + "y"
	case strings.HasSuffix(w, "ss"):
	case strings.HasSuffix(w, "ing") && len(w) >= 6:
		w = w[:len(w)-3]
	case strings.HasSuffix(w, "ed") && len(w) >= 5:
		w = w[:len(w)-2]
	case strings.HasSuffix(w, "es") && len(w) >= 5:
		w = w[:len(w)-2]
	case strings.HasSuffix(w, "s"):
		w = w[:len(w)-1]
	}
	// stopped, running: one of a doubled last letter goes.
	if n := len(w); n >= 4 && w[n-1] == w[n-2] && !strings.ContainsRune("lsz", rune(w[n-1])) && !strings.ContainsRune("aeiou", rune(w[n-1])) {
		w = w[:n-1]
	}
	if n := len(w); n >= 4 && w[n-1] == 'e' {
		w = w[:n-1]
	}
	return w
}

// oneEdit says whether a and b are one letter apart: one changed,
// added, dropped, or two side by side swapped.
func oneEdit(a, b string) bool {
	if a == b {
		return false
	}
	if len(a) == len(b) {
		diff := []int{}
		for i := range len(a) {
			if a[i] != b[i] {
				diff = append(diff, i)
				if len(diff) > 2 {
					return false
				}
			}
		}
		if len(diff) == 1 {
			return true
		}
		i, j := diff[0], diff[1]
		return j == i+1 && a[i] == b[j] && a[j] == b[i]
	}
	if len(a) > len(b) {
		a, b = b, a
	}
	if len(b)-len(a) != 1 {
		return false
	}
	i := 0
	for i < len(a) && a[i] == b[i] {
		i++
	}
	return a[i:] == b[i+1:]
}

var stopword = set("a an the to of in on for with from by at as is are be it its i me my we our you your this that these those and or " +
	"how do does did done was were can could should would will what which when where why want need way some any just please command commands")

func set(s string) map[string]bool {
	m := map[string]bool{}
	for _, w := range strings.Fields(s) {
		m[w] = true
	}
	return m
}

// synonymGroups are words people use for the same thing. A word may be
// in more than one: "last" is the previous one, and the end of a file.
var synonymGroups = [][]string{
	{"undo", "revert", "reset", "restore", "rollback", "roll", "back", "discard", "uncommit", "unstage"},
	{"delete", "remove", "rm", "erase", "drop", "purge", "destroy", "clean", "prune", "wipe"},
	{"list", "show", "view", "display", "see", "get", "ls", "print", "describe", "inspect", "check"},
	{"log", "logs", "output", "print"},
	{"rename", "move", "mv"},
	{"copy", "cp", "duplicate", "clone"},
	{"find", "search", "locate", "look", "grep", "where", "filter", "match"},
	{"change", "modify", "edit", "update", "set", "replace", "patch", "alter"},
	{"create", "make", "new", "add", "start", "init", "begin"},
	{"stop", "kill", "terminate", "halt", "end", "quit", "cancel", "abort"},
	{"restart", "reboot", "bounce", "reload", "rollout", "redeploy"},
	{"save", "stash", "shelve", "aside", "keep", "store"},
	{"combine", "merge", "join", "squash", "rebase", "integrate"},
	{"history", "log", "commits", "past", "earlier"},
	{"size", "space", "disk", "big", "large", "usage"},
	{"process", "processes", "proc", "ps", "running", "program"},
	{"network", "port", "listen", "connection", "socket", "forward"},
	{"permission", "permissions", "chmod", "access", "owner", "chown", "mode"},
	{"allowed", "allow", "permission", "forbidden", "authorized", "rbac", "can-i"},
	{"compress", "zip", "archive", "tar", "pack", "unzip", "extract", "unpack"},
	{"download", "fetch", "pull", "get", "retrieve", "sync"},
	{"upload", "send", "push", "publish", "share"},
	{"previous", "last", "latest", "recent", "prior", "before", "head"},
	{"first", "top", "head", "beginning", "start"},
	{"last", "tail", "end", "bottom"},
	{"follow", "watch", "stream", "live", "tail", "monitor"},
	{"error", "errors", "fail", "failure", "crash", "broken", "problem"},
	{"count", "number", "many", "total"},
	{"difference", "diff", "compare", "changed", "changes", "between"},
	{"who", "blame", "author", "wrote"},
	{"tag", "release", "version", "label"},
	{"branch", "branches", "switch", "checkout"},
	{"commit", "commits", "snapshot", "save"},
	{"force", "overwrite", "override"},
	{"secret", "password", "credential", "token"},
	{"login", "signin", "sign", "authenticate", "credentials"},
	{"server", "machine", "host", "remote", "box", "vm"},
	{"config", "configuration", "settings", "setting", "context"},
	{"scale", "replicas", "replica", "instances", "resize"},
	{"namespace", "namespaces", "ns", "project"},
	{"pod", "pods", "po"},
	{"deployment", "deploy", "deployments", "app", "workload"},
	{"container", "containers", "image"},
	{"file", "files", "folder", "directory", "dir"},
	{"all", "every", "everything", "each"},
	{"temporary", "temp", "aside", "later"},
	{"repository", "repo", "remote", "origin"},
	{"chart", "charts", "release", "package"},
	{"exec", "shell", "attach", "into", "inside", "run"},
}

// synonymsOf maps a root to its groups.
var synonymsOf = func() map[string][]int {
	m := map[string][]int{}
	for g, ws := range synonymGroups {
		for _, w := range ws {
			r := stem(w)
			if !slices.Contains(m[r], g) {
				m[r] = append(m[r], g)
			}
		}
	}
	return m
}()
