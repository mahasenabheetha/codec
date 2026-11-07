package quote

import (
	"strings"
	"testing"
)

// The values every target must carry exactly: quotes of both kinds,
// the shells' specials, line breaks, tabs, non-ASCII, YAML's traps.
var values = []string{
	"plain", "two words", "it's", `say "hi"`, `$HOME and ${X} and $(id)`, "back`tick", `C:\path\to\dir\`,
	"a!b", "50%", "line one\nline two", "tab\there", "trailing\n", "  leading", "ünïcödé ✓", "no", "1.10", "0755",
	"key: value", "- item", "#comment", "a,b", "x]]>y", "]]", "''", `\"`, "-n", "*.log", "",
}

// Each target's best form reads back to the value (for makefile, its
// variable form: a recipe's is read by the shell too).
func TestRoundTrip(t *testing.T) {
	for _, tg := range Targets {
		for _, v := range values {
			if skip(tg.ID, v) {
				continue
			}
			r, err := Quote(tg.ID, v)
			if err != nil {
				t.Fatal(err)
			}
			form := r.Forms[0]
			if tg.ID == "makefile" {
				form = r.Forms[1]
			}
			got, err := Unquote(tg.ID, form.Value)
			if err != nil || got != v {
				t.Errorf("%s %q: %s %q reads back as %q (%v)", tg.ID, v, form.Label, form.Value, got, err)
			}
		}
	}
}

// What a target can't carry, said in its notes instead.
func skip(target, v string) bool {
	switch target {
	case "cmd":
		return strings.ContainsAny(v, "\n\t") // cmd has no way to pass a line break
	case "env", "dockerfile", "html", "xml", "helm-set", "makefile", "regex":
		return v == "" && target == "regex"
	}
	return false
}

func TestQuote(t *testing.T) {
	for _, c := range []struct{ target, value, want, note string }{
		{"bash", "it's", `'it'\''s'`, ""},
		{"bash", "a\nb", `$'a\nb'`, ""},
		{"bash", "plain.txt", "plain.txt", ""},
		{"bash", "-n", "-n", "put -- before it"},
		{"sh", "a\nb", "'a\nb'", "no $'…'"},
		{"powershell", "it's $x", `'it''s $x'`, ""},
		{"cmd", `say "hi"`, `"say \"hi\""`, ""},
		{"cmd", "50%", `"50%"`, "%NAME%"},
		{"json", "a\"b</", `"a\"b</"`, ""},
		{"yaml", "no", `'no'`, "boolean"},
		{"yaml", "1.10", `'1.10'`, "1.10 becomes 1.1"},
		{"yaml", "0755", `'0755'`, "octal"},
		{"yaml", "hello world", "hello world", ""},
		{"yaml", "key: value", `'key: value'`, "plain, it would not"},
		{"yaml", "a\nb\n", "|\n  a\n  b\n", ""},
		{"env", "two words", `"two words"`, "--env-file"},
		{"helm-set", "a,b", `a\,b`, ""},
		{"helm-set", "true", `true`, "--set-string"},
		{"makefile", "$HOME", "'$$HOME'", ""},
		{"dockerfile", "echo $X", `["echo $X"]`, ""},
		{"sql", "O'Brien", "'O''Brien'", "parameters"},
		{"regex", "a.b*", `a\.b\*`, ""},
		{"go", "a\"b", "`a\"b`", ""},
		{"go", "back`tick", "\"back`tick\"", ""},
		{"xml", "<a & b>", "&lt;a &amp; b&gt;", ""},
		{"html", `<b class="x">`, "&lt;b class=&#34;x&#34;&gt;", "<script>"},
	} {
		r, _ := Quote(c.target, c.value)
		if r.Forms[0].Value != c.want {
			t.Errorf("%s %q: %q, want %q", c.target, c.value, r.Forms[0].Value, c.want)
		}
		if c.note != "" && !strings.Contains(strings.Join(r.Notes, " "), c.note) {
			t.Errorf("%s %q: notes %q don't say %q", c.target, c.value, r.Notes, c.note)
		}
	}
}

// bash in kubectl exec -- sh -c, in ssh: each layer quotes the whole of
// the one inside it.
func TestLayers(t *testing.T) {
	steps, err := Layers(`echo "$HOME"`, []Layer{
		{Target: "sh", Before: "kubectl exec -it web -- sh -c "},
		{Target: "bash", Before: "ssh bastion "},
	})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		`kubectl exec -it web -- sh -c 'echo "$HOME"'`,
		`ssh bastion 'kubectl exec -it web -- sh -c '\''echo "$HOME"'\'''`,
	}
	for i, s := range steps {
		if s.Value != want[i] {
			t.Errorf("step %d: %s\nwant %s", i, s.Value, want[i])
		}
	}
	// Read back layer by layer, the value comes out.
	inner, _ := Unquote("bash", strings.TrimPrefix(steps[1].Value, "ssh bastion "))
	if inner != steps[0].Value {
		t.Errorf("outer layer reads back as %q", inner)
	}
	if _, err := Layers("x", []Layer{{Target: "nope"}}); err == nil {
		t.Error("unknown target passed")
	}
}

func FuzzRoundTrip(f *testing.F) {
	for _, v := range values {
		f.Add(v)
	}
	f.Fuzz(func(t *testing.T, v string) {
		if !utf8Valid(v) {
			return
		}
		for _, tg := range []string{"bash", "json", "go", "sql", "powershell"} {
			r, _ := Quote(tg, v)
			got, err := Unquote(tg, r.Forms[0].Value)
			if err != nil || got != v {
				t.Fatalf("%s %q: %q reads back as %q (%v)", tg, v, r.Forms[0].Value, got, err)
			}
		}
	})
}

func utf8Valid(s string) bool { return strings.ToValidUTF8(s, "\uFFFD") == s }
