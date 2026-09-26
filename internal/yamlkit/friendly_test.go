package yamlkit

import "testing"

func TestFriendlyErrors(t *testing.T) {
	tests := []struct {
		name     string
		src      string
		wantCode string
		wantLine int
		wantCol  int
	}{
		{"tab indentation", "a:\n\tb: 1\n", "tab-indent", 2, 1},
		{"tab after spaces", "a:\n  b:\n  \tc: 1\n", "tab-indent", 3, 3},
		{"duplicate key", "a: 1\nb: 2\na: 3\n", "duplicate-key", 3, 1},
		{"nested duplicate key", "spec:\n  x: 1\n  x: 2\n", "duplicate-key", 3, 3},
		{"bad indentation", "a:\n  b: 1\n c: 2\n", "bad-indent", 3, 1},
		{"unquoted colon", "msg: error: bad thing\n", "unquoted-colon", 1, 11},
		{"unquoted colon in list", "- name: step: one\n", "unquoted-colon", 1, 13},
		{"nested under a value", "app:\n  name: demo\n    port: 80\n", "nested-under-value", 3, 1},
		{"nested under a top-level value", "a: 1\n  b: 2\n", "nested-under-value", 2, 1},
		{"unclosed double quote", "a: \"hello\nb: 2\n", "unclosed-quote", 1, 4},
		{"unclosed single quote", "a: 'hello\nb: 2\n", "unclosed-quote", 1, 4},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f := Parse([]byte(tt.src))
			if len(f.Diagnostics) == 0 {
				t.Fatal("expected a diagnostic")
			}
			d := f.Diagnostics[0]
			if d.Code != tt.wantCode {
				t.Fatalf("code = %q (%s), want %q", d.Code, d.Message, tt.wantCode)
			}
			if s := d.Range.Start; s.Line != tt.wantLine || s.Col != tt.wantCol {
				t.Errorf("at %d:%d, want %d:%d", s.Line, s.Col, tt.wantLine, tt.wantCol)
			}
			if d.Message == "" || d.Hint == "" {
				t.Errorf("friendly errors need a message and a hint: %+v", d)
			}
		})
	}
}

// Regression: whole-line template expressions inside block scalars were
// masked to spaces, leaving over-indented blank lines (invalid YAML).
// Patterns taken from real Ansible playbooks.
func TestTemplatesInsideBlockScalars(t *testing.T) {
	tests := map[string]string{
		"multi-line {{ }} in >-": `- name: pick map
  set_fact:
    versions_map: >-
      {{
        (versions | from_json)
        if (versions is string)
        else (versions | default({}))
      }}

- name: next
  debug: {}
`,
		"jinja {% %} lines in >-": `- name: status
  set_fact:
    status: >-
      {%- if logs | length == 0 -%}
        No log found
      {%- else -%}
        Done
      {%- endif -%}
`,
	}
	for name, src := range tests {
		t.Run(name, func(t *testing.T) {
			f := Parse([]byte(src))
			if len(f.Diagnostics) != 0 {
				t.Fatalf("unexpected diagnostics: %+v", f.Diagnostics)
			}
			if len(f.Docs) != 1 || f.Docs[0].Root.Kind != KindSeq {
				t.Fatalf("expected one list document")
			}
		})
	}
}

// Template control flow produces "duplicate" keys that aren't bugs.
func TestNoDuplicateKeyInTemplateBranches(t *testing.T) {
	src := "spec:\n{{- if .Values.a }}\n  mode: fast\n{{- else }}\n  mode: safe\n{{- end }}\n"
	if f := Parse([]byte(src)); len(f.Diagnostics) != 0 {
		t.Errorf("unexpected diagnostics: %+v", f.Diagnostics)
	}
}
