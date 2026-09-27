package scaffold

import (
	"bytes"
	"cmp"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/mahasenabheetha/codec/v2/internal/yamlkit"
)

// CloneOptions say what to clone and how to rename it.
type CloneOptions struct {
	Doc  int    // document index; -1 = the whole file
	From string // name to replace; "" = the first metadata.name (or top-level name)
	To   string // new name
	// Flip lists change IDs whose default is swapped: an applied rename
	// is left out, a suggestion is applied.
	Flip []string
}

// CloneResult is the renamed copy and what changed in it.
type CloneResult struct {
	From    string   `json:"from"`
	To      string   `json:"to"`
	Text    string   `json:"text"`
	Changes []Change `json:"changes"`
}

// Change is one value (or key) the clone renamed, could rename, or
// that usually needs a look in a copy.
type Change struct {
	ID string `json:"id"` // stable across calls: the offset in the original
	// Kind is rename (applied unless flipped), suggest (not applied
	// unless flipped: it points outside the clone), or check (nothing
	// to rename, but copies usually change it).
	Kind    string `json:"kind"`
	Applied bool   `json:"applied"`
	Path    string `json:"path"`
	Key     bool   `json:"key,omitempty"` // the change is to a map key
	Line    int    `json:"line"`          // 1-based, in Text
	Before  string `json:"before"`
	After   string `json:"after,omitempty"`
	Reason  string `json:"reason,omitempty"`
}

// Field names by what they hold. Paths are matched by their last keys,
// list indexes ignored.
var (
	// refFields name another object of the given kinds ("" = any).
	// Renamed only when that object is part of the clone; otherwise the
	// copy still means the original.
	refFields = []ref{
		{[]string{"configMapRef", "name"}, "ConfigMap"},
		{[]string{"configMapKeyRef", "name"}, "ConfigMap"},
		{[]string{"configMap", "name"}, "ConfigMap"},
		{[]string{"persistentVolumeClaim", "claimName"}, "PersistentVolumeClaim"},
		{[]string{"serviceAccountName"}, "ServiceAccount"},
		{[]string{"serviceAccount"}, "ServiceAccount"},
		{[]string{"backend", "service", "name"}, "Service"},
		{[]string{"serviceName"}, "Service"},
		{[]string{"roleRef", "name"}, "Role ClusterRole"},
		{[]string{"subjects", "name"}, "ServiceAccount"},
		{[]string{"templateRef", "name"}, "WorkflowTemplate ClusterWorkflowTemplate"},
		{[]string{"workflowTemplateRef", "name"}, "WorkflowTemplate ClusterWorkflowTemplate"},
		{[]string{"scaleTargetRef", "name"}, ""},
	}
	// secretFields name Secrets: a copy usually needs its own, or
	// deliberately shares the old one, so they are never renamed
	// silently.
	secretFields = [][]string{
		{"secretKeyRef", "name"}, {"secretRef", "name"}, {"secret", "secretName"},
		{"imagePullSecrets", "name"}, {"tls", "secretName"}, {"existingSecret"},
	}
	// reviewFields usually differ in a copy, whatever they contain.
	reviewFields = [][]string{
		{"image"}, {"host"}, {"hosts"}, {"repoURL"}, {"url"},
	}
	// Keys under these maps are other tools' names (label keys such as
	// app.kubernetes.io/name), never the object's.
	fixedKeys = []string{"labels", "annotations", "matchLabels", "nodeSelector", "selector"}
)

type ref struct {
	path  []string
	kinds string // space-separated; "" = any kind
}

// reAddress finds addresses of things outside the files: URLs and
// host/path references such as registry.example.com/team/app.
var reAddress = regexp.MustCompile(`://|(^|[^\w.-])[\w-]+(\.[\w-]+)+(:\d+)?/`)

type cloner struct {
	src     []byte
	opts    CloneOptions
	targets map[string]bool // metadata.name of the documents cloned
	flip    map[string]bool
	edits   []edit
	changes []Change
	at      []int // original offset of each change
}

type edit struct {
	start, end int
	text       string
}

// Clone copies a document (or a whole file) under a new name: every
// whole-word use of the old name is renamed, keeping comments and
// layout. References to objects outside the copy, Secrets and images
// are suggestions, since the copy often still means the original.
func Clone(src []byte, opts CloneOptions) (*CloneResult, error) {
	docs, err := pick(src, opts.Doc)
	if err != nil {
		return nil, err
	}

	c := &cloner{src: src, opts: opts, targets: map[string]bool{}, flip: map[string]bool{}}
	for _, id := range opts.Flip {
		c.flip[id] = true
	}
	for _, d := range docs {
		if name := plainName(d.Root.Get("metadata").Get("name")); name != "" {
			c.targets[d.Root.Get("kind").Str()+"/"+name] = true
		}
	}
	if c.opts.From == "" {
		c.opts.From = defaultName(docs)
	}
	c.opts.From, c.opts.To = strings.TrimSpace(c.opts.From), strings.TrimSpace(c.opts.To)
	switch {
	case c.opts.From == "":
		return nil, errors.New("no name found (metadata.name or name); say which name to replace")
	case c.opts.To == "":
		return nil, errors.New("give the new name")
	case c.opts.To == c.opts.From:
		return nil, errors.New("the new name is the same as the old one")
	}

	for _, d := range docs {
		c.walk(d.Root, nil)
	}

	// The text: the documents cloned, with the edits applied.
	start, end := docs[0].Range.Start.Offset, docs[len(docs)-1].Range.End.Offset
	if opts.Doc < 0 {
		start, end = 0, len(src)
	}
	start = lineStart(src, start)
	if opts.Doc >= 0 && bytes.HasPrefix(src[start:], []byte("---")) {
		// One document of several: leave its separator behind.
		if nl := bytes.IndexByte(src[start:end], '\n'); nl >= 0 {
			start += nl + 1
		}
	}
	var b strings.Builder
	pos, shift := start, 0
	offsets := map[int]int{} // original offset → offset in the result
	slices.SortFunc(c.edits, func(a, b edit) int { return a.start - b.start })
	ei := 0
	order := slices.Clone(c.at)
	slices.Sort(order)
	for _, o := range order {
		for ei < len(c.edits) && c.edits[ei].start < o {
			shift += len(c.edits[ei].text) - (c.edits[ei].end - c.edits[ei].start)
			ei++
		}
		offsets[o] = o - start + shift
	}
	for _, e := range c.edits {
		b.Write(src[pos:e.start])
		b.WriteString(e.text)
		pos = e.end
	}
	b.Write(src[pos:end])
	text := b.String()
	if !strings.HasSuffix(text, "\n") {
		text += "\n"
	}
	for i := range c.changes {
		c.changes[i].Line = 1 + strings.Count(text[:min(offsets[c.at[i]], len(text))], "\n")
	}
	if c.changes == nil {
		c.changes = []Change{}
	}
	return &CloneResult{From: c.opts.From, To: c.opts.To, Text: text, Changes: c.changes}, nil
}

// lineStart moves back to the start of the line, so the first line's
// indentation (and a leading "- " of a list item) is kept.
func lineStart(src []byte, off int) int {
	for off > 0 && src[off-1] != '\n' {
		off--
	}
	return off
}

// walk visits every key and value below n. path holds the keys to n,
// with "[i]" for list items.
func (c *cloner) walk(n *yamlkit.Node, path []string) {
	if n == nil {
		return
	}
	switch n.Kind {
	case yamlkit.KindMap:
		fixed := len(path) > 0 && slices.Contains(fixedKeys, path[len(path)-1])
		for _, p := range n.Pairs {
			if p.Key == nil {
				continue
			}
			sub := append(slices.Clone(path), p.Key.Value)
			if !fixed && !strings.Contains(p.Key.Value, "/") {
				c.scalar(p.Key, sub, true)
			}
			if len(path) == 0 && (p.Key.Value == "apiVersion" || p.Key.Value == "kind") {
				continue
			}
			c.walk(p.Value, sub)
		}
	case yamlkit.KindSeq:
		for i, it := range n.Items {
			c.walk(it, append(slices.Clone(path), "["+strconv.Itoa(i)+"]"))
		}
	case yamlkit.KindScalar:
		c.scalar(n, path, false)
	}
}

// scalar decides what to do with one key or value.
func (c *cloner) scalar(n *yamlkit.Node, path []string, isKey bool) {
	raw := c.src[n.Range.Start.Offset:n.Range.End.Offset]
	hits := wordHits(string(raw), c.opts.From)
	keys := keysOf(path)
	secret, review := !isKey && matches(keys, secretFields), !isKey && matches(keys, reviewFields)

	if len(hits) == 0 {
		// Nothing to rename, but copies usually change these.
		if (secret || review) && n.Value != "" && !n.Templated {
			reason := "Copies usually use their own image or host"
			if secret {
				reason = "Check whether the copy should share this Secret"
			}
			c.add(n, Change{Kind: "check", Path: format(path), Before: n.Value, Reason: reason})
		}
		return
	}

	ch := Change{Kind: "rename", Path: format(path), Key: isKey, Before: n.Value, After: replaceWords(n.Value, c.opts.From, c.opts.To)}
	switch {
	case secret:
		ch.Kind, ch.Reason = "suggest", "A Secret: the copy may share it or need its own"
	case review:
		ch.Kind, ch.Reason = "suggest", "Check that the renamed value exists"
	case !isKey && reAddress.MatchString(n.Value):
		ch.Kind, ch.Reason = "suggest", "An address of something outside these files"
	case !isKey && !c.inClone(keys, n.Value):
		ch.Kind, ch.Reason = "suggest", "Refers to "+n.Value+", which is not part of the copy"
	}
	ch.Applied = ch.Kind == "rename"
	if !c.add(n, ch) {
		return
	}
	for _, h := range hits {
		start := n.Range.Start.Offset + h
		c.edits = append(c.edits, edit{start, start + len(c.opts.From), c.opts.To})
	}
}

// inClone reports whether a value is fine to rename as a reference: the
// field doesn't name another object, or the object is part of the copy.
func (c *cloner) inClone(keys []string, value string) bool {
	for _, r := range refFields {
		if len(keys) < len(r.path) || !slices.Equal(keys[len(keys)-len(r.path):], r.path) {
			continue
		}
		for target := range c.targets {
			kind, name, _ := strings.Cut(target, "/")
			if name == value && (r.kinds == "" || slices.Contains(strings.Fields(r.kinds), kind)) {
				return true
			}
		}
		return false
	}
	return true
}

// baseName picks the name the others are built from: the one found (as
// a whole word) in most names, the shortest on a tie. For a Deployment
// "shop" with ConfigMap "shop-config" it is "shop".
func baseName(names []string) string {
	best, score := "", 0
	for _, n := range names {
		s := 0
		for _, m := range names {
			if len(wordHits(m, n)) > 0 {
				s++
			}
		}
		if s > score || s == score && len(n) < len(best) {
			best, score = n, s
		}
	}
	return best
}

// add records a change, applying the user's flip, and reports whether
// it is applied.
func (c *cloner) add(n *yamlkit.Node, ch Change) bool {
	ch.ID = strconv.Itoa(n.Range.Start.Offset)
	if ch.Kind != "check" && c.flip[ch.ID] {
		ch.Applied = !ch.Applied
	}
	c.changes = append(c.changes, ch)
	c.at = append(c.at, n.Range.Start.Offset)
	return ch.Applied
}

// wordHits finds whole-word uses of name in s: the characters around
// it are not letters or digits, so "api" matches "api-svc" and
// "{{steps.api.outputs}}" but not "rapid" or "apis".
func wordHits(s, name string) []int {
	var out []int
	for i := 0; ; {
		j := strings.Index(s[i:], name)
		if j < 0 {
			return out
		}
		at := i + j
		end := at + len(name)
		if (at == 0 || !alnum(s[at-1])) && (end == len(s) || !alnum(s[end])) {
			out = append(out, at)
		}
		i = at + 1
	}
}

func replaceWords(s, from, to string) string {
	hits := wordHits(s, from)
	for i := len(hits) - 1; i >= 0; i-- {
		s = s[:hits[i]] + to + s[hits[i]+len(from):]
	}
	return s
}

func alnum(b byte) bool {
	return b >= 'a' && b <= 'z' || b >= 'A' && b <= 'Z' || b >= '0' && b <= '9'
}

func keysOf(path []string) []string {
	return slices.DeleteFunc(slices.Clone(path), func(k string) bool { return strings.HasPrefix(k, "[") })
}

// matches reports whether keys ends with one of the field paths.
func matches(keys []string, fields [][]string) bool {
	for _, f := range fields {
		if len(keys) >= len(f) && slices.Equal(keys[len(keys)-len(f):], f) {
			return true
		}
	}
	return false
}

func format(path []string) string {
	var b strings.Builder
	for _, k := range path {
		if !strings.HasPrefix(k, "[") && b.Len() > 0 {
			b.WriteByte('.')
		}
		b.WriteString(k)
	}
	return b.String()
}

// OldName is the name Clone replaces when none is given: "" if the
// documents have none (or don't parse).
func OldName(src []byte, doc int) string {
	docs, err := pick(src, doc)
	if err != nil {
		return ""
	}
	return defaultName(docs)
}

// pick parses src and returns the documents to clone.
func pick(src []byte, doc int) ([]*yamlkit.Document, error) {
	f := yamlkit.Parse(src)
	if len(f.Docs) == 0 {
		return nil, errors.New("the file is empty")
	}
	for _, d := range f.Diagnostics {
		if d.Severity == yamlkit.SeverityError {
			// Edits go by position: a broken tree would misplace them.
			return nil, fmt.Errorf("fix the YAML first (line %d: %s)", d.Range.Start.Line, d.Message)
		}
	}
	switch {
	case doc < 0:
		return f.Docs, nil
	case doc < len(f.Docs):
		return []*yamlkit.Document{f.Docs[doc]}, nil
	}
	return nil, fmt.Errorf("the file has %d documents", len(f.Docs))
}

// defaultName is the base of the documents' metadata.names, else the
// first document's top-level name.
func defaultName(docs []*yamlkit.Document) string {
	var names []string
	for _, d := range docs {
		if name := plainName(d.Root.Get("metadata").Get("name")); name != "" {
			names = append(names, name)
		}
	}
	return cmp.Or(baseName(names), plainName(docs[0].Root.Get("name")))
}

// plainName is a name written out; a template expression (Helm's
// {{ include … }}) is only known after rendering.
func plainName(n *yamlkit.Node) string {
	if n == nil || n.Templated {
		return ""
	}
	return n.Str()
}
