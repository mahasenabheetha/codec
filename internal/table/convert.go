package table

import (
	"fmt"
	"strings"

	"github.com/mahasenabheetha/codec/v2/internal/jsonkit"
)

// Formats are what a table converts to.
var Formats = []string{"json", "jsonl", "yaml", "markdown", "csv", "tsv"}

// Convert writes t as format. typed gives JSON and YAML values by their
// column's type (numbers, booleans, null for an empty cell); otherwise
// every value is text. indent is JSON's ("" for one line).
func Convert(t *Table, format string, typed bool, indent string) (string, error) {
	switch format {
	case "json":
		return jsonkit.Encode(t.Value(typed), indent), nil
	case "jsonl":
		var b strings.Builder
		for _, it := range t.Value(typed).Items {
			b.WriteString(jsonkit.Encode(it, ""))
			b.WriteByte('\n')
		}
		return b.String(), nil
	case "yaml":
		return jsonkit.ToYAML(t.Value(typed))
	case "markdown":
		return t.Markdown(), nil
	case "csv":
		return t.Delimited(','), nil
	case "tsv":
		return t.Delimited('\t'), nil
	}
	return "", fmt.Errorf("unknown format %q (json, jsonl, yaml, markdown, csv or tsv)", format)
}

// Value is t as JSON: an array of objects keyed by column name, or of
// arrays when there was no header.
func (t *Table) Value(typed bool) *jsonkit.Value {
	out := &jsonkit.Value{Kind: jsonkit.Array, Items: make([]*jsonkit.Value, len(t.Rows))}
	for i, r := range t.Rows {
		row := &jsonkit.Value{Kind: jsonkit.Array}
		if t.Header {
			row.Kind = jsonkit.Object
			row.Members = make([]jsonkit.Member, len(r))
		} else {
			row.Items = make([]*jsonkit.Value, len(r))
		}
		for c, v := range r {
			cell := t.cell(c, v, typed)
			if t.Header {
				row.Members[c] = jsonkit.Member{Key: t.Columns[c].Name, Value: cell}
			} else {
				row.Items[c] = cell
			}
		}
		out.Items[i] = row
	}
	return out
}

// cell is one value as JSON, typed by its column.
func (t *Table) cell(c int, v string, typed bool) *jsonkit.Value {
	if !typed {
		return &jsonkit.Value{Kind: jsonkit.String, Str: v}
	}
	s := strings.TrimSpace(v)
	if s == "" {
		return &jsonkit.Value{Kind: jsonkit.Null}
	}
	switch t.Columns[c].Type {
	case "integer", "number":
		return &jsonkit.Value{Kind: jsonkit.Number, Num: s}
	case "boolean":
		return &jsonkit.Value{Kind: jsonkit.Bool, Bool: strings.EqualFold(s, "true")}
	}
	return &jsonkit.Value{Kind: jsonkit.String, Str: v}
}

// Markdown is t as a GitHub table; number columns are right-aligned.
func (t *Table) Markdown() string {
	if len(t.Columns) == 0 {
		return ""
	}
	esc := func(s string) string {
		s = strings.ReplaceAll(s, "|", `\|`)
		s = strings.ReplaceAll(s, "\r\n", "<br>")
		return strings.ReplaceAll(s, "\n", "<br>")
	}
	var b strings.Builder
	line := func(cells []string) {
		b.WriteByte('|')
		for _, c := range cells {
			b.WriteByte(' ')
			b.WriteString(esc(c))
			b.WriteString(" |")
		}
		b.WriteByte('\n')
	}
	names := make([]string, len(t.Columns))
	rule := make([]string, len(t.Columns))
	for i, c := range t.Columns {
		names[i] = c.Name
		rule[i] = "---"
		if c.Type == "integer" || c.Type == "number" {
			rule[i] = "--:"
		}
	}
	line(names)
	b.WriteByte('|')
	for _, r := range rule {
		b.WriteString(" " + r + " |")
	}
	b.WriteByte('\n')
	for _, r := range t.Rows {
		line(r)
	}
	return b.String()
}

// Delimited writes t with delim, the header row first when it had one.
// A field is quoted when it holds the delimiter, a quote or a line
// break; a row of one empty field is written "" so it isn't a blank
// line (which readers skip).
func (t *Table) Delimited(delim rune) string {
	var b strings.Builder
	row := func(cells []string) {
		if len(cells) == 1 && cells[0] == "" {
			b.WriteString(`""` + "\n")
			return
		}
		for i, c := range cells {
			if i > 0 {
				b.WriteRune(delim)
			}
			if strings.ContainsRune(c, delim) || strings.ContainsAny(c, "\"\r\n") {
				b.WriteByte('"')
				b.WriteString(strings.ReplaceAll(c, `"`, `""`))
				b.WriteByte('"')
			} else {
				b.WriteString(c)
			}
		}
		b.WriteByte('\n')
	}
	if t.Header {
		names := make([]string, len(t.Columns))
		for i, c := range t.Columns {
			names[i] = c.Name
		}
		row(names)
	}
	for _, r := range t.Rows {
		row(r)
	}
	return b.String()
}

// FromJSON reads JSON (or JSONC, JSON5, JSON Lines) into a table: an
// array of objects gives one column per key in the order first seen; an
// array of arrays gives rows without a header; one object is one row.
// Nested values are written as JSON, null as an empty cell.
func FromJSON(src string) (*Table, error) {
	vals, _, err := jsonkit.ParseAll(src)
	if err != nil {
		return nil, err
	}
	var items []*jsonkit.Value
	switch {
	case len(vals) == 1 && vals[0].Kind == jsonkit.Array:
		items = vals[0].Items
	default:
		items = vals
	}
	t := &Table{Delim: ','}
	if len(items) == 0 {
		return t, nil
	}
	switch items[0].Kind {
	case jsonkit.Object:
		t.Header = true
		index := map[string]int{}
		var names []string
		for n, it := range items {
			if it.Kind != jsonkit.Object {
				return nil, fmt.Errorf("item %d is %s %s; every item must be an object, like the first", n+1, article(it.Kind), it.Kind)
			}
			for _, m := range it.Members {
				if _, ok := index[m.Key]; !ok {
					index[m.Key] = len(names)
					names = append(names, m.Key)
				}
			}
		}
		t.Columns = make([]Column, len(names))
		for i, n := range names {
			t.Columns[i].Name = n
		}
		for _, it := range items {
			r := make([]string, len(names))
			for _, m := range it.Members {
				r[index[m.Key]] = cellText(m.Value)
			}
			t.Rows = append(t.Rows, r)
		}
	case jsonkit.Array:
		width := 0
		for n, it := range items {
			if it.Kind != jsonkit.Array {
				return nil, fmt.Errorf("item %d is %s %s; every item must be an array, like the first", n+1, article(it.Kind), it.Kind)
			}
			width = max(width, len(it.Items))
		}
		t.Columns = make([]Column, width)
		for i := range t.Columns {
			t.Columns[i].Name = fmt.Sprintf("column %d", i+1)
		}
		for _, it := range items {
			r := make([]string, width)
			for i, v := range it.Items {
				r[i] = cellText(v)
			}
			t.Rows = append(t.Rows, r)
		}
	default:
		t.Header = true
		t.Columns = []Column{{Name: "value"}}
		for _, it := range items {
			t.Rows = append(t.Rows, []string{cellText(it)})
		}
	}
	t.count()
	return t, nil
}

// cellText is a JSON value as one cell.
func cellText(v *jsonkit.Value) string {
	switch v.Kind {
	case jsonkit.Null:
		return ""
	case jsonkit.String:
		return v.Str
	case jsonkit.Number:
		return v.Num
	case jsonkit.Bool:
		if v.Bool {
			return "true"
		}
		return "false"
	}
	return jsonkit.Encode(v, "")
}

func article(k jsonkit.Kind) string {
	if k == jsonkit.Array || k == jsonkit.Object {
		return "an"
	}
	return "a"
}
