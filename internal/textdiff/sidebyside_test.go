package textdiff

import (
	"fmt"
	"strings"
	"testing"
	"time"
	"unicode/utf16"
)

// marked shows a cell with its spans in [brackets].
func marked(c *Cell) string {
	if c == nil {
		return "-"
	}
	u := utf16.Encode([]rune(c.Text))
	var b strings.Builder
	at := 0
	for _, s := range c.Spans {
		b.WriteString(string(utf16.Decode(u[at:s[0]])) + "[" + string(utf16.Decode(u[s[0]:s[1]])) + "]")
		at = s[1]
	}
	b.WriteString(string(utf16.Decode(u[at:])))
	return b.String()
}

func TestInline(t *testing.T) {
	tests := []struct {
		name, a, b, wantA, wantB string
		o                        Options
	}{
		{"tail of a hash", "sha256:4f00f62e38fcd6bfe349fb00449ca84a322235", "sha256:4f00f62e38fcd6w8as4de4f7e8s4f8e64fwswf",
			"sha256:4f00f62e38fcd6[bfe349fb00449ca84a322235]", "sha256:4f00f62e38fcd6[w8as4de4f7e8s4f8e64fwswf]", Options{}},
		{"one character", "replicas: 2", "replicas: 3", "replicas: [2]", "replicas: [3]", Options{}},
		{"insert only", "image: shop:1.2", "image: shop:1.2.1", "image: shop:1.2", "image: shop:1.2[.1]", Options{}},
		{"two separate edits", "name: api, port: 80", "name: web, port: 81", "name: [api], port: 8[0]", "name: [web], port: 8[1]", Options{}},
		{"mostly different", "abc", "xyz", "[abc]", "[xyz]", Options{}},
		{"utf-16 offsets", "😀 tag: a", "😀 tag: b", "😀 tag: [a]", "😀 tag: [b]", Options{}},
		{"ignore case", "Image: Shop", "image: shop:2", "Image: Shop", "image: shop[:2]", Options{IgnoreCase: true}},
		{"ignore space drops blank spans", "a:  b c", "a: b cd", "a:  b c", "a: b c[d]", Options{IgnoreSpace: true}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			budget := inlineBudget
			l, r := inline(tt.a, tt.b, tt.o, &budget)
			if got := marked(&Cell{Text: tt.a, Spans: l}); got != tt.wantA {
				t.Errorf("left  %s\nwant  %s", got, tt.wantA)
			}
			if got := marked(&Cell{Text: tt.b, Spans: r}); got != tt.wantB {
				t.Errorf("right %s\nwant  %s", got, tt.wantB)
			}
		})
	}
}

func TestSideBySide(t *testing.T) {
	var old, new []string
	for k := 1; k <= 20; k++ {
		old = append(old, fmt.Sprintf("line %d", k))
	}
	new = append(new, old...)
	new[9] = "line ten"                 // change
	new = append(new[:15], new[16:]...) // delete line 16
	new = append(new, "line 21")        // insert at the end
	rows := SideBySide(strings.Join(old, "\n")+"\n", strings.Join(new, "\r\n"), 2, Options{})

	var got []string
	for _, r := range rows {
		switch r.Kind {
		case "gap":
			got = append(got, fmt.Sprintf("gap %d@%d/%d", r.Count, r.Left.Line, r.Right.Line))
		default:
			got = append(got, fmt.Sprintf("%s %s|%s", r.Kind, marked(r.Left), marked(r.Right)))
		}
	}
	want := []string{
		"gap 7@1/1",
		"equal line 8|line 8", "equal line 9|line 9",
		"change line [10]|line [ten]",
		"equal line 11|line 11", "equal line 12|line 12",
		"equal line 13|line 13", // one line between contexts is not folded
		"equal line 14|line 14", "equal line 15|line 15",
		"delete line 16|-",
		"equal line 17|line 17", "equal line 18|line 18",
		"equal line 19|line 19",
		"equal line 20|line 20",
		"insert -|line 21",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("got\n%s\nwant\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}

	if rows := SideBySide("a\nb\n", "a\r\nb", 3, Options{}); rows != nil {
		t.Errorf("line endings only: %v", rows)
	}
	if rows := SideBySide("A  b\n", "a b\n", 3, Options{IgnoreSpace: true, IgnoreCase: true}); rows != nil {
		t.Errorf("loose match: %v", rows)
	}
}

// Two large, wholly different texts must stay cheap: the character
// diff budget runs out and the rest is marked whole.
func TestSideBySideBudget(t *testing.T) {
	var a, b strings.Builder
	for k := range 5000 {
		fmt.Fprintf(&a, "%d %s\n", k, strings.Repeat("abcdefghij", 25))
		fmt.Fprintf(&b, "%d %s\n", k, strings.Repeat("jihgfedcba", 25))
	}
	start := time.Now()
	rows := SideBySide(a.String(), b.String(), 3, Options{})
	if len(rows) != 5000 || rows[4999].Kind != "change" || len(rows[4999].Left.Spans) == 0 {
		t.Fatalf("rows = %d", len(rows))
	}
	if d := time.Since(start); d > 2*time.Second {
		t.Errorf("took %v", d)
	}
}
