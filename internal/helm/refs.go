package helm

import (
	"bytes"
	"fmt"
	"regexp"
	"slices"
	"sort"
	"strings"
	"unicode/utf8"

	chart "helm.sh/helm/v4/pkg/chart/v2"
	"helm.sh/helm/v4/pkg/chart/v2/loader"

	"github.com/mahasenabheetha/codec/v2/internal/yamlkit"
)

var (
	// .Values.image.tag and $.Values.image.tag
	reValuesRef = regexp.MustCompile(`\$?\.Values((?:\.[A-Za-z_][A-Za-z0-9_]*)+)`)
	// index .Values "image" "tag"
	reIndexRef = regexp.MustCompile(`index \$?\.Values((?: +"[^"]+")+)`)
	reQuoted   = regexp.MustCompile(`"([^"]+)"`)
	// .Values on its own (toYaml .Values, with .Values, passing it on):
	// then any value may be used, so "unused" can't be judged.
	reBareValues = regexp.MustCompile(`\$?\.Values(?:[^.\w]|$)`)
)

// ref is one .Values path used in a template.
type ref struct {
	path      []string
	file      string
	line, col int
	// guarded: the template checks or defaults it (if, with, default,
	// required, hasKey, …), so an unset value is intended, not a bug.
	guarded bool
}

// checkReferences compares the .Values paths templates use with the
// values that exist: used-but-never-set paths are warnings (they render
// empty), set-but-never-used defaults are infos.
func checkReferences(ch *chart.Chart, files []File, final map[string]any) []Diagnostic {
	scopes := chartScopes(ch, files)
	var out []Diagnostic
	used := map[string][][]string{} // scope prefix -> referenced paths
	bare := map[string]bool{}

	for _, f := range files {
		scope, ok := templateScope(f.Name, scopes)
		if !ok {
			continue
		}
		refs, hasBare := findRefs(f)
		if hasBare {
			bare[scope] = true
		}
		seen := map[string]bool{}
		for _, r := range refs {
			used[scope] = append(used[scope], r.path)
			key := strings.Join(r.path, ".")
			if seen[key] {
				continue
			}
			seen[key] = true
			vals := final
			if scope != "" {
				vals, _ = final[scope].(map[string]any)
			}
			if !r.guarded && !defined(vals, r.path) {
				out = append(out, Diagnostic{
					Severity: "warning", Code: "undefined-value", File: r.file, Line: r.line, Col: r.col,
					Message: fmt.Sprintf(".Values.%s is not set by any values layer", key),
					Hint:    "It renders as empty. Set it in values.yaml (even as null) or guard it with if/default.",
				})
			}
		}
	}

	// Unused defaults in the chart's own values.yaml.
	raw := fileData(files, "values.yaml")
	if raw == nil || bare[""] {
		return out
	}
	defaults, _ := loader.LoadValues(bytes.NewReader(raw))
	lines := keyLines(raw)
	deps := map[string]bool{}
	for _, d := range ch.Dependencies() {
		if d.Metadata != nil {
			deps[d.Metadata.Name] = true
		}
	}
	for _, leaf := range leaves(defaults, nil) {
		head := leaf[0].Key
		if head == "global" && len(deps) > 0 {
			continue // globals feed subcharts, whose use we may not see
		}
		// Values for a subchart are used by that subchart's templates.
		scope, rest := "", pathKeys(leaf)
		if deps[head] {
			if _, visible := scopes[head]; !visible || bare[head] {
				continue // packaged subchart: its templates aren't in view
			}
			scope, rest = head, rest[1:]
		}
		if len(rest) == 0 || coveredBy(rest, used[scope]) {
			continue
		}
		out = append(out, Diagnostic{
			Severity: "info", Code: "unused-value", File: "values.yaml", Line: lines[leaf.String()],
			Message: fmt.Sprintf("%s is set but no template uses it", leaf.String()),
		})
	}
	return out
}

// chartScopes maps a subchart's values key to the directory holding its
// templates ("" is the chart itself). Only unpacked subcharts are
// visible; packaged (.tgz) ones are skipped.
func chartScopes(ch *chart.Chart, files []File) map[string]string {
	scopes := map[string]string{"": ""}
	byName := map[string]bool{}
	for _, d := range ch.Dependencies() {
		if d.Metadata != nil {
			byName[d.Metadata.Name] = true
		}
	}
	reName := regexp.MustCompile(`(?m)^name:\s*["']?([^"'\s#]+)`)
	for _, f := range files {
		dir, ok := strings.CutSuffix(f.Name, "/Chart.yaml")
		if !ok || !strings.HasPrefix(dir, "charts/") || strings.Count(dir, "/") != 1 {
			continue
		}
		if m := reName.FindSubmatch(f.Data); m != nil && byName[string(m[1])] {
			scopes[string(m[1])] = dir + "/"
		}
	}
	return scopes
}

// templateScope says which chart a template belongs to.
func templateScope(name string, scopes map[string]string) (string, bool) {
	if !isTemplate(name) {
		return "", false
	}
	best, bestDir := "", ""
	for scope, dir := range scopes {
		if strings.HasPrefix(name, dir+"templates/") && len(dir) >= len(bestDir) {
			best, bestDir = scope, dir
		}
	}
	if best == "" && !strings.HasPrefix(name, "templates/") {
		return "", false
	}
	return best, true
}

func isTemplate(name string) bool {
	if !strings.Contains(name, "templates/") {
		return false
	}
	for _, ext := range []string{".yaml", ".yml", ".tpl", ".txt", ".json"} {
		if strings.HasSuffix(name, ext) {
			return true
		}
	}
	return false
}

// defined reports whether path exists in vals, allowing it to end
// inside a non-map value (e.g. a field of a list item via index).
func defined(vals map[string]any, path []string) bool {
	var cur any = vals
	for _, k := range path {
		m, ok := cur.(map[string]any)
		if !ok {
			return cur != nil // stepped into a scalar/list: not ours to judge
		}
		if cur, ok = m[k]; !ok {
			return false
		}
	}
	return true
}

// leaves lists the paths of all non-map values (maps recurse, lists
// and scalars are leaves; empty maps count as leaves too).
func leaves(vals map[string]any, path yamlkit.Path) []yamlkit.Path {
	var out []yamlkit.Path
	for k, v := range vals {
		p := append(slices.Clone(path), yamlkit.Segment{Key: k})
		if m, ok := v.(map[string]any); ok && len(m) > 0 {
			out = append(out, leaves(m, p)...)
			continue
		}
		out = append(out, p)
	}
	slices.SortFunc(out, func(a, b yamlkit.Path) int { return strings.Compare(a.String(), b.String()) })
	return out
}

func pathKeys(p yamlkit.Path) []string {
	keys := make([]string, len(p))
	for i, s := range p {
		keys[i] = s.Key
	}
	return keys
}

// coveredBy: a leaf counts as used when a reference names it, one of
// its parents (toYaml .Values.resources), or something inside it.
func coveredBy(leaf []string, refs [][]string) bool {
	for _, r := range refs {
		n := min(len(r), len(leaf))
		if slices.Equal(r[:n], leaf[:n]) {
			return true
		}
	}
	return false
}

// reAction finds {{ … }} actions (across lines).
var reAction = regexp.MustCompile(`(?s)\{\{(.*?)\}\}`)

// guards are template constructs that make an unset value harmless.
var reGuard = regexp.MustCompile(`^(?:if|else if|with|else with|range)\b|\b(?:default|required|hasKey|empty|coalesce|ternary)\b`)

// findRefs lists .Values paths in a template with their positions. A
// reference is guarded when its action checks or defaults it, or when
// it sits inside an if/with/range block on the same (or a parent) path:
//
//	{{- if hasKey .Values.pdb "maxUnavailable" }}
//	maxUnavailable: {{ .Values.pdb.maxUnavailable }}   <- guarded
func findRefs(f File) (refs []ref, bare bool) {
	src := string(f.Data)
	starts := []int{0}
	for i, c := range src {
		if c == '\n' {
			starts = append(starts, i+1)
		}
	}
	lineCol := func(off int) (int, int) {
		i := sort.SearchInts(starts, off+1) - 1
		return i + 1, utf8.RuneCountInString(src[starts[i]:off]) + 1
	}

	var stack [][][]string // open blocks, each with the paths it guards
	guardedBy := func(p []string) bool {
		for _, frame := range stack {
			for _, g := range frame {
				if len(g) <= len(p) && slices.Equal(g, p[:len(g)]) {
					return true
				}
			}
		}
		return false
	}

	for _, m := range reAction.FindAllStringSubmatchIndex(src, -1) {
		bodyStart := m[2]
		body := strings.TrimSpace(strings.Trim(src[m[2]:m[3]], "-"))
		if strings.HasPrefix(body, "/*") {
			continue // a comment
		}
		if reBareValues.MatchString(body + " ") {
			bare = true
		}

		var here []ref
		add := func(path []string, off int) {
			line, col := lineCol(off)
			here = append(here, ref{path: path, file: f.Name, line: line, col: col})
		}
		raw := src[m[2]:m[3]]
		for _, rm := range reValuesRef.FindAllStringSubmatchIndex(raw, -1) {
			add(strings.Split(strings.TrimPrefix(raw[rm[2]:rm[3]], "."), "."), bodyStart+rm[0])
		}
		for _, rm := range reIndexRef.FindAllStringSubmatchIndex(raw, -1) {
			var path []string
			for _, q := range reQuoted.FindAllStringSubmatch(raw[rm[2]:rm[3]], -1) {
				path = append(path, q[1])
			}
			add(path, bodyStart+rm[0])
		}

		checked := reGuard.MatchString(body)
		for i := range here {
			here[i].guarded = checked || guardedBy(here[i].path)
		}
		refs = append(refs, here...)

		// Track blocks: what an if/with/range tests is safe inside it.
		word, _, _ := strings.Cut(body, " ")
		switch word {
		case "if", "with", "range", "define", "block":
			var frame [][]string
			if word != "define" && word != "block" {
				for _, r := range here {
					frame = append(frame, r.path)
				}
			}
			stack = append(stack, frame)
		case "else":
			if len(stack) > 0 {
				for _, r := range here {
					stack[len(stack)-1] = append(stack[len(stack)-1], r.path)
				}
			}
		case "end":
			if len(stack) > 0 {
				stack = stack[:len(stack)-1]
			}
		}
	}
	return refs, bare
}
