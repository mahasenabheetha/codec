package yamlkit

import (
	"os"
	"strings"
	"testing"
)

// findKey returns the first map pair whose key is key, depth-first.
func findKey(n *Node, key string) *Pair {
	if n == nil {
		return nil
	}
	for _, p := range n.Pairs {
		if p.Key.Value == key {
			return p
		}
		if hit := findKey(p.Value, key); hit != nil {
			return hit
		}
	}
	for _, it := range n.Items {
		if hit := findKey(it, key); hit != nil {
			return hit
		}
	}
	return nil
}

// bomPrefix is the UTF-8 byte-order mark, spelled as bytes so the
// source file itself stays BOM-free.
var bomPrefix = string([]byte{0xEF, 0xBB, 0xBF})

func TestPositions(t *testing.T) {
	tests := []struct {
		name     string
		src      string
		wantLine int
		wantCol  int
		wantOff  int
	}{
		{"LF", "a: 1\nspec:\n  image: nginx\n", 3, 3, 13},
		{"CRLF", "a: 1\r\nspec:\r\n  image: nginx\r\n", 3, 3, 15},
		{"BOM is not counted", bomPrefix + "a: 1\nspec:\n  image: nginx\n", 3, 3, 13},
		{"non-ASCII before key", "é: 1\nspec:\n  image: nginx\n", 3, 3, 14},
		{"second document", "kind: A\n---\nspec:\n  image: nginx\n", 4, 3, 20},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := Parse([]byte(tt.src))
			if len(f.Diagnostics) > 0 {
				t.Fatalf("unexpected diagnostics: %+v", f.Diagnostics)
			}
			var pair *Pair
			for _, d := range f.Docs {
				if pair = findKey(d.Root, "image"); pair != nil {
					break
				}
			}
			if pair == nil {
				t.Fatal("key image not found")
			}
			got := pair.Key.Range.Start
			if got.Line != tt.wantLine || got.Col != tt.wantCol || got.Offset != tt.wantOff {
				t.Errorf("image key at %+v, want line %d col %d offset %d", got, tt.wantLine, tt.wantCol, tt.wantOff)
			}
			// The value ends right after "nginx" on the same line.
			if end := pair.Value.Range.End; end.Line != tt.wantLine || end.Col != tt.wantCol+12 {
				t.Errorf("value ends at %+v, want line %d col %d", end, tt.wantLine, tt.wantCol+12)
			}
		})
	}
}

func TestScalarRanges(t *testing.T) {
	src := "a: \"quoted\"  # note\nb: |\n  line1\n  line2\nc: plain\n  continued\nd:\n"
	f := Parse([]byte(src))
	root := f.Docs[0].Root

	tests := []struct {
		key                 string
		startLine, startCol int
		endLine, endCol     int
		value, tag          string
	}{
		{"a", 1, 4, 1, 12, "quoted", TagStr},
		{"b", 2, 4, 4, 8, "line1\nline2\n", TagStr},
		{"c", 5, 4, 6, 12, "plain continued", TagStr},
		{"d", 7, 3, 7, 3, "", TagNull},
	}
	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			v := root.Get(tt.key)
			r := v.Range
			if r.Start.Line != tt.startLine || r.Start.Col != tt.startCol || r.End.Line != tt.endLine || r.End.Col != tt.endCol {
				t.Errorf("range %d:%d-%d:%d, want %d:%d-%d:%d", r.Start.Line, r.Start.Col, r.End.Line, r.End.Col,
					tt.startLine, tt.startCol, tt.endLine, tt.endCol)
			}
			if v.Value != tt.value || v.Tag != tt.tag {
				t.Errorf("value %q tag %s, want %q %s", v.Value, v.Tag, tt.value, tt.tag)
			}
		})
	}
}

// Regression: goccy counts each comment line twice in CRLF files, which
// shifted every position after a comment (found on real playbooks).
func TestCRLFWithComments(t *testing.T) {
	src := "# c1\r\n# c2\r\n\r\n- name: x\r\n  run: |\r\n    echo a\r\n    echo b\r\n  after: 1\r\n"
	root := Parse([]byte(src)).Docs[0].Root
	item := root.Items[0]
	if l := item.Get("name").Range.Start.Line; l != 4 {
		t.Errorf("name on line %d, want 4", l)
	}
	run := item.Get("run")
	if s, e := run.Range.Start, run.Range.End; s.Line != 5 || e.Line != 7 || e.Col != 11 {
		t.Errorf("run block spans %d:%d-%d:%d, want 5:8-7:11", s.Line, s.Col, e.Line, e.Col)
	}
	if l := item.Get("after").Range.Start.Line; l != 8 {
		t.Errorf("after on line %d, want 8", l)
	}
}

// Regression: goccy shifts a plain scalar's start column right by the
// number of trailing spaces after it (found on real playbooks).
func TestTrailingSpaces(t *testing.T) {
	src := "no_log: true    \nwhen:\n  - a is defined \n"
	root := Parse([]byte(src)).Docs[0].Root
	if s := root.Get("no_log").Range.Start; s.Col != 9 {
		t.Errorf("no_log value starts at col %d, want 9", s.Col)
	}
	item := root.Get("when").Items[0]
	if s, e := item.Range.Start, item.Range.End; s.Col != 5 || e.Col != 17 {
		t.Errorf("item spans cols %d-%d, want 5-17", s.Col, e.Col)
	}
}

func TestMultiDocument(t *testing.T) {
	src := `apiVersion: v1
kind: Service
metadata:
  name: web
---
this: is: broken
---
kind: ConfigMap
metadata:
  name: settings
`
	f := Parse([]byte(src))
	if len(f.Docs) != 3 {
		t.Fatalf("got %d docs, want 3", len(f.Docs))
	}
	names := []string{f.Docs[0].Name, f.Docs[2].Name}
	if names[0] != "Service/web" || names[1] != "ConfigMap/settings" {
		t.Errorf("names = %v", names)
	}
	if f.Docs[1].Root != nil {
		t.Error("broken document should have no root")
	}
	if f.Docs[2].Root == nil {
		t.Error("a broken document must not hide the ones after it")
	}
	if len(f.Diagnostics) != 1 || f.Diagnostics[0].Range.Start.Line != 6 {
		t.Errorf("want one diagnostic on line 6, got %+v", f.Diagnostics)
	}
}

func TestHelmTemplate(t *testing.T) {
	src, err := os.ReadFile("testdata/helm-deployment.yaml")
	if err != nil {
		t.Fatal(err)
	}
	f := Parse(src)
	if len(f.Diagnostics) > 0 {
		t.Fatalf("a Helm template should parse via masking, got %+v", f.Diagnostics)
	}
	if !f.HasTemplates() {
		t.Error("expected template expressions")
	}

	root := f.Docs[0].Root
	name := root.Get("metadata").Get("name")
	if !name.Templated || name.Value != `{{ include "app.fullname" . }}` {
		t.Errorf("metadata.name = %q (templated %v)", name.Value, name.Templated)
	}
	image := findKey(root, "image").Value
	want := `{{ .Values.image.repository }}:{{ .Values.image.tag | default .Chart.AppVersion }}`
	if image.Value != want || image.Style != StyleDouble {
		t.Errorf("image = %q (%s)", image.Value, image.Style)
	}
	if got := f.Docs[0].Name; got != "Deployment" {
		t.Errorf("doc name = %q, want Deployment (name is templated)", got)
	}

	var standalone, inline int
	for _, e := range f.Expressions {
		if e.Syntax != "go-template" {
			t.Errorf("expression %q has syntax %s", e.Text, e.Syntax)
		}
		if e.Standalone {
			standalone++
		} else {
			inline++
		}
	}
	if standalone != 12 || inline != 7 {
		t.Errorf("standalone=%d inline=%d, want 12 and 7", standalone, inline)
	}
}

func TestExpressionSyntax(t *testing.T) {
	tests := []struct {
		src    string
		syntax string
		phase  Phase
	}{
		{"name: {{ .Values.name }}", "go-template", PhaseTemplate},
		{`args: ["{{inputs.parameters.env}}"]`, "argo", PhaseRuntime},
		{"x: '{{workflow.name}}'", "argo", PhaseRuntime},
		{"if: ${{ github.event_name == 'push' }}", "github", PhaseRuntime},
		{"{% if x %}\nname: {{ var }}\n{% endif %}", "jinja", PhaseTemplate},
		{`msg: {{ "}}" }}`, "go-template", PhaseTemplate},
	}
	for _, tt := range tests {
		t.Run(tt.src, func(t *testing.T) {
			f := Parse([]byte(tt.src))
			if len(f.Expressions) == 0 {
				t.Fatal("no expressions found")
			}
			last := f.Expressions[len(f.Expressions)-1]
			if tt.syntax == "jinja" {
				last = f.Expressions[1] // the inline {{ var }}
			}
			if last.Syntax != tt.syntax || last.Phase != tt.phase {
				t.Errorf("got %s/%s, want %s/%s (%q)", last.Syntax, last.Phase, tt.syntax, tt.phase, last.Text)
			}
		})
	}
}

func TestPlainTags(t *testing.T) {
	tests := map[string]string{
		"": TagNull, "~": TagNull, "null": TagNull,
		"true": TagBool, "False": TagBool,
		"yes": TagStr, "on": TagStr, // YAML 1.2: not booleans
		"42": TagInt, "-7": TagInt, "0x1F": TagInt, "0o17": TagInt,
		"1.5": TagFloat, ".5": TagFloat, "1e3": TagFloat, ".inf": TagFloat,
		"1.2.3": TagStr, "nginx": TagStr, "012a": TagStr,
	}
	for in, want := range tests {
		if got := resolvePlain(in); got != want {
			t.Errorf("resolvePlain(%q) = %s, want %s", in, got, want)
		}
	}
}

func TestAnchorsAndTags(t *testing.T) {
	f := Parse([]byte("base: &b\n  x: 1\nuse: *b\nref: !Ref thing\n"))
	root := f.Docs[0].Root
	if a := root.Get("base").Anchor; a != "b" {
		t.Errorf("anchor = %q", a)
	}
	if u := root.Get("use"); u.Kind != KindAlias || u.Value != "b" {
		t.Errorf("alias = %+v", u)
	}
	if r := root.Get("ref"); r.Tag != "!Ref" || r.Value != "thing" {
		t.Errorf("tagged = %+v", r)
	}
}

func TestEmptyAndCommentOnly(t *testing.T) {
	for _, src := range []string{"", "# just a comment\n", "---\n---\n"} {
		f := Parse([]byte(src))
		if len(f.Docs) != 0 || len(f.Diagnostics) != 0 {
			t.Errorf("%q: docs=%d diags=%v", src, len(f.Docs), f.Diagnostics)
		}
	}
	if !strings.Contains(ErrTemplated.Error(), "template") {
		t.Error("ErrTemplated should mention templates")
	}
}
