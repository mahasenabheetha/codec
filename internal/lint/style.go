package lint

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/mahasenabheetha/codec/v2/internal/yamlkit"
)

func (c *checker) style() {
	c.textRules()
	for _, d := range c.in.YAML.Docs {
		if d.Root == nil {
			continue
		}
		c.scalars(d.Root)
		c.indent(d.Root)
	}
}

// --- rules on raw lines ---

func (c *checker) textRules() {
	max := c.cfg.LineLength
	if max <= 0 {
		max = 120
	}
	checkLength := c.enabled("line-length")
	checkTrailing := c.enabled("trailing-spaces")
	first := true // document-start: looking for the first content line
	for n := 1; n <= c.text.count(); n++ {
		line := c.text.line(n)
		if first && c.enabled("document-start") {
			t := strings.TrimSpace(line)
			if t != "" && !strings.HasPrefix(t, "#") {
				first = false
				if !strings.HasPrefix(t, "---") && !strings.HasPrefix(t, "%") {
					c.report("document-start", c.text.span(n, 1, utf8.RuneCountInString(line)+1),
						"The file doesn't start with ---", "Add --- as the first line.")
				}
			}
		}
		if checkTrailing {
			if trimmed := strings.TrimRight(line, " \t"); len(trimmed) < len(line) {
				from := utf8.RuneCountInString(trimmed) + 1
				c.report("trailing-spaces", c.text.span(n, from, utf8.RuneCountInString(line)+1),
					"Trailing whitespace", "Delete the spaces at the end of the line.")
			}
		}
		if checkLength {
			if l := utf8.RuneCountInString(line); l > max && !unbreakable(line) {
				c.report("line-length", c.text.span(n, max+1, l+1),
					fmt.Sprintf("Line is %d characters long (limit %d)", l, max),
					"Split the value, e.g. with a folded block scalar (>-).")
			}
		}
	}
}

// unbreakable reports whether a long line is one long word (a URL, a
// hash) after its indentation, "- " and key: nothing to split.
func unbreakable(line string) bool {
	s := strings.TrimLeft(line, " ")
	s = strings.TrimPrefix(s, "- ")
	if i := strings.Index(s, ": "); i >= 0 {
		s = s[i+2:]
	}
	s = strings.Trim(strings.TrimSpace(s), `"'`)
	return !strings.ContainsAny(s, " \t")
}

// --- rules on scalar values ---

var (
	truthyWords = map[string]bool{"yes": true, "no": true, "on": true, "off": true, "y": true, "n": true}
	reOctal     = regexp.MustCompile(`^[-+]?0[0-9]+$`)
	reOctal12   = regexp.MustCompile(`^0o[0-7]+$`)
)

// scalars walks values (never keys: GitHub Actions' "on:" is a key and
// fine) for the YAML 1.1 traps and empty values.
func (c *checker) scalars(n *yamlkit.Node) {
	switch n.Kind {
	case yamlkit.KindMap:
		for _, p := range n.Pairs {
			if p.Value == nil {
				continue
			}
			c.emptyValue(p)
			c.scalars(p.Value)
		}
	case yamlkit.KindSeq:
		for _, it := range n.Items {
			if it != nil {
				c.scalars(it)
			}
		}
	case yamlkit.KindScalar:
		if n.Style != yamlkit.StylePlain || n.Templated {
			return
		}
		c.truthy(n)
		c.octal(n)
	}
}

func (c *checker) truthy(n *yamlkit.Node) {
	lower := strings.ToLower(n.Value)
	if !truthyWords[lower] {
		return
	}
	b := lower == "yes" || lower == "on" || lower == "y"
	c.report("truthy", n.Range,
		fmt.Sprintf("%q is text in YAML 1.2 but %t in YAML 1.1", n.Value, b),
		fmt.Sprintf("Write %t, or quote it (%q) if you mean the word.", b, n.Value))
}

func (c *checker) octal(n *yamlkit.Node) {
	v := n.Value
	switch {
	case reOctal12.MatchString(v):
		c.report("octal", n.Range,
			fmt.Sprintf("%s is an octal number in YAML 1.2 but text in YAML 1.1", v),
			"Quote it if it is text, or write the decimal value.")
	case reOctal.MatchString(v):
		digits := strings.TrimLeft(v, "+-")
		if o, err := strconv.ParseInt(digits, 8, 64); err == nil {
			c.report("octal", n.Range,
				fmt.Sprintf("%s is octal (%d) in YAML 1.1 but decimal in YAML 1.2", v, o),
				fmt.Sprintf("Quote it (%q) if it is text such as a file mode, or drop the leading zero.", v))
		} else {
			c.report("octal", n.Range,
				fmt.Sprintf("%s is text in YAML 1.1 (8 and 9 aren't octal digits) but a number in YAML 1.2", v),
				fmt.Sprintf("Quote it (%q) if it is text, or drop the leading zero.", v))
		}
	}
}

// emptyValue flags "key:" with nothing after it. Templates are
// skipped: there an empty key is usually filled by the next line's
// {{ toYaml … | nindent }}.
func (c *checker) emptyValue(p *yamlkit.Pair) {
	v := p.Value
	if v.Kind != yamlkit.KindScalar || v.Tag != yamlkit.TagNull || v.Value != "" || p.Key == nil {
		return
	}
	if c.in.YAML.HasTemplates() {
		return
	}
	c.report("empty-value", p.Key.Range,
		fmt.Sprintf("%q has no value, so it is null", p.Key.Value),
		"Add the value, or write {} , [] or null to make an empty value explicit.")
}

// --- indentation consistency ---

// indent learns the file's usual nesting width and list style, then
// flags the lines that differ. Flow collections ({…}) don't count.
func (c *checker) indent(root *yamlkit.Node) {
	if !c.enabled("indent") {
		return
	}
	type step struct {
		width int
		at    *yamlkit.Node // first key of the nested map, or first item
	}
	var maps, seqs []step
	var walk func(n *yamlkit.Node)
	walk = func(n *yamlkit.Node) {
		switch n.Kind {
		case yamlkit.KindMap:
			for _, p := range n.Pairs {
				if p.Key == nil || p.Value == nil {
					continue
				}
				v := p.Value
				if !v.Flow && p.Key.Range.Start.Line < v.Range.Start.Line {
					switch {
					case v.Kind == yamlkit.KindMap && len(v.Pairs) > 0 && v.Pairs[0].Key != nil:
						maps = append(maps, step{v.Pairs[0].Key.Range.Start.Col - p.Key.Range.Start.Col, v.Pairs[0].Key})
					case v.Kind == yamlkit.KindSeq && len(v.Items) > 0:
						// The item's node starts after "- ", two columns right of the dash.
						seqs = append(seqs, step{v.Items[0].Range.Start.Col - 2 - p.Key.Range.Start.Col, v.Items[0]})
					}
				}
				walk(v)
			}
		case yamlkit.KindSeq:
			for _, it := range n.Items {
				if it != nil {
					walk(it)
				}
			}
		}
	}
	walk(root)

	mode := func(steps []step, valid func(int) bool) (int, bool) {
		count := map[int]int{}
		best, bestN := 0, 0
		for _, s := range steps {
			if !valid(s.width) {
				continue
			}
			count[s.width]++
			if count[s.width] > bestN || (count[s.width] == bestN && s.width < best) {
				best, bestN = s.width, count[s.width]
			}
		}
		return best, bestN > 0
	}
	if w, ok := mode(maps, func(w int) bool { return w > 0 }); ok {
		for _, s := range maps {
			if s.width > 0 && s.width != w {
				c.report("indent", s.at.Range,
					fmt.Sprintf("Indented by %d spaces; the rest of the file uses %d", s.width, w),
					fmt.Sprintf("Indent nested keys by %d spaces.", w))
			}
		}
	}
	// Lists either sit under their key ("key:\n- a") or are indented;
	// both are fine, mixing them isn't.
	if w, ok := mode(seqs, func(w int) bool { return w >= 0 }); ok {
		for _, s := range seqs {
			if s.width >= 0 && s.width != w {
				style := "at the key's column"
				if w > 0 {
					style = fmt.Sprintf("indented by %d", w)
				}
				c.report("indent", itemRange(c, s.at),
					"List items indented differently from the rest of the file",
					fmt.Sprintf("This file puts the - of list items %s.", style))
			}
		}
	}
}

// itemRange covers a list item's "- " marker on its first line.
func itemRange(c *checker, item *yamlkit.Node) yamlkit.Range {
	p := item.Range.Start
	if p.Col > 2 {
		return c.text.span(p.Line, p.Col-2, p.Col-1)
	}
	return yamlkit.Range{Start: p, End: p}
}
