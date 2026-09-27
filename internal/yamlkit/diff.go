package yamlkit

import (
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"

	yaml "go.yaml.in/yaml/v3"
)

// ChangeKind says what happened to a value between two versions.
type ChangeKind string

const (
	Added     ChangeKind = "added"
	Removed   ChangeKind = "removed"
	Changed   ChangeKind = "changed"
	Reordered ChangeKind = "reordered" // same items, different order, where order matters
)

// Change is one semantic difference.
type Change struct {
	Kind ChangeKind `json:"kind"`
	// Doc names the document: "Deployment prod/api" for Kubernetes
	// objects, "#2" (1-based) otherwise.
	Doc string `json:"doc"`
	// Path within the document, e.g. spec.template.spec.containers[name=api].image;
	// "" is the whole document.
	Path     string `json:"path"`
	Old      string `json:"old,omitempty"` // YAML text of the old value
	New      string `json:"new,omitempty"`
	OldRange *Range `json:"oldRange,omitempty"`
	NewRange *Range `json:"newRange,omitempty"`
}

// DiffOptions tune a comparison.
type DiffOptions struct {
	// Ignore lists path patterns whose changes don't matter, e.g.
	// "metadata.labels.helm.sh/chart" or "spec.template.spec.containers[*].image".
	// "*" matches any run of characters.
	Ignore []string
}

// ErrUnparsed means a side has syntax errors: compare it as text.
var ErrUnparsed = errors.New("a file has syntax errors, so it can't be compared by structure")

// Diff compares two files by meaning, not text: key order doesn't
// matter, documents pair up by identity (kind, namespace and name),
// and list items by an identity key such as "name". What remains is
// reported as added, removed or changed values, by path.
func Diff(a, b *File, opts DiffOptions) ([]Change, error) {
	if a.HasErrors() || b.HasErrors() {
		return nil, ErrUnparsed
	}
	ra, err := resolveDocs(a)
	if err != nil {
		return nil, err
	}
	rb, err := resolveDocs(b)
	if err != nil {
		return nil, err
	}
	d := &differ{ignore: compileIgnore(opts.Ignore)}

	// Two single-document files are the same document by definition,
	// even if their names differ (the difference shows as a change).
	if len(ra) == 1 && len(rb) == 1 {
		d.doc = docKeys(ra)[0]
		d.walk(nil, ra[0], rb[0])
		return d.out, nil
	}
	ka, kb := docKeys(ra), docKeys(rb)
	pairs, onlyA, onlyB := matchBy(indexes(ra), indexes(rb),
		func(i int) (string, bool) { return ka[i], true },
		func(i int) (string, bool) { return kb[i], true })
	// Walk in the new file's order, with removals where they were.
	for _, p := range pairs {
		d.doc = kb[p[1]]
		d.walk(nil, ra[p[0]], rb[p[1]])
	}
	for _, i := range onlyA {
		d.doc = ka[i]
		d.walk(nil, ra[i], nil)
	}
	for _, i := range onlyB {
		d.doc = kb[i]
		d.walk(nil, nil, rb[i])
	}
	return d.out, nil
}

func resolveDocs(f *File) ([]*Node, error) {
	var out []*Node
	for _, d := range f.Docs {
		if d.Root == nil {
			continue
		}
		r, err := Resolve(d.Root)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, nil
}

func indexes[T any](s []T) []int {
	out := make([]int, len(s))
	for i := range s {
		out[i] = i
	}
	return out
}

// docKeys names each document; duplicates get "#n" so they stay unique.
func docKeys(roots []*Node) []string {
	keys := make([]string, len(roots))
	seen := map[string]int{}
	for i, r := range roots {
		k := DocIdentity(r)
		if k == "" {
			k = fmt.Sprintf("#%d", i+1)
		}
		if seen[k]++; seen[k] > 1 {
			k = fmt.Sprintf("%s #%d", k, seen[k])
		}
		keys[i] = k
	}
	return keys
}

// DocIdentity names a Kubernetes object as "Kind namespace/name" (or
// "Kind name" without a namespace); "" for anything else.
func DocIdentity(root *Node) string {
	kind := root.Get("kind").Str()
	meta := root.Get("metadata")
	name := meta.Get("name").Str()
	if name == "" && meta.Get("generateName").Str() != "" {
		name = meta.Get("generateName").Str() + "*"
	}
	if kind == "" || name == "" {
		return ""
	}
	if ns := meta.Get("namespace").Str(); ns != "" {
		return kind + " " + ns + "/" + name
	}
	return kind + " " + name
}

// matchBy pairs items of a and b whose keys are equal (first come,
// first matched); items without a key never match. It returns index
// pairs in b's order, then the leftovers of each side.
func matchBy[A, B any](a []A, b []B, keyA func(A) (string, bool), keyB func(B) (string, bool)) (pairs [][2]int, onlyA, onlyB []int) {
	byKey := map[string][]int{}
	for i, x := range a {
		if k, ok := keyA(x); ok {
			byKey[k] = append(byKey[k], i)
		}
	}
	used := make([]bool, len(a))
	for j, y := range b {
		k, ok := keyB(y)
		if q := byKey[k]; ok && len(q) > 0 {
			pairs = append(pairs, [2]int{q[0], j})
			used[q[0]] = true
			byKey[k] = q[1:]
			continue
		}
		onlyB = append(onlyB, j)
	}
	for i := range a {
		if !used[i] {
			onlyA = append(onlyA, i)
		}
	}
	return pairs, onlyA, onlyB
}

// --- the walk ---

type differ struct {
	ignore []*regexp.Regexp
	doc    string
	out    []Change
}

func (d *differ) ignored(path string) bool {
	for _, re := range d.ignore {
		if re.MatchString(path) {
			return true
		}
	}
	return false
}

func (d *differ) add(kind ChangeKind, path string, a, b *Node) {
	c := Change{Kind: kind, Doc: d.doc, Path: path}
	if a != nil {
		r := a.Range
		c.OldRange, c.Old = &r, ValueText(a)
	}
	if b != nil {
		r := b.Range
		c.NewRange, c.New = &r, ValueText(b)
	}
	d.out = append(d.out, c)
}

func (d *differ) walk(path []string, a, b *Node) {
	p := strings.Join(path, "")
	if d.ignored(p) {
		return
	}
	switch {
	case a == nil && b == nil:
	case a == nil:
		d.add(Added, p, nil, b)
	case b == nil:
		d.add(Removed, p, a, nil)
	case a.Kind == KindMap && b.Kind == KindMap:
		d.maps(path, a, b)
	case a.Kind == KindSeq && b.Kind == KindSeq:
		d.seqs(path, a, b)
	case a.Kind == KindScalar && b.Kind == KindScalar:
		if !scalarEqual(a, b) {
			d.add(Changed, p, a, b)
		}
	default:
		d.add(Changed, p, a, b)
	}
}

// child extends a path by a map key (keys are shown as written, even
// when they contain dots, like helm.sh/chart).
func child(path []string, key string) []string {
	seg := key
	if len(path) > 0 {
		seg = "." + seg
	}
	return append(slices.Clip(path), seg)
}

func (d *differ) maps(path []string, a, b *Node) {
	va := map[string]*Node{}
	for _, p := range a.Pairs {
		va[keyText(p.Key)] = p.Value
	}
	seen := map[string]bool{}
	for _, p := range b.Pairs {
		k := keyText(p.Key)
		seen[k] = true
		old, ok := va[k]
		if !ok {
			if !d.ignored(strings.Join(child(path, k), "")) {
				d.add(Added, strings.Join(child(path, k), ""), nil, orNull(p.Value, p.Key))
			}
			continue
		}
		d.walk(child(path, k), orNull(old, nil), orNull(p.Value, p.Key))
	}
	for _, p := range a.Pairs {
		if k := keyText(p.Key); !seen[k] {
			if cp := strings.Join(child(path, k), ""); !d.ignored(cp) {
				d.add(Removed, cp, orNull(p.Value, p.Key), nil)
			}
		}
	}
}

// orNull stands in an empty value for a missing one, placed at its key.
func orNull(n, key *Node) *Node {
	if n != nil {
		return n
	}
	r := Range{}
	if key != nil {
		r = key.Range
	}
	return &Node{Kind: KindScalar, Tag: TagNull, Range: r}
}

// orderMatters lists keys whose scalar lists mean something in order.
var orderMatters = map[string]bool{"command": true, "args": true, "entrypoint": true, "script": true}

// identityKeys are tried, in order, to pair up list items of maps.
var identityKeys = []string{"name", "key", "mountPath", "containerPort", "port", "id"}

func (d *differ) seqs(path []string, a, b *Node) {
	p := strings.Join(path, "")
	if allScalars(a) && allScalars(b) {
		if seqEqual(a, b) {
			return
		}
		ta, tb := scalarTexts(a), scalarTexts(b)
		ordered := orderMatters[lastKey(path)]
		switch {
		case sameMultiset(ta, tb) && ordered:
			d.add(Reordered, p, a, b)
			return
		case sameMultiset(ta, tb):
			return
		case ordered:
			// A command line reads as a whole: show before and after.
			d.add(Changed, p, a, b)
			return
		}
		// Items as a set: report what disappeared and what appeared.
		left := map[string]int{}
		for _, t := range tb {
			left[t]++
		}
		for i, t := range ta {
			if left[t] > 0 {
				left[t]--
				continue
			}
			d.add(Removed, p+"["+strconv.Itoa(i)+"]", a.Items[i], nil)
		}
		right := map[string]int{}
		for _, t := range ta {
			right[t]++
		}
		for j, t := range tb {
			if right[t] > 0 {
				right[t]--
				continue
			}
			d.add(Added, p+"["+strconv.Itoa(j)+"]", nil, b.Items[j])
		}
		return
	}

	key := identityKey(a.Items, b.Items)
	id := func(n *Node) (string, bool) {
		if key == "" {
			return "", false
		}
		return n.Get(key).Str(), true
	}
	pairs, onlyA, onlyB := matchBy(a.Items, b.Items, id, id)
	seg := func(n *Node, i int) string {
		if key != "" {
			return "[" + key + "=" + n.Get(key).Str() + "]"
		}
		return "[" + strconv.Itoa(i) + "]"
	}
	// Leftovers pair up when they are mostly alike (a renamed
	// container is a change, not a removal plus an addition).
	more, onlyA, onlyB := pairSimilar(a.Items, b.Items, onlyA, onlyB)
	pairs = append(pairs, more...)
	slices.SortFunc(pairs, func(x, y [2]int) int { return x[1] - y[1] })
	for _, pr := range pairs {
		d.walk(append(slices.Clip(path), seg(a.Items[pr[0]], pr[0])), a.Items[pr[0]], b.Items[pr[1]])
	}
	for _, i := range onlyA {
		d.walk(append(slices.Clip(path), seg(a.Items[i], i)), a.Items[i], nil)
	}
	for _, j := range onlyB {
		d.walk(append(slices.Clip(path), seg(b.Items[j], j)), nil, b.Items[j])
	}
}

// identityKey picks a key that every item on both sides has, with
// values unique within each side.
func identityKey(a, b []*Node) string {
	for _, k := range identityKeys {
		if uniqueKey(a, k) && uniqueKey(b, k) {
			return k
		}
	}
	return ""
}

func uniqueKey(items []*Node, k string) bool {
	seen := map[string]bool{}
	for _, it := range items {
		v := it.Get(k)
		if v == nil || v.Kind != KindScalar || v.Templated || seen[v.Value] {
			return false
		}
		seen[v.Value] = true
	}
	return true
}

// pairSimilar matches leftover items whose leaves are at least half the
// same, best matches first.
func pairSimilar(a, b []*Node, onlyA, onlyB []int) (pairs [][2]int, restA, restB []int) {
	used := map[int]bool{}
	for _, i := range onlyA {
		best, bestScore := -1, 0.5
		la := leaves(a[i])
		for _, j := range onlyB {
			if used[j] {
				continue
			}
			if s := similarity(la, leaves(b[j])); s >= bestScore {
				best, bestScore = j, s
			}
		}
		if best >= 0 {
			used[best] = true
			pairs = append(pairs, [2]int{i, best})
		} else {
			restA = append(restA, i)
		}
	}
	for _, j := range onlyB {
		if !used[j] {
			restB = append(restB, j)
		}
	}
	return pairs, restA, restB
}

// leaves flattens a node into path → scalar text.
func leaves(n *Node) map[string]string {
	out := map[string]string{}
	var walk func(p string, n *Node)
	walk = func(p string, n *Node) {
		switch {
		case n == nil:
		case n.Kind == KindMap:
			for _, pr := range n.Pairs {
				walk(p+"."+keyText(pr.Key), pr.Value)
			}
		case n.Kind == KindSeq:
			for i, it := range n.Items {
				walk(p+"["+strconv.Itoa(i)+"]", it)
			}
		default:
			out[p] = scalarText(n)
		}
	}
	walk("", n)
	return out
}

func similarity(a, b map[string]string) float64 {
	if len(a) == 0 && len(b) == 0 {
		return 1
	}
	same := 0
	for k, v := range a {
		if b[k] == v {
			same++
		}
	}
	return float64(same) / float64(max(len(a), len(b)))
}

func allScalars(n *Node) bool {
	for _, it := range n.Items {
		if it == nil || it.Kind != KindScalar {
			return false
		}
	}
	return true
}

func seqEqual(a, b *Node) bool {
	if len(a.Items) != len(b.Items) {
		return false
	}
	for i := range a.Items {
		if !scalarEqual(a.Items[i], b.Items[i]) {
			return false
		}
	}
	return true
}

func scalarTexts(n *Node) []string {
	out := make([]string, len(n.Items))
	for i, it := range n.Items {
		out[i] = canonical(it)
	}
	return out
}

func sameMultiset(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	x, y := slices.Clone(a), slices.Clone(b)
	slices.Sort(x)
	slices.Sort(y)
	return slices.Equal(x, y)
}

func lastKey(path []string) string {
	if len(path) == 0 {
		return ""
	}
	return strings.TrimPrefix(path[len(path)-1], ".")
}

// scalarEqual compares by meaning: 1 and 1.0 differ from "1", but
// null, ~ and an empty value are the same, as are 0x10 and 16.
func scalarEqual(a, b *Node) bool {
	return canonical(a) == canonical(b)
}

func canonical(n *Node) string {
	switch n.Tag {
	case TagNull:
		return "null"
	case TagBool:
		return strings.ToLower(n.Value)
	case TagInt:
		if i, err := strconv.ParseInt(strings.ReplaceAll(n.Value, "_", ""), 0, 64); err == nil {
			return "int:" + strconv.FormatInt(i, 10)
		}
	case TagFloat:
		if f, err := strconv.ParseFloat(n.Value, 64); err == nil {
			return "float:" + strconv.FormatFloat(f, 'g', -1, 64)
		}
	}
	return n.Tag + ":" + n.Value
}

// scalarText shows a scalar as YAML would, quoting text that would
// otherwise read as another type ("80", "true").
func scalarText(n *Node) string {
	switch {
	case n.Tag == TagNull:
		return "null"
	case n.Tag == TagStr && needsQuotes(n.Value):
		return strconv.Quote(n.Value)
	}
	return n.Value
}

// needsQuotes reports whether plain text would read differently as a
// YAML plain scalar (another type, or broken syntax).
func needsQuotes(s string) bool {
	return s == "" || resolvePlain(s) != TagStr || strings.TrimSpace(s) != s ||
		strings.Contains(s, ": ") || strings.Contains(s, " #") || strings.ContainsRune(s, '\n') ||
		strings.HasSuffix(s, ":") || strings.ContainsRune(",[]{}#&*!|>'\"%@`", rune(s[0])) ||
		s == "-" || s == "?" || strings.HasPrefix(s, "- ") || strings.HasPrefix(s, "? ") || strings.HasPrefix(s, ": ")
}

// ValueText renders a value as YAML, clipped to 20 lines for display.
func ValueText(n *Node) string {
	if n.Kind == KindScalar {
		return scalarText(n)
	}
	out, err := yaml.Marshal(toYAMLNode(n))
	if err != nil {
		return ""
	}
	s := strings.TrimRight(string(out), "\n")
	if lines := strings.Split(s, "\n"); len(lines) > 20 {
		s = strings.Join(lines[:20], "\n") + "\n…"
	}
	return s
}

// compileIgnore turns glob patterns into anchored regexps.
func compileIgnore(patterns []string) []*regexp.Regexp {
	var out []*regexp.Regexp
	for _, p := range patterns {
		if p = strings.TrimSpace(p); p == "" {
			continue
		}
		re := "^" + strings.ReplaceAll(regexp.QuoteMeta(p), `\*`, ".*") + "$"
		out = append(out, regexp.MustCompile(re))
	}
	return out
}
