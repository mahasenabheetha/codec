package fake

import (
	"encoding/json"
	"net"
	"strings"
	"testing"
)

func TestLorem(t *testing.T) {
	s, err := Lorem("words", 5, 1, true)
	if err != nil || s != "lorem ipsum dolor sit amet" {
		t.Errorf("classic words: %q %v", s, err)
	}
	a, _ := Lorem("paragraphs", 3, 42, false)
	b, _ := Lorem("paragraphs", 3, 42, false)
	c, _ := Lorem("paragraphs", 3, 43, false)
	if a != b || a == c || strings.Count(a, "\n\n") != 2 {
		t.Errorf("seeded paragraphs: same=%v differ=%v", a == b, a != c)
	}
	s, _ = Lorem("sentences", 2, 7, true)
	if !strings.HasPrefix(s, "Lorem ipsum dolor sit amet") || strings.Count(s, ".") != 2 {
		t.Errorf("sentences: %q", s)
	}
	for _, bad := range []struct {
		unit string
		n    int
	}{{"words", 0}, {"words", MaxLorem + 1}, {"pages", 1}} {
		if _, err := Lorem(bad.unit, bad.n, 1, false); err == nil {
			t.Errorf("%+v accepted", bad)
		}
	}
}

func TestRowsAndFormats(t *testing.T) {
	cols, err := ParseSpec("id:id,name:name,email:email,ip:ipv4,v6:ipv6,phone:phone,age:integer:18:90,score:number:0:1:3,ok:boolean,on:date:2024-01-01:2024-12-31,plan:choice:free|pro,note:word?50")
	if err != nil {
		t.Fatal(err)
	}
	out, err := Write(cols, 50, 9, "json", "")
	if err != nil {
		t.Fatal(err)
	}
	var rows []map[string]any
	if err := json.Unmarshal([]byte(out), &rows); err != nil || len(rows) != 50 {
		t.Fatalf("json: %v %d", err, len(rows))
	}
	blanks := 0
	for i, r := range rows {
		name := r["name"].(string)
		first, last, _ := strings.Cut(name, " ")
		email := r["email"].(string)
		if !strings.HasPrefix(email, strings.ToLower(first)+"."+strings.ToLower(last)+"@example.") {
			t.Errorf("email %q doesn't follow %q", email, name)
		}
		if r["id"] != float64(i+1) {
			t.Errorf("id %v", r["id"])
		}
		ip := net.ParseIP(r["ip"].(string))
		ok := false
		for _, n := range []string{"192.0.2.0/24", "198.51.100.0/24", "203.0.113.0/24"} {
			_, nw, _ := net.ParseCIDR(n)
			ok = ok || nw.Contains(ip)
		}
		_, v6net, _ := net.ParseCIDR("2001:db8::/32")
		if !ok || !v6net.Contains(net.ParseIP(r["v6"].(string))) {
			t.Errorf("not documentation addresses: %v %v", r["ip"], r["v6"])
		}
		if !strings.Contains(r["phone"].(string), "-555-01") {
			t.Errorf("phone %v", r["phone"])
		}
		if a := r["age"].(float64); a < 18 || a > 90 {
			t.Errorf("age %v", a)
		}
		if d := r["on"].(string); d < "2024-01-01" || d > "2024-12-31" {
			t.Errorf("date %v", d)
		}
		if p := r["plan"]; p != "free" && p != "pro" {
			t.Errorf("plan %v", p)
		}
		if r["note"] == nil {
			blanks++
		}
	}
	if blanks < 10 || blanks > 40 {
		t.Errorf("about half the notes blank, got %d of 50", blanks)
	}
	if !strings.Contains(out, `"score": 0.`) {
		t.Errorf("three decimals kept as numbers: %s", out[:200])
	}
	// The same seed, the same rows, in every format.
	for _, f := range Formats {
		a, err := Write(cols, 3, 5, f, "people")
		b, _ := Write(cols, 3, 5, f, "people")
		if err != nil || a != b || a == "" {
			t.Errorf("%s: %v", f, err)
		}
	}
	sql, _ := Write([]Column{{Name: "id", Kind: "id"}, {Name: "full name", Kind: "choice", Choices: []string{"O'Brien"}}, {Name: "ok", Kind: "boolean", Blank: 100}}, 1, 1, "sql", "my table")
	if sql != `INSERT INTO "my table" (id, "full name", ok) VALUES (1, 'O''Brien', NULL);`+"\n" {
		t.Errorf("sql: %q", sql)
	}
	csv, _ := Write([]Column{{Name: "a", Kind: "choice", Choices: []string{"x,y"}}}, 1, 1, "csv", "")
	if csv != "a\n\"x,y\"\n" {
		t.Errorf("csv: %q", csv)
	}
}

func TestErrors(t *testing.T) {
	for _, spec := range []string{"", "x", "a:nope", "a:integer:5:1", "a:choice", "a:id,a:id", "a:word:extra", "a:date:soon", "a:word?200"} {
		cols, err := ParseSpec(spec)
		if err == nil {
			_, err = Rows(cols, 1, 1)
		}
		if err == nil {
			t.Errorf("%q accepted", spec)
		}
	}
	if _, err := Rows(Presets["users"], MaxRows+1, 1); err == nil {
		t.Error("too many rows accepted")
	}
	if _, err := Write(Presets["users"], 1, 1, "xml", ""); err == nil {
		t.Error("xml accepted")
	}
	for name, cols := range Presets {
		if _, err := Write(cols, 5, 1, "json", ""); err != nil {
			t.Errorf("preset %s: %v", name, err)
		}
	}
}
