package table

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestGuessDelim(t *testing.T) {
	cases := map[string]rune{
		"a,b,c\n1,2,3\n":                 ',',
		"a\tb\tc\n1\t2\t3\n":             '\t',
		"name;price\nmilk;1,5\negg;2,25": ';',
		"a|b\n1|2\n":                     '|',
		"just one column\nsecond\n":      ',',
		"note\tsize\nhi, there\t3\n":     '\t',
		"\"a,b\";c\n\"x\";y\n":           ';',
	}
	for src, want := range cases {
		tb, err := Parse(src, Options{})
		if err != nil {
			t.Fatalf("%q: %v", src, err)
		}
		if tb.Delim != want || !tb.DelimGuessed {
			t.Errorf("%q: delimiter %q, want %q", src, tb.Delim, want)
		}
	}
}

func TestGuessHeader(t *testing.T) {
	cases := map[string]bool{
		"name,age\nann,31\nbob,42":       true,  // numbers below text
		"1,2\n3,4":                       false, // numbers can't be names
		"name,city\nann,paris\nbob,rome": true,  // nothing says otherwise
		"a,b\na,c\nd,e":                  false, // a name is also a value
		"id,code\nx,AB12\ny,CD34":        true,  // codes of one length
		",b\n1,2":                        false, // an empty name
		"x,x\n1,2":                       false, // a repeated name
		"when,ok\n2026-01-02,true\n2026-01-03,false": true,
	}
	for src, want := range cases {
		tb, err := Parse(src, Options{})
		if err != nil {
			t.Fatal(err)
		}
		if tb.Header != want {
			t.Errorf("%q: header %v, want %v", src, tb.Header, want)
		}
	}
}

func TestParse(t *testing.T) {
	src := "\xef\xbb\xbfid,name,score,ok,when,zip\r\n" +
		"1,\"Smith, Ann\",3.5,true,2026-10-31,007\r\n" +
		"2,\"two\nlines\",,FALSE,2026-10-31T10:00:00Z,123\r\n" +
		"3,\"say \"\"hi\"\"\",4,true,,0\r\n"
	tb, err := Parse(src, Options{})
	if err != nil {
		t.Fatal(err)
	}
	if !tb.Header || tb.Delim != ',' || len(tb.Rows) != 3 || tb.Ragged != 0 {
		t.Fatalf("got %+v", tb)
	}
	types := []string{}
	for _, c := range tb.Columns {
		types = append(types, c.Name+":"+c.Type)
	}
	want := []string{"id:integer", "name:text", "score:number", "ok:boolean", "when:date", "zip:text"}
	if !reflect.DeepEqual(types, want) {
		t.Errorf("columns %v, want %v", types, want)
	}
	if tb.Rows[1][1] != "two\nlines" || tb.Rows[2][1] != `say "hi"` {
		t.Errorf("rows %q", tb.Rows)
	}
	if c := tb.Columns[2]; c.Filled != 2 || c.Empty != 1 || c.Distinct != 2 {
		t.Errorf("score counts %+v", c)
	}
}

func TestRaggedAndNames(t *testing.T) {
	tb, err := Parse("a,,a\n1,2\n1,2,3,4\n", Options{Header: "yes"})
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, c := range tb.Columns {
		names = append(names, c.Name)
	}
	if !reflect.DeepEqual(names, []string{"a", "column 2", "a_2", "column 4"}) {
		t.Errorf("names %q", names)
	}
	if tb.Ragged != 2 || len(tb.Rows[0]) != 4 {
		t.Errorf("ragged %d, rows %q", tb.Ragged, tb.Rows)
	}
}

func TestSyntaxError(t *testing.T) {
	_, err := Parse("a,b\n1,\"é open\n", Options{Delim: ','})
	var se *SyntaxError
	if !errors.As(err, &se) || se.Line < 2 {
		t.Fatalf("got %v", err)
	}
	_, err = Parse("a,b\n1,x\"y\n", Options{Delim: ','})
	if !errors.As(err, &se) || se.Line != 2 || se.Column != 4 || !strings.Contains(se.Msg, "quote") {
		t.Fatalf("got %#v", err)
	}
	// A quote inside a TSV field is just a character.
	tb, err := Parse("a\tb\n1\tx\"y\n", Options{Delim: '\t'})
	if err != nil || tb.Rows[0][1] != `x"y` {
		t.Fatalf("tsv: %v %v", err, tb)
	}
	if _, err := Parse("a", Options{Header: "maybe"}); err == nil {
		t.Error("a bad header option passed")
	}
}

func TestConvert(t *testing.T) {
	tb, err := Parse("id,name,ok,note\n1,ann,true,\n2,\"b|c\",false,x\n", Options{})
	if err != nil {
		t.Fatal(err)
	}
	check := func(format string, typed bool, want string) {
		t.Helper()
		got, err := Convert(tb, format, typed, "")
		if err != nil {
			t.Fatal(err)
		}
		if got != want {
			t.Errorf("%s typed=%v:\n%s\nwant\n%s", format, typed, got, want)
		}
	}
	check("json", true, `[{"id":1,"name":"ann","ok":true,"note":null},{"id":2,"name":"b|c","ok":false,"note":"x"}]`)
	check("json", false, `[{"id":"1","name":"ann","ok":"true","note":""},{"id":"2","name":"b|c","ok":"false","note":"x"}]`)
	check("jsonl", true, "{\"id\":1,\"name\":\"ann\",\"ok\":true,\"note\":null}\n{\"id\":2,\"name\":\"b|c\",\"ok\":false,\"note\":\"x\"}\n")
	check("markdown", true, "| id | name | ok | note |\n| --: | --- | --- | --- |\n| 1 | ann | true |  |\n| 2 | b\\|c | false | x |\n")
	check("tsv", true, "id\tname\tok\tnote\n1\tann\ttrue\t\n2\tb|c\tfalse\tx\n")
	check("yaml", true, "- id: 1\n  name: ann\n  ok: true\n  note: null\n- id: 2\n  name: b|c\n  ok: false\n  note: x\n")
	if _, err := Convert(tb, "xml", true, ""); err == nil {
		t.Error("xml converted")
	}

	plain, _ := Parse("1,2\n3,4\n", Options{})
	got, _ := Convert(plain, "json", true, "")
	if got != "[[1,2],[3,4]]" {
		t.Errorf("no header: %s", got)
	}
}

func TestFromJSON(t *testing.T) {
	tb, err := FromJSON(`[{"a": 1, "b": "x,y"}, {"c": null, "a": {"n": [1]}, "b": true}]`)
	if err != nil {
		t.Fatal(err)
	}
	want := "a,b,c\n1,\"x,y\",\n\"{\"\"n\"\":[1]}\",true,\n"
	if got := tb.Delimited(','); got != want {
		t.Errorf("got\n%s\nwant\n%s", got, want)
	}
	if tb.Columns[0].Type != "text" || tb.Columns[2].Type != "empty" {
		t.Errorf("types %+v", tb.Columns)
	}

	lines, err := FromJSON("{\"a\":1}\n{\"a\":2}\n")
	if err != nil || len(lines.Rows) != 2 {
		t.Fatalf("JSON Lines: %v %v", err, lines)
	}
	rows, _ := FromJSON(`[[1, 2], [3]]`)
	if rows.Header || rows.Delimited('\t') != "1\t2\n3\t\n" {
		t.Errorf("arrays: %q", rows.Delimited('\t'))
	}
	scalars, _ := FromJSON(`["a", 2]`)
	if scalars.Delimited(',') != "value\na\n2\n" {
		t.Errorf("scalars: %q", scalars.Delimited(','))
	}
	if _, err := FromJSON(`[{"a":1}, 2]`); err == nil || !strings.Contains(err.Error(), "item 2 is a number") {
		t.Errorf("mixed: %v", err)
	}
	empty, err := FromJSON(`[]`)
	if err != nil || empty.Delimited(',') != "" {
		t.Errorf("empty: %v", err)
	}
}

func TestRoundTrip(t *testing.T) {
	src := "a,b\n\"x\ny\",\"q\"\"\"\n\"\",\n"
	tb, err := Parse(src, Options{Header: "no"})
	if err != nil {
		t.Fatal(err)
	}
	back, err := Parse(tb.Delimited(','), Options{Delim: ',', Header: "no"})
	if err != nil || !reflect.DeepEqual(back.Rows, tb.Rows) {
		t.Errorf("round trip: %v\n%q\n%q", err, tb.Rows, back.Rows)
	}
	one, _ := Parse("a\n\"\"\n", Options{Delim: ',', Header: "yes"})
	if len(one.Rows) != 1 || one.Delimited(',') != "a\n\"\"\n" {
		t.Errorf("empty row: %q", one.Delimited(','))
	}
}
