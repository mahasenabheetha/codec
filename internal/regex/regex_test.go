package regex

import (
	"errors"
	"strings"
	"testing"
	"time"
	"unicode/utf16"
)

func TestRunBothStyles(t *testing.T) {
	text := "2026-02-03 deploy ok\n2026-02-04 deploy FAILED\n"
	for _, style := range []string{Go, PCRE} {
		o := Options{Pattern: `(?P<date>\d{4}-\d\d-\d\d) deploy (\w+)`, Style: style, All: true}
		if style == PCRE {
			o.Pattern = `(?<date>\d{4}-\d\d-\d\d) deploy (\w+)`
		}
		r, err := Run(o, text)
		if err != nil {
			t.Fatalf("%s: %v", style, err)
		}
		if len(r.Matches) != 2 || r.Matches[1].Line != 2 || r.Matches[1].Start != 21 {
			t.Fatalf("%s: %+v", style, r.Matches)
		}
		// RE2 numbers groups in order; .NET puts named ones last.
		date, word := r.Matches[1].Groups[0], r.Matches[1].Groups[1]
		if style == PCRE {
			date, word = word, date
		}
		if date.Name != "date" || date.Text != "2026-02-04" || word.Text != "FAILED" || word.Start != 39 {
			t.Errorf("%s groups: %+v", style, r.Matches[1].Groups)
		}
	}
}

func TestFlagsAndFirst(t *testing.T) {
	text := "Alpha\nbeta\nGAMMA"
	for _, style := range []string{Go, PCRE} {
		r, _ := Run(Options{Pattern: `^[a-z]+$`, Style: style, Flags: "im", All: true}, text)
		if len(r.Matches) != 3 {
			t.Errorf("%s im: %d matches", style, len(r.Matches))
		}
		r, _ = Run(Options{Pattern: `a.b`, Style: style, Flags: "s"}, "a\nb a-b")
		if len(r.Matches) != 1 || r.Matches[0].Text != "a\nb" {
			t.Errorf("%s s, first only: %+v", style, r.Matches)
		}
	}
	if _, err := Run(Options{Pattern: `a b`, Flags: "x"}, ""); err == nil {
		t.Error("x accepted by RE2")
	}
	if r, err := Run(Options{Pattern: `a b # comment`, Style: PCRE, Flags: "x"}, "ab"); err != nil || len(r.Matches) != 1 {
		t.Errorf("x in pcre: %v %v", r, err)
	}
}

func TestPCREFeatures(t *testing.T) {
	r, err := Run(Options{Pattern: `\b(\w+) \1\b`, Style: PCRE, All: true}, "it is is fine fine")
	if err != nil || len(r.Matches) != 2 {
		t.Errorf("backreference: %+v %v", r, err)
	}
	r, _ = Run(Options{Pattern: `\d+(?= ms)`, Style: PCRE, All: true}, "took 120 ms, 3 retries")
	if len(r.Matches) != 1 || r.Matches[0].Text != "120" {
		t.Errorf("lookahead: %+v", r.Matches)
	}
	r, _ = Run(Options{Pattern: `(?P<w>a)(?P=w)`, Style: PCRE}, "xaa")
	if len(r.Matches) != 1 || r.Groups[1] != "w" {
		t.Errorf("python names: %+v", r)
	}
	// Positions count UTF-16 units: é is one, 😀 is two.
	r, _ = Run(Options{Pattern: `x`, Style: PCRE, All: true}, "é😀x")
	if r.Matches[0].Start != 3 {
		t.Errorf("utf-16 start %d", r.Matches[0].Start)
	}
	r, _ = Run(Options{Pattern: `x`, All: true}, "é😀x")
	if r.Matches[0].Start != 3 {
		t.Errorf("utf-16 start (go) %d", r.Matches[0].Start)
	}
}

func TestRE2Hints(t *testing.T) {
	for _, p := range []string{`a(?=b)`, `(?<!a)b`, `(a)\1`} {
		_, err := Run(Options{Pattern: p}, "")
		var e *Error
		if !errors.As(err, &e) || !strings.Contains(e.Hint, "Python / .NET / JavaScript") {
			t.Errorf("%s: %v (hint %q)", p, err, e.Hint)
		}
	}
	if _, err := Run(Options{Pattern: `(`}, ""); err == nil || strings.HasPrefix(err.Error(), "error parsing") {
		t.Errorf("unclosed: %v", err)
	}
}

func TestTimeLimitAndCap(t *testing.T) {
	start := time.Now()
	r, err := Run(Options{Pattern: `(a+)+$`, Style: PCRE}, strings.Repeat("a", 40)+"!")
	if err != nil || !r.TimedOut || time.Since(start) > Timeout+2*time.Second {
		t.Errorf("catastrophic backtracking: %+v %v after %v", r, err, time.Since(start))
	}
	// The same pattern is linear in RE2.
	if r, err := Run(Options{Pattern: `(a+)+$`}, strings.Repeat("a", 40)+"!"); err != nil || r.TimedOut || len(r.Matches) != 0 {
		t.Errorf("re2: %+v %v", r, err)
	}
	r, _ = Run(Options{Pattern: `a`, All: true}, strings.Repeat("a", MaxMatches+5))
	if len(r.Matches) != MaxMatches || !r.Truncated {
		t.Errorf("cap: %d %v", len(r.Matches), r.Truncated)
	}
}

func TestReplace(t *testing.T) {
	text := "2026-02-03, 2026-02-04"
	for _, style := range []string{Go, PCRE} {
		o := Options{Pattern: `(\d{4})-(\d\d)-(?P<day>\d\d)`, Style: style, All: true}
		if style == PCRE {
			o.Pattern = `(\d{4})-(\d\d)-(?<day>\d\d)`
		}
		for _, tmpl := range []string{`${day}.$2.$1`, `\g<day>.\2.\1`, `$<day>.$2.$1`} {
			got, err := Replace(o, text, tmpl)
			if err != nil || got != "03.02.2026, 04.02.2026" {
				t.Errorf("%s %q: %q %v", style, tmpl, got, err)
			}
		}
		if got, _ := Replace(o, text, `$1x`); got != "2026x, 2026x" {
			t.Errorf("%s $1x: %q", style, got)
		}
		o.All = false
		if got, _ := Replace(o, text, `[$0]`); got != "[2026-02-03], 2026-02-04" {
			t.Errorf("%s first only: %q", style, got)
		}
	}
}

func TestExplain(t *testing.T) {
	flat := func(ns []*Node) string {
		var out []string
		var walk func(ns []*Node, depth int)
		walk = func(ns []*Node, depth int) {
			for _, n := range ns {
				out = append(out, strings.Repeat("  ", depth)+n.Text+" = "+n.Desc)
				walk(n.Children, depth+1)
			}
		}
		walk(ns, 0)
		return strings.Join(out, "\n")
	}
	got := flat(Explain(`^(?P<year>\d{4})-v\.[0-9a-f]+?(?:rc|beta)?\b$`, Go, ""))
	want := strings.Join([]string{
		`^ = Start of the text`,
		`(?P<year>\d{4}) = Group 1, named "year": captured`,
		`  \d{4} = Exactly 4 times, as many as possible`,
		`    \d = A digit`,
		`-v\. = The text "-v."`,
		`[0-9a-f]+? = One or more times, as few as possible (lazy)`,
		`  [0-9a-f] = One of: '0' to '9', 'a' to 'f'`,
		`(?:rc|beta)? = Optional (zero or one time), as many as possible`,
		`  (?:rc|beta) = Group (not captured)`,
		`    rc|beta = Either of 2 alternatives`,
		`      rc = either`,
		`        rc = The text "rc"`,
		`      beta = or`,
		`        beta = The text "beta"`,
		`\b = A word boundary`,
		`$ = End of the text`,
	}, "\n")
	if got != want {
		t.Errorf("explain:\n%s\nwant:\n%s", got, want)
	}
	for _, c := range []struct{ pattern, style, flags, want string }{
		{`a(?=b)`, PCRE, "", "Lookahead: followed by this"},
		{`(?<!x)y`, PCRE, "", "Negative lookbehind"},
		{`(a)\1`, PCRE, "", "The same text group 1 matched"},
		{`\k<w>`, PCRE, "", `The same text group "w" matched`},
		{`[^\s,]`, Go, "", "Any character except whitespace (space, tab, line break), ','"},
		{`.`, Go, "s", "Any character"},
		{`^`, Go, "m", "Start of a line"},
		{`(?i)abc`, Go, "", "From here on: ignore case"},
		{`a{2,}`, Go, "", "2 or more times"},
		{`a{2,5}`, Go, "", "2 to 5 times"},
		{`a++`, PCRE, "", "A repeat with nothing before it to repeat"}, // no possessive repeats in regexp2
		{`(?<n>x)(y)`, PCRE, "", `Group 2, named "n"`},                 // .NET numbering
		{`\p{L}`, Go, "", "Unicode class L"},
		{`(ab`, Go, "", "A ( without a matching )"},
		{`[ab`, Go, "", "A [ without a matching ]"},
	} {
		if got := flat(Explain(c.pattern, c.style, c.flags)); !strings.Contains(got, c.want) {
			t.Errorf("%s (%s %s):\n%s\nwant %q", c.pattern, c.style, c.flags, got, c.want)
		}
	}
	// Positions are UTF-16 offsets in the pattern.
	ns := Explain(`😀(a)`, Go, "")
	if g := ns[1]; g.Start != 2 || g.End != 5 {
		t.Errorf("group at %d-%d", g.Start, g.End)
	}
}

// Findings from the 2.3.0 review.
func TestReviewCases(t *testing.T) {
	// Unfinished patterns, as typed one key at a time, explain without a panic.
	for _, p := range []string{`(?`, `(?<`, `(?<n`, `(?P<`, `(?P=x`, `(?#x`, `a(?i`, `[[:`, `\k<x`, `\x{12`, `\p{L`, `\`, `[`, `(`, `a{2,`, `\Q`} {
		Explain(p, PCRE, "")
		Explain(p, Go, "")
	}
	// A long literal explains in linear time.
	start := time.Now()
	if ns := Explain(strings.Repeat("a", 100_000), Go, ""); len(ns) != 1 || len(ns[0].Text) != 100_000 || time.Since(start) > time.Second {
		t.Errorf("long literal: %d nodes in %v", len(ns), time.Since(start))
	}
	// \Q…\E stays one part, and parentheses in it or a ] first in a class open no group.
	if ns := Explain(`a\Qb.c\E(x)`, Go, ""); len(ns) != 3 || ns[1].Kind != "quote" || ns[2].Group != 1 {
		t.Errorf(`\Q: %+v`, ns)
	}
	if ns := Explain(`[](](a)`, Go, ""); ns[len(ns)-1].Group != 1 {
		t.Errorf("leading ] in a class: %+v", ns[len(ns)-1])
	}
	// Text ending in an invalid UTF-8 byte.
	for _, style := range []string{Go, PCRE} {
		if r, err := Run(Options{Pattern: `b`, Style: style, All: true}, "a\xffb\xff"); err != nil || len(r.Matches) != 1 || r.Matches[0].Start != 2 {
			t.Errorf("%s invalid utf-8: %+v %v", style, r, err)
		}
	}
	// Templates: $$ and \ are literal, and a group the pattern doesn't
	// have stays as written, the same in both styles.
	for _, style := range []string{Go, PCRE} {
		o := Options{Pattern: `(?<n>b)`, Style: style, All: true}
		for tmpl, want := range map[string]string{`$$1`: "a$1c", `$$<n>`: "a$<n>c", `\\1`: `a\1c`, `$USD`: "a$USDc", `$9`: "a$9c", `[${n}$1]`: "a[bb]c", `$`: "a$c"} {
			if got, err := Replace(o, "abc", tmpl); err != nil || got != want {
				t.Errorf("%s %q: %q %v, want %q", style, tmpl, got, err, want)
			}
		}
	}
	// A replace that backtracks badly on many segments stops near the limit.
	start = time.Now()
	_, err := Replace(Options{Pattern: `(a+)+b|c`, Style: PCRE, All: true}, strings.Repeat(strings.Repeat("a", 22)+"c", 8), "x")
	if !errors.Is(err, ErrTimeout) || time.Since(start) > Timeout+3*time.Second {
		t.Errorf("replace limit: %v after %v", err, time.Since(start))
	}
}

// FuzzExplain: any pattern explains without a panic, with parts inside
// the pattern.
func FuzzExplain(f *testing.F) {
	for _, s := range []string{`^(?P<y>\d{4})-v\.[0-9a-f]+?(?:rc|beta)?\b$`, `(?<!x)y\k<w>`, `[^\s,][]a]`, `a\Qb(c\E`, `(?i:a|b)*`, `x{2,5}?`, "😀(a)"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, p string) {
		limit := len(utf16.Encode([]rune(p)))
		var walk func(ns []*Node)
		walk = func(ns []*Node) {
			for _, n := range ns {
				if n.Start < 0 || n.End > limit || n.Start > n.End {
					t.Fatalf("%q: part %d-%d of %d", p, n.Start, n.End, limit)
				}
				walk(n.Children)
			}
		}
		for _, style := range []string{Go, PCRE} {
			walk(Explain(p, style, "imsx"))
		}
	})
}
