package ansible

import (
	"bytes"
	"cmp"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/mahasenabheetha/codec/v2/internal/yamlkit"
)

// Match is a task definition a log line may refer to.
type Match struct {
	File  string        `json:"file"`
	Line  int           `json:"line"`
	Range yamlkit.Range `json:"range"`
	Name  string        `json:"name"`           // the task's name as written
	Role  string        `json:"role,omitempty"` // the role the file belongs to
	Why   string        `json:"why"`            // task path, exact name, name with variables, similar name
	Score int           `json:"score"`
}

var lineSuffix = regexp.MustCompile(`:(\d+)$`)

// FindTask finds the definitions of the task a log names: "TASK [role :
// name]" gives logName "role : name"; "task path: /…/file.yml:12"
// gives logPath. A path matching a workspace file by its trailing
// folders wins; otherwise task names are compared, the ones written
// with {{ variables }} matching any value, and the log's role
// preferred.
func FindTask(logName, logPath string, files []string, load Loader) []Match {
	role, name := "", strings.TrimSpace(logName)
	if r, n, ok := strings.Cut(name, " : "); ok {
		role, name = strings.TrimSpace(r), strings.TrimSpace(n)
		if i := strings.LastIndexByte(role, '.'); i >= 0 {
			role = role[i+1:] // namespace.collection.role
		}
	}
	var out []Match
	pathFound := false
	if m, ok := byPath(logPath, files, load); ok {
		out = append(out, m)
		_, why := nameScore(m.Name, name)
		pathFound = why != ""
	}
	if name == "" {
		return out
	}
	words := significant(name)
	for _, f := range files {
		if !isYAML(f) {
			continue
		}
		data, err := load(f)
		if err != nil || !bytes.Contains(data, []byte("name")) || !containsAny(data, words) {
			continue
		}
		fileRole, _ := roleOf(f)
		for _, t := range namedTasks(yamlkit.Parse(data)) {
			score, why := nameScore(text(t.Get("name")), name)
			if score == 0 {
				continue
			}
			switch {
			case role != "" && fileRole == role:
				score += 20
			case role != "" && fileRole != role:
				score -= 30
			case role == "" && fileRole != "":
				score -= 5 // a task outside a role logs without a prefix
			}
			m := Match{File: f, Line: t.Range.Start.Line, Range: t.Range, Name: text(t.Get("name")), Role: fileRole, Why: why, Score: score}
			// When the task path found the task, only exact namesakes (a
			// copy of the role elsewhere) are worth listing too.
			if pathFound && (m.Why != "exact name" || (out[0].File == m.File && out[0].Line == m.Line)) {
				continue
			}
			out = append(out, m)
		}
	}
	slices.SortStableFunc(out, func(x, y Match) int {
		return cmp.Or(cmp.Compare(y.Score, x.Score), cmp.Compare(x.File, y.File), cmp.Compare(x.Line, y.Line))
	})
	if len(out) > 20 {
		out = out[:20]
	}
	return orEmpty(out)
}

// byPath maps a log's task path to a workspace file by the longest run
// of trailing path segments they share.
func byPath(logPath string, files []string, load Loader) (Match, bool) {
	p := strings.ReplaceAll(strings.TrimSpace(logPath), `\`, "/")
	if p == "" {
		return Match{}, false
	}
	line := 0
	if m := lineSuffix.FindStringSubmatch(p); m != nil {
		line, _ = strconv.Atoi(m[1])
		p = strings.TrimSuffix(p, m[0])
	}
	// Inside a role ("…/roles/app/tasks/main.yml") the role's name must
	// be shared too: every role has a tasks/main.yml.
	// The role is the last "roles/<name>/<part>/…" in the path, so a
	// "roles" folder higher up (/builds/roles/repo/…) doesn't count.
	need := 1
	segs := strings.Split(p, "/")
	for i := len(segs) - 3; i >= 0; i-- {
		if segs[i] == "roles" && slices.Contains(rolePartNames, segs[i+2]) {
			need = len(segs) - 1 - i
			break
		}
	}
	best, bestN := "", 0
	for _, f := range files {
		if n := sharedTail(p, f); n >= need && n > bestN {
			best, bestN = f, n
		}
	}
	if best == "" {
		return Match{}, false
	}
	m := Match{File: best, Line: max(line, 1), Why: "task path", Score: 1000}
	m.Role, _ = roleOf(best)
	// Name the task that starts on that line.
	if data, err := load(best); err == nil {
		for _, t := range namedTasks(yamlkit.Parse(data)) {
			if t.Range.Start.Line == line {
				m.Name, m.Range = text(t.Get("name")), t.Range
			}
		}
	}
	return m, true
}

// sharedTail counts the trailing path segments a and b share; the file
// name must match.
func sharedTail(a, b string) int {
	as, bs := strings.Split(a, "/"), strings.Split(b, "/")
	n := 0
	for n < len(as) && n < len(bs) && as[len(as)-1-n] == bs[len(bs)-1-n] {
		n++
	}
	return n
}

// namedTasks finds maps with a name inside lists, anywhere in the file
// (tasks, handlers, blocks, rescue, always), except plays.
func namedTasks(f *yamlkit.File) []*yamlkit.Node {
	var out []*yamlkit.Node
	var walk func(n *yamlkit.Node, inList bool)
	walk = func(n *yamlkit.Node, inList bool) {
		switch {
		case n == nil:
		case n.Kind == yamlkit.KindSeq:
			for _, it := range n.Items {
				walk(it, true)
			}
		case n.Kind == yamlkit.KindMap:
			if inList && text(n.Get("name")) != "" && n.Get("hosts") == nil && len(n.Pairs) >= 2 {
				out = append(out, n)
			}
			for _, p := range n.Pairs {
				walk(p.Value, false)
			}
		}
	}
	for _, d := range f.Docs {
		walk(d.Root, false)
	}
	return out
}

var jinjaExpr = regexp.MustCompile(`\{\{.*?\}\}`)

// nameScore compares a task name as written with the name a log shows.
func nameScore(written, logged string) (int, string) {
	written = strings.TrimSpace(written)
	switch {
	case written == "":
		return 0, ""
	case written == logged:
		return 100, "exact name"
	case strings.EqualFold(written, logged):
		return 70, "name, other case"
	case strings.Contains(written, "{{"):
		// Each {{ … }} matches whatever value it had in the run.
		// A name that is mostly variables would match anything.
		if len(strings.TrimSpace(jinjaExpr.ReplaceAllString(written, ""))) < 4 {
			return 0, ""
		}
		parts := jinjaExpr.Split(written, -1)
		for i := range parts {
			parts[i] = regexp.QuoteMeta(parts[i])
		}
		if re, err := regexp.Compile("^" + strings.Join(parts, ".*?") + "$"); err == nil && re.MatchString(logged) {
			return 80, "name with variables"
		}
	}
	return 0, ""
}

// significant picks words of a log name that a definition probably
// contains literally, for a quick filter before parsing.
func significant(name string) [][]byte {
	var out [][]byte
	for _, w := range strings.FieldsFunc(name, func(r rune) bool {
		return !(r == '_' || r == '-' || r >= '0' && r <= '9' || r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z')
	}) {
		if len(w) >= 4 {
			out = append(out, []byte(w))
		}
	}
	if len(out) == 0 {
		out = append(out, []byte(name))
	}
	return out
}

func containsAny(data []byte, words [][]byte) bool {
	for _, w := range words {
		if bytes.Contains(data, w) {
			return true
		}
	}
	return false
}
