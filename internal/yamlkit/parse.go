package yamlkit

import (
	"bytes"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"unicode/utf8"

	goyaml "github.com/goccy/go-yaml"
	"github.com/goccy/go-yaml/ast"
	"github.com/goccy/go-yaml/parser"
	"github.com/goccy/go-yaml/token"
)

// Parse reads YAML content — one or many documents, possibly full of
// template expressions — and never fails: problems are reported as
// Diagnostics, and every document that can be parsed still is.
func Parse(content []byte) *File {
	src, hasBOM := stripBOM(content)
	f := &File{
		Docs:        []*Document{},
		Diagnostics: []Diagnostic{},
		Expressions: []Expression{},
		BOM:         hasBOM,
		CRLF:        bytes.Contains(src, []byte("\r\n")),
	}
	li := newLineIndex(src)
	spans := findExpressions(src)
	for _, s := range spans {
		f.Expressions = append(f.Expressions, Expression{
			Range:      li.span(s.start, s.end),
			Text:       string(src[s.start:s.end]),
			Syntax:     s.syntax,
			Phase:      s.phase,
			Standalone: s.standalone,
		})
	}

	p := &parseState{src: src, masked: mask(src, spans), li: li, spans: spans, f: f}
	for _, c := range p.splitDocuments() {
		p.parseChunk(c)
	}
	return f
}

type parseState struct {
	src    []byte // content without BOM
	masked []byte // src with expressions masked; same length and lines
	li     *lineIndex
	spans  []exprSpan
	f      *File
}

// chunk is the byte range of one document within the source.
type chunk struct {
	start, end int
	line       int // 1-based line where the chunk starts
}

// splitDocuments cuts the source at "---" / "..." marker lines. Doing
// this ourselves (instead of letting the parser read the whole stream)
// means a syntax error in one document can't hide the others.
func (p *parseState) splitDocuments() []chunk {
	var chunks []chunk
	cur := chunk{start: 0, line: 1}
	for n := 1; n <= p.li.lines(); n++ {
		text := p.lineIn(p.masked, n)
		start := p.li.starts[n-1]
		switch {
		case isMarker(text, "---"):
			// The marker line opens the next document (it may carry
			// content, e.g. "--- |"), so it stays with the new chunk.
			cur.end = start
			chunks = append(chunks, cur)
			cur = chunk{start: start, line: n}
		case isMarker(text, "..."):
			end := len(p.src)
			if n < p.li.lines() {
				end = p.li.starts[n]
			}
			cur.end = end
			chunks = append(chunks, cur)
			cur = chunk{start: end, line: n + 1}
		}
	}
	cur.end = len(p.src)
	chunks = append(chunks, cur)

	// Drop chunks with nothing but blank lines, comments, directives
	// and markers — they are not documents.
	var docs []chunk
	for _, c := range chunks {
		if hasContent(p.masked[c.start:c.end]) {
			docs = append(docs, c)
		}
	}
	return docs
}

func isMarker(line, marker string) bool {
	if !strings.HasPrefix(line, marker) {
		return false
	}
	rest := line[len(marker):]
	return rest == "" || rest[0] == ' ' || rest[0] == '\t'
}

func hasContent(b []byte) bool {
	for _, raw := range bytes.Split(b, []byte("\n")) {
		line := strings.TrimSpace(string(raw))
		if line == "" || line[0] == '#' || line[0] == '%' || line == "---" || line == "..." {
			continue
		}
		return true
	}
	return false
}

// lineIn returns line n of buf, which must share src's line layout.
func (p *parseState) lineIn(buf []byte, n int) string {
	if n < 1 || n > p.li.lines() {
		return ""
	}
	start := p.li.starts[n-1]
	end := len(buf)
	if n < p.li.lines() {
		end = p.li.starts[n] - 1
	}
	return strings.TrimSuffix(string(buf[start:end]), "\r")
}

func (p *parseState) parseChunk(c chunk) {
	doc := &Document{Index: len(p.f.Docs), Range: p.li.span(c.start, trimEnd(p.src, c.start, c.end))}
	p.f.Docs = append(p.f.Docs, doc)

	// Duplicate keys are ours to report (with a friendlier message and
	// without aborting the parse), so the parser is told to allow them.
	// goccy miscounts lines after comments in CRLF files, so it gets LF
	// text. Lines and columns are unaffected (\r only ends lines), and
	// nothing below relies on goccy's byte offsets.
	text := bytes.ReplaceAll(p.masked[c.start:c.end], []byte("\r\n"), []byte("\n"))
	tree, err := parser.ParseBytes(text, parser.ParseComments, parser.AllowDuplicateMapKey())
	if err != nil {
		p.f.Diagnostics = append(p.f.Diagnostics, p.syntaxError(err, c))
		doc.Name = guessName(p.masked[c.start:c.end])
		return
	}

	cv := &converter{p: p, lineOff: c.line - 1}
	for _, d := range tree.Docs {
		if d == nil || d.Body == nil {
			continue
		}
		if root := cv.node(d.Body); root != nil {
			doc.Root = root
			break
		}
	}
	doc.Name = docName(doc.Root)

	// Template control flow ({{- if }} … {{- else }} …) legitimately
	// produces the same key twice in the source; only flag duplicates
	// in documents without it.
	if !p.hasStandaloneTemplates(c) {
		p.checkDuplicates(doc.Root)
	}
}

func trimEnd(b []byte, start, end int) int {
	for end > start && (b[end-1] == '\n' || b[end-1] == '\r' || b[end-1] == ' ' || b[end-1] == '\t') {
		end--
	}
	return end
}

func (p *parseState) hasStandaloneTemplates(c chunk) bool {
	for _, s := range p.spans {
		if s.start >= c.start && s.end <= c.end && s.standalone && s.phase == PhaseTemplate {
			return true
		}
	}
	return false
}

func (p *parseState) overlapsExpression(start, end int) bool {
	for _, s := range p.spans {
		if s.start < end && start < s.end {
			return true
		}
	}
	return false
}

func (p *parseState) syntaxError(err error, c chunk) Diagnostic {
	line, col, msg := c.line, 1, err.Error()
	var se *goyaml.SyntaxError
	if errors.As(err, &se) {
		msg = se.GetMessage()
		if t := se.GetToken(); t != nil {
			line, col = max(1, t.Position.Line)+c.line-1, max(1, t.Position.Column)
		}
	}
	d := p.friendly(line, col, msg)
	if p.hasStandaloneTemplates(c) {
		d.Hint = strings.TrimSpace(d.Hint + " This document contains template logic, which can also cause this.")
	}
	return d
}

// docName labels a document "Kind/name" when it looks like a
// Kubernetes-style object, "Kind" when only the kind is known, or "".
func docName(root *Node) string {
	kind := root.Get("kind").Str()
	name := ""
	// A templated name ({{ include … }}) is noise in a label; show the kind.
	if n := root.Get("metadata").Get("name"); n != nil && !n.Templated {
		name = n.Str()
	}
	switch {
	case kind != "" && name != "":
		return kind + "/" + name
	default:
		return kind
	}
}

var (
	kindLine = regexp.MustCompile(`(?m)^kind:[ \t]*([\w.-]+)`)
	nameLine = regexp.MustCompile(`(?m)^[ \t]+name:[ \t]*([\w.-]+)`)
)

// guessName names a document that failed to parse, from its text.
func guessName(b []byte) string {
	k := kindLine.FindSubmatch(b)
	if k == nil {
		return ""
	}
	if n := nameLine.FindSubmatch(b); n != nil {
		return string(k[1]) + "/" + string(n[1])
	}
	return string(k[1])
}

// checkDuplicates reports keys that appear twice in the same map. Most
// tools keep only the last value without a word, which hides bugs.
func (p *parseState) checkDuplicates(n *Node) {
	if n == nil {
		return
	}
	switch n.Kind {
	case KindMap:
		seen := map[string]*Node{}
		for _, pr := range n.Pairs {
			if k := pr.Key; k != nil && k.Kind == KindScalar && k.Tag != TagMerge {
				if first, dup := seen[k.Value]; dup {
					p.f.Diagnostics = append(p.f.Diagnostics, Diagnostic{
						Severity: SeverityError,
						Code:     "duplicate-key",
						Message:  fmt.Sprintf("Duplicate key %q (first defined on line %d)", k.Value, first.Range.Start.Line),
						Hint:     "Most tools silently keep only the last value. Remove or rename one of them.",
						Range:    k.Range,
					})
				} else {
					seen[k.Value] = k
				}
			}
			p.checkDuplicates(pr.Value)
		}
	case KindSeq:
		for _, it := range n.Items {
			p.checkDuplicates(it)
		}
	}
}

// converter turns goccy's AST into Nodes with absolute positions.
type converter struct {
	p       *parseState
	lineOff int // lines before the chunk being converted
}

func (c *converter) pos(t *token.Token) Pos {
	return c.p.li.at(t.Position.Line+c.lineOff, max(1, t.Position.Column))
}

// tokenEnd is where a token's text ends. Origin holds the raw source
// text plus surrounding whitespace, so the trimmed text, laid out from
// the token's start, gives its extent.
func (c *converter) tokenEnd(t *token.Token) Pos {
	return c.endAfter(c.pos(t), strings.TrimSpace(t.Origin))
}

// endAfter returns where text ends when it starts at start. It works in
// lines and columns rather than bytes, so the \r removed from CRLF files
// before parsing doesn't skew multi-line values.
func (c *converter) endAfter(start Pos, text string) Pos {
	nl := strings.Count(text, "\n")
	if nl == 0 {
		return c.p.li.at(start.Line, start.Col+utf8.RuneCountInString(text))
	}
	last := strings.TrimSuffix(text[strings.LastIndexByte(text, '\n')+1:], "\r")
	return c.p.li.at(start.Line+nl, utf8.RuneCountInString(last)+1)
}

func (c *converter) node(n ast.Node) *Node {
	switch v := n.(type) {
	case nil:
		return nil
	case *ast.DocumentNode:
		return c.node(v.Body)
	case *ast.MappingNode:
		return c.mapping(v)
	case *ast.MappingValueNode:
		// A lone key/value may appear without its MappingNode wrapper.
		return c.mapping(&ast.MappingNode{Values: []*ast.MappingValueNode{v}})
	case *ast.SequenceNode:
		return c.sequence(v)
	case *ast.MappingKeyNode: // explicit "? key"
		return c.node(v.Value)
	case *ast.AnchorNode:
		inner := c.node(v.Value)
		if inner == nil {
			inner = c.implicitNull(v.Start)
		}
		if v.Name != nil && v.Name.GetToken() != nil {
			inner.Anchor = v.Name.GetToken().Value
		}
		inner.Range.Start = c.pos(v.Start)
		return inner
	case *ast.AliasNode:
		name := v.Value.GetToken()
		return &Node{Kind: KindAlias, Value: name.Value, Range: Range{Start: c.pos(v.Start), End: c.tokenEnd(name)}}
	case *ast.TagNode:
		inner := c.node(v.Value)
		if inner == nil {
			inner = c.implicitNull(v.Start)
		}
		inner.Tag = v.Start.Value
		inner.Range.Start = c.pos(v.Start)
		return inner
	case *ast.LiteralNode:
		return c.literal(v)
	case *ast.MergeKeyNode:
		return &Node{Kind: KindScalar, Tag: TagMerge, Value: "<<", Style: StylePlain,
			Range: Range{Start: c.pos(v.Token), End: c.tokenEnd(v.Token)}}
	case *ast.NullNode:
		if v.Token == nil || v.Token.Type == token.ImplicitNullType {
			return c.implicitNull(v.Token)
		}
		return c.scalar(v.Token)
	case *ast.CommentGroupNode, *ast.CommentNode:
		return nil
	default:
		if t := n.GetToken(); t != nil {
			return c.scalar(t)
		}
		return nil
	}
}

func (c *converter) mapping(m *ast.MappingNode) *Node {
	out := &Node{Kind: KindMap, Flow: m.IsFlowStyle, Pairs: []*Pair{}}
	for _, mv := range m.Values {
		key := c.node(mv.Key)
		if key == nil {
			continue
		}
		val := c.node(mv.Value)
		if val == nil {
			// "key:" with nothing after it: an empty value just past the colon.
			val = c.implicitNull(mv.Start)
		}
		out.Pairs = append(out.Pairs, &Pair{Key: key, Value: val})
	}
	switch {
	case m.IsFlowStyle && m.Start != nil && m.End != nil:
		out.Range = Range{Start: c.pos(m.Start), End: c.tokenEnd(m.End)}
	case len(out.Pairs) > 0:
		out.Range = Range{Start: out.Pairs[0].Key.Range.Start, End: out.Pairs[len(out.Pairs)-1].Value.Range.End}
		if last := out.Pairs[len(out.Pairs)-1]; last.Value.Range.End.Before(last.Key.Range.End) {
			out.Range.End = last.Key.Range.End
		}
	}
	return out
}

func (c *converter) sequence(s *ast.SequenceNode) *Node {
	out := &Node{Kind: KindSeq, Flow: s.IsFlowStyle, Items: []*Node{}}
	for i, v := range s.Values {
		item := c.node(v)
		if item == nil {
			// "- " with nothing after the dash.
			var at *token.Token
			if i < len(s.Entries) && s.Entries[i] != nil {
				at = s.Entries[i].Start
			}
			item = c.implicitNull(at)
		}
		out.Items = append(out.Items, item)
	}
	switch {
	case s.IsFlowStyle && s.Start != nil && s.End != nil:
		out.Range = Range{Start: c.pos(s.Start), End: c.tokenEnd(s.End)}
	case len(out.Items) > 0:
		start := out.Items[0].Range.Start
		if s.Start != nil {
			start = c.pos(s.Start) // the first "-"
		}
		out.Range = Range{Start: start, End: out.Items[len(out.Items)-1].Range.End}
	}
	return out
}

// implicitNull is an empty value, placed as a zero-width range right
// after the token it follows (a ':' or '-').
func (c *converter) implicitNull(after *token.Token) *Node {
	var at Pos
	if after != nil {
		at = c.pos(after)
		if after.Type != token.ImplicitNullType {
			at = c.tokenEnd(after)
		}
	}
	return &Node{Kind: KindScalar, Tag: TagNull, Style: StylePlain, Range: Range{Start: at, End: at}}
}

// scalarStart corrects goccy's start column for a scalar: after a plain
// scalar with trailing spaces ("no_log: true    ") it reports a column
// shifted right by their count. So the token's first line of text is
// looked up on its line, taking the match at or just before that column.
func (c *converter) scalarStart(t *token.Token) Pos {
	p := c.pos(t)
	text := strings.TrimSpace(t.Origin)
	if i := strings.IndexByte(text, '\n'); i >= 0 {
		text = strings.TrimRight(text[:i], " \t\r")
	}
	line := c.p.lineIn(c.p.masked, p.Line)
	if text == "" || !strings.Contains(line, text) {
		return p
	}
	best := -1
	for from := 0; ; {
		i := strings.Index(line[from:], text)
		if i < 0 {
			break
		}
		col := utf8.RuneCountInString(line[:from+i]) + 1
		if col > p.Col && best >= 0 {
			break
		}
		best = col
		if col >= p.Col {
			break
		}
		from += i + 1
	}
	return c.p.li.at(p.Line, best)
}

func (c *converter) scalar(t *token.Token) *Node {
	start := c.scalarStart(t)
	end := c.endAfter(start, strings.TrimSpace(t.Origin))
	n := &Node{Kind: KindScalar, Value: t.Value, Style: StylePlain, Range: Range{Start: start, End: end}}
	switch t.Type {
	case token.SingleQuoteType:
		n.Style, n.Tag = StyleSingle, TagStr
	case token.DoubleQuoteType:
		n.Style, n.Tag = StyleDouble, TagStr
	default:
		n.Tag = resolvePlain(t.Value)
	}
	c.restoreTemplated(n)
	return n
}

func (c *converter) literal(l *ast.LiteralNode) *Node {
	n := &Node{Kind: KindScalar, Tag: TagStr, Style: StyleLiteral}
	if l.Value == nil {
		n.Range = Range{Start: c.pos(l.Start), End: c.tokenEnd(l.Start)}
		return n
	}
	n.Value = l.Value.Value
	if strings.HasPrefix(l.Start.Value, ">") {
		n.Style = StyleFolded
	}
	n.Range.Start = c.pos(l.Start)
	n.Range.End = c.tokenEnd(l.Start)
	// The content token starts at the beginning of the line after the
	// indicator, and its Origin holds the raw content lines.
	if content := strings.TrimRight(l.Value.GetToken().Origin, " \t\r\n"); content != "" {
		if line := c.pos(l.Start).Line + 1; line <= c.p.li.lines() {
			n.Range.End = c.endAfter(Pos{Line: line, Col: 1}, content)
		}
	}
	c.restoreTemplated(n)
	return n
}

// restoreTemplated marks scalars that contained masked expressions and
// puts the original text back into Value, so users see
// "{{ .Values.image }}" rather than the parser's placeholder.
func (c *converter) restoreTemplated(n *Node) {
	start, end := n.Range.Start.Offset, n.Range.End.Offset
	if end <= start || !c.p.overlapsExpression(start, end) {
		return
	}
	n.Templated = true
	n.Tag = TagStr
	raw := string(c.p.src[start:end])
	switch n.Style {
	case StyleSingle:
		n.Value = strings.ReplaceAll(strings.TrimSuffix(strings.TrimPrefix(raw, "'"), "'"), "''", "'")
	case StyleDouble:
		n.Value = unescapeDouble(strings.TrimSuffix(strings.TrimPrefix(raw, `"`), `"`))
	case StyleLiteral, StyleFolded:
		n.Value = blockContent(raw)
	default:
		n.Value = foldPlain(raw)
	}
}

// foldPlain joins a multi-line plain scalar the way YAML does: line
// breaks and the indentation after them become single spaces.
func foldPlain(raw string) string {
	lines := strings.Split(strings.ReplaceAll(raw, "\r\n", "\n"), "\n")
	for i := range lines {
		lines[i] = strings.TrimSpace(lines[i])
	}
	return strings.Join(lines, " ")
}

func unescapeDouble(s string) string {
	return strings.NewReplacer(`\"`, `"`, `\\`, `\`, `\n`, "\n", `\t`, "\t").Replace(s)
}

// blockContent extracts a literal/folded block's text from its raw
// source (indicator line included), removing the block's indentation.
func blockContent(raw string) string {
	lines := strings.Split(strings.ReplaceAll(raw, "\r\n", "\n"), "\n")
	if len(lines) < 2 {
		return ""
	}
	body := lines[1:]
	indent := -1
	for _, l := range body {
		if strings.TrimSpace(l) == "" {
			continue
		}
		if n := len(l) - len(strings.TrimLeft(l, " ")); indent < 0 || n < indent {
			indent = n
		}
	}
	for i, l := range body {
		if len(l) >= indent && indent > 0 {
			body[i] = l[indent:]
		} else {
			body[i] = strings.TrimLeft(l, " ")
		}
	}
	return strings.Join(body, "\n") + "\n"
}

var (
	intRe   = regexp.MustCompile(`^([-+]?[0-9]+|0o[0-7]+|0x[0-9a-fA-F]+)$`)
	floatRe = regexp.MustCompile(`^([-+]?(\.[0-9]+|[0-9]+(\.[0-9]*)?)([eE][-+]?[0-9]+)?|[-+]?\.(inf|Inf|INF)|\.(nan|NaN|NAN))$`)
)

// resolvePlain assigns a YAML 1.2 core-schema tag to an unquoted scalar.
func resolvePlain(v string) string {
	switch v {
	case "", "~", "null", "Null", "NULL":
		return TagNull
	case "true", "True", "TRUE", "false", "False", "FALSE":
		return TagBool
	}
	if intRe.MatchString(v) {
		return TagInt
	}
	if floatRe.MatchString(v) {
		return TagFloat
	}
	return TagStr
}
