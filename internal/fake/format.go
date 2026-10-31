package fake

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/mahasenabheetha/codec/v2/internal/jsonkit"
	"github.com/mahasenabheetha/codec/v2/internal/table"
)

// Formats are what rows can be written as.
var Formats = []string{"json", "ndjson", "csv", "yaml", "sql"}

// sqlName is a table or column name SQL takes without quoting.
var sqlName = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// Write makes count rows for cols and writes them as format. Table is
// the SQL table's name.
func Write(cols []Column, count int, seed uint64, format, tableName string) (string, error) {
	rows, err := Rows(cols, count, seed)
	if err != nil {
		return "", err
	}
	switch format {
	case "", "json":
		return jsonkit.Encode(asJSON(cols, rows), "  ") + "\n", nil
	case "ndjson":
		var b strings.Builder
		for _, it := range asJSON(cols, rows).Items {
			b.WriteString(jsonkit.Encode(it, ""))
			b.WriteByte('\n')
		}
		return b.String(), nil
	case "yaml":
		return jsonkit.ToYAML(asJSON(cols, rows))
	case "csv":
		t := &table.Table{Header: true, Columns: make([]table.Column, len(cols))}
		for i, c := range cols {
			t.Columns[i].Name = c.Name
		}
		for _, r := range rows {
			cells := make([]string, len(r))
			for i, v := range r {
				cells[i] = text(v)
			}
			t.Rows = append(t.Rows, cells)
		}
		return t.Delimited(','), nil
	case "sql":
		return asSQL(cols, rows, tableName)
	}
	return "", fmt.Errorf("format is json, ndjson, csv, yaml or sql, not %q", format)
}

func text(v value) string {
	switch {
	case v.blank:
		return ""
	case v.kind == 'b':
		if v.b {
			return "true"
		}
		return "false"
	}
	return v.s
}

func asJSON(cols []Column, rows [][]value) *jsonkit.Value {
	out := &jsonkit.Value{Kind: jsonkit.Array, Items: make([]*jsonkit.Value, len(rows))}
	for i, r := range rows {
		obj := &jsonkit.Value{Kind: jsonkit.Object, Members: make([]jsonkit.Member, len(r))}
		for j, v := range r {
			var jv *jsonkit.Value
			switch {
			case v.blank:
				jv = &jsonkit.Value{Kind: jsonkit.Null}
			case v.kind == 'i' || v.kind == 'n':
				jv = &jsonkit.Value{Kind: jsonkit.Number, Num: v.s}
			case v.kind == 'b':
				jv = &jsonkit.Value{Kind: jsonkit.Bool, Bool: v.b}
			default:
				jv = &jsonkit.Value{Kind: jsonkit.String, Str: v.s}
			}
			obj.Members[j] = jsonkit.Member{Key: cols[j].Name, Value: jv}
		}
		out.Items[i] = obj
	}
	return out
}

// asSQL writes one INSERT per row (easy to cut or reorder), names quoted
// with double quotes only when they need it.
func asSQL(cols []Column, rows [][]value, tableName string) (string, error) {
	if tableName == "" {
		tableName = "items"
	}
	ident := func(s string) string {
		if sqlName.MatchString(s) {
			return s
		}
		return `"` + strings.ReplaceAll(s, `"`, `""`) + `"`
	}
	names := make([]string, len(cols))
	for i, c := range cols {
		names[i] = ident(c.Name)
	}
	head := fmt.Sprintf("INSERT INTO %s (%s) VALUES (", ident(tableName), strings.Join(names, ", "))
	var b strings.Builder
	for _, r := range rows {
		vals := make([]string, len(r))
		for i, v := range r {
			switch {
			case v.blank:
				vals[i] = "NULL"
			case v.kind == 'i' || v.kind == 'n':
				vals[i] = v.s
			case v.kind == 'b':
				vals[i] = strings.ToUpper(text(v))
			default:
				vals[i] = "'" + strings.ReplaceAll(v.s, "'", "''") + "'"
			}
		}
		b.WriteString(head)
		b.WriteString(strings.Join(vals, ", "))
		b.WriteString(");\n")
	}
	return b.String(), nil
}
