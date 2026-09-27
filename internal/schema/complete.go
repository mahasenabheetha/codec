package schema

import (
	"bytes"
	"regexp"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/santhosh-tekuri/jsonschema/v6"

	"github.com/mahasenabheetha/codec/v2/internal/yamlkit"
)

// Suggestion is one completion item.
type Suggestion struct {
	Label  string
	Insert string // text to insert; "" = Label
	Detail string // e.g. "integer · required"
	Doc    string // the field's description
	Kind   string // "key" or "value"
}

// Complete suggests keys or values at line/col (1-based, col in runes)
// from the raw text: while typing, the YAML is usually unparseable, so
// the path to the cursor is worked out from indentation. from is the
// column where the partial word being typed starts.
func Complete(root *jsonschema.Schema, content []byte, line, col int) (items []Suggestion, from int) {
	lines := splitLines(content)
	if root == nil || line < 1 || line > len(lines) {
		return nil, col
	}
	text := lines[line-1]
	prefix := string([]rune(text)[:min(utf8.RuneCountInString(text), max(col-1, 0))])
	cur := parseLine(prefix)

	// "key: val|" → allowed values for key.
	if m := reValue.FindStringSubmatch(prefix); m != nil && cur.hasKey {
		partial := m[1]
		path, ok := containerPath(lines, line-1, cur)
		if !ok {
			return nil, col
		}
		info, _ := InfoAt(root, append(path, Step{Key: cur.key}))
		values := info.Enum
		if len(values) == 0 && slices.Contains(info.Types, "boolean") {
			values = []string{"true", "false"}
		}
		for _, v := range values {
			if strings.HasPrefix(strings.ToLower(v), strings.ToLower(partial)) {
				items = append(items, Suggestion{Label: v, Kind: "value"})
			}
		}
		return items, col - utf8.RuneCountInString(partial)
	}

	// "  part|" or "- part|" → keys allowed in that map.
	m := reKey.FindStringSubmatch(prefix)
	if m == nil {
		return nil, col
	}
	partial := m[2]
	cur.keyCol = len(m[1])
	path, ok := containerPath(lines, line-1, cur)
	if !ok {
		return nil, col
	}
	have := siblings(lines, line-1, cur)
	for _, p := range Properties(root, path) {
		if have[p.Name] || !strings.HasPrefix(strings.ToLower(p.Name), strings.ToLower(partial)) {
			continue
		}
		detail := strings.Join(p.Info.Types, " | ")
		if p.Required {
			detail = strings.TrimPrefix(detail+" · required", " · ")
		}
		items = append(items, Suggestion{Label: p.Name, Insert: p.Name + ": ", Detail: detail, Doc: firstSentences(p.Info.Description), Kind: "key"})
	}
	return items, col - utf8.RuneCountInString(partial)
}

var (
	// indentation and "- " markers, then the partial key typed so far
	reKey = regexp.MustCompile(`^((?:\s|- )*)([\w.\-/]*)$`)
	// "key: " then the partial value typed so far
	reValue = regexp.MustCompile(`:\s+([\w.\-/]*)$`)
)

// lineInfo is what one raw line says about structure.
type lineInfo struct {
	blank  bool // empty or comment only
	marker bool // --- or ... (document boundary)
	indent int  // leading spaces
	dash   int  // column of the last "- " marker, -1 if none
	keyCol int  // column where the content (a key) starts
	key    string
	hasKey bool
	empty  bool // "key:" with nothing after: its value is on the next lines
}

func parseLine(s string) lineInfo {
	s = strings.TrimRight(s, "\r")
	t := strings.TrimLeft(s, " ")
	li := lineInfo{indent: len(s) - len(t), dash: -1}
	if strings.HasPrefix(s, "---") || strings.HasPrefix(s, "...") {
		li.marker = true
		return li
	}
	if t == "" || t[0] == '#' {
		li.blank = true
		return li
	}
	col := li.indent
	for t == "-" || strings.HasPrefix(t, "- ") {
		li.dash = col
		rest := t[1:]
		t = strings.TrimLeft(rest, " ")
		col += 1 + len(rest) - len(t)
	}
	li.keyCol = col
	key, after, ok := splitKey(t)
	if !ok {
		return li
	}
	li.key, li.hasKey = key, true
	if i := strings.Index(after, " #"); i >= 0 {
		after = after[:i]
	}
	after = strings.TrimSpace(after)
	li.empty = after == "" || (strings.HasPrefix(after, "&") || strings.HasPrefix(after, "!")) && !strings.Contains(after, " ")
	return li
}

// splitKey splits "key: value" (or "key:") into its parts.
func splitKey(t string) (key, after string, ok bool) {
	if t == "" || strings.ContainsRune("{[#|>", rune(t[0])) {
		return "", "", false
	}
	start := 0
	if q := t[0]; q == '"' || q == '\'' {
		end := strings.IndexByte(t[1:], q)
		if end < 0 {
			return "", "", false
		}
		start = end + 2
	}
	for i := start; i < len(t); i++ {
		if t[i] == ':' && (i+1 == len(t) || t[i+1] == ' ' || t[i+1] == '\t') {
			return strings.Trim(t[:i], `"' `), t[i+1:], true
		}
	}
	return "", "", false
}

// containerPath finds the path of the map that line i (parsed as cur)
// is an entry of, by walking up to the lines it is nested under.
func containerPath(lines []string, i int, cur lineInfo) ([]Step, bool) {
	var path []Step
	target := cur.keyCol
	needSeq, seqCol := false, 0 // looking for the key that owns a list
	if cur.dash >= 0 {
		path = []Step{{Item: true}}
		needSeq, seqCol = true, cur.dash
	}
	for j := i - 1; j >= 0; j-- {
		l := parseLine(lines[j])
		if l.marker {
			break
		}
		if l.blank {
			continue
		}
		start := l.indent // first non-space column
		if needSeq {
			switch {
			case start > seqCol || (l.dash == seqCol && start == seqCol):
				continue // inside a sibling item, or a sibling item
			case l.hasKey && l.empty && l.keyCol <= seqCol:
				path = append([]Step{{Key: l.key}}, path...)
				needSeq = false
				if l.dash >= 0 {
					path = append([]Step{{Item: true}}, path...)
					needSeq, seqCol = true, l.dash
				} else {
					target = l.keyCol
				}
				continue
			}
			return nil, false
		}
		if start >= target {
			continue // a sibling entry, or deeper inside one
		}
		switch {
		case l.dash >= 0 && l.keyCol == target:
			// "- a: 1": the cursor's map is this list item.
			path = append([]Step{{Item: true}}, path...)
			needSeq, seqCol = true, l.dash
		case l.hasKey && l.empty && l.keyCol < target:
			path = append([]Step{{Key: l.key}}, path...)
			if l.dash >= 0 {
				path = append([]Step{{Item: true}}, path...)
				needSeq, seqCol = true, l.dash
			} else {
				target = l.keyCol
			}
		default:
			return nil, false // nested under a line that has an inline value
		}
		if target == 0 && !needSeq {
			break
		}
	}
	if needSeq && seqCol > 0 {
		return nil, false
	}
	return path, true
}

// siblings collects the keys already present in the cursor's map.
func siblings(lines []string, i int, cur lineInfo) map[string]bool {
	have := map[string]bool{}
	col := cur.keyCol
	scan := func(j int) (stop bool) {
		l := parseLine(lines[j])
		if l.marker {
			return true
		}
		if l.blank {
			return false
		}
		if l.hasKey && l.keyCol == col {
			have[l.key] = true
		}
		start := l.indent
		if l.dash >= 0 {
			start = l.dash
		}
		return start < col
	}
	if cur.dash < 0 { // a new item has no siblings above it
		for j := i - 1; j >= 0; j-- {
			if scan(j) {
				break
			}
		}
	}
	for j := i + 1; j < len(lines); j++ {
		if scan(j) {
			break
		}
	}
	return have
}

func splitLines(content []byte) []string {
	content = bytes.TrimPrefix(content, []byte{0xEF, 0xBB, 0xBF})
	return strings.Split(string(content), "\n")
}

// firstSentences keeps a long description to its first two sentences.
func firstSentences(s string) string {
	n := 0
	for i := 0; i+1 < len(s); i++ {
		if s[i] == '.' && s[i+1] == ' ' {
			if n++; n == 2 {
				return s[:i+1]
			}
		}
	}
	return s
}

// StepsOf converts a yamlkit path (with concrete indexes) to schema steps.
func StepsOf(p yamlkit.Path) []Step {
	out := make([]Step, len(p))
	for i, s := range p {
		out[i] = Step{Key: s.Key, Item: s.IsIndex}
	}
	return out
}

// Identity finds apiVersion and kind of the document around line
// (1-based) from raw text, for when the YAML doesn't parse mid-edit.
func Identity(content []byte, line int) (apiVersion, kind string) {
	lines := splitLines(content)
	start, end := 0, len(lines)
	for j := min(line-1, len(lines)-1); j >= 0; j-- {
		if parseLine(lines[j]).marker {
			start = j + 1
			break
		}
	}
	for j := max(line, start); j < len(lines); j++ {
		if parseLine(lines[j]).marker {
			end = j
			break
		}
	}
	for _, l := range lines[start:end] {
		if v, ok := strings.CutPrefix(l, "apiVersion:"); ok {
			apiVersion = strings.Trim(strings.TrimSpace(v), `"'`)
		}
		if v, ok := strings.CutPrefix(l, "kind:"); ok {
			kind = strings.Trim(strings.TrimSpace(v), `"'`)
		}
	}
	return apiVersion, kind
}
