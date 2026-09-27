package yamlkit

import (
	"errors"
	"strings"
	"testing"
)

func TestFormat(t *testing.T) {
	tests := []struct {
		name string
		src  string
		opts FormatOptions
		want string
	}{
		{
			name: "re-indents and keeps comments",
			src:  "a:\n    b: 1   # keep\n    # about c\n    c:\n        - x\n",
			want: "a:\n  b: 1 # keep\n  # about c\n  c:\n    - x\n",
		},
		{
			name: "kubernetes key order at the top only",
			src:  "spec:\n  z: 1\n  a: 2\nkind: Pod\napiVersion: v1\n",
			opts: FormatOptions{KubernetesOrder: true},
			want: "apiVersion: v1\nkind: Pod\nspec:\n  z: 1\n  a: 2\n",
		},
		{
			name: "sort keys everywhere",
			src:  "b:\n  y: 1\n  x: 2\na: 3\n",
			opts: FormatOptions{SortKeys: true},
			want: "a: 3\nb:\n  x: 2\n  y: 1\n",
		},
		{
			name: "multi-document and long strings",
			src:  "a: " + strings.Repeat("word ", 30) + "end\n---\nb: 2\n",
			want: "a: " + strings.Repeat("word ", 30) + "end\n---\nb: 2\n",
		},
		{
			// Regression: yaml.v3 re-emitted merge keys as "!!merge <<:".
			name: "merge keys stay plain",
			src:  "d: &d\n    a: 1\ne:\n    <<: *d\n",
			want: "d: &d\n  a: 1\ne:\n  <<: *d\n",
		},
		{
			name: "keeps CRLF",
			src:  "a:\r\n    b: 1\r\n",
			want: "a:\r\n  b: 1\r\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Format([]byte(tt.src), tt.opts)
			if err != nil {
				t.Fatal(err)
			}
			if string(got) != tt.want {
				t.Errorf("got:\n%s\nwant:\n%s", got, tt.want)
			}
		})
	}
}

func TestFormatRefuses(t *testing.T) {
	if _, err := Format([]byte("name: {{ .Values.name }}\n"), FormatOptions{}); !errors.Is(err, ErrTemplated) {
		t.Errorf("templates: err = %v, want ErrTemplated", err)
	}
	_, err := Format([]byte("a:\n\tb: 1\n"), FormatOptions{})
	var e *Error
	if !errors.As(err, &e) || e.Code != "tab-indent" {
		t.Errorf("invalid YAML: err = %v, want a tab-indent *Error", err)
	}
}

func TestToJSON(t *testing.T) {
	src := `name: api
replicas: 3
hex: 0x1F
ratio: .5
enabled: true
nothing: ~
version: "1.10"
yes: on
list: [a, 1]
empty: {}
`
	f := Parse([]byte(src))
	got, err := ToJSON(f, "")
	if err != nil {
		t.Fatal(err)
	}
	want := `{"name":"api","replicas":3,"hex":31,"ratio":0.5,"enabled":true,"nothing":null,"version":"1.10","yes":"on","list":["a",1],"empty":{}}` + "\n"
	if string(got) != want {
		t.Errorf("got  %s\nwant %s", got, want)
	}

	multi, _ := ToJSON(Parse([]byte("a: 1\n---\nb: 2\n")), "")
	if string(multi) != `[{"a":1},{"b":2}]`+"\n" {
		t.Errorf("multi-doc = %s", multi)
	}
}

func TestMergeKeys(t *testing.T) {
	src := `defaults: &defaults
  image: nginx
  replicas: 1
prod:
  <<: *defaults
  replicas: 3
`
	got, err := ToJSON(Parse([]byte(src)), "")
	if err != nil {
		t.Fatal(err)
	}
	// Explicit keys win over merged ones, wherever they appear.
	want := `{"defaults":{"image":"nginx","replicas":1},"prod":{"image":"nginx","replicas":3}}` + "\n"
	if string(got) != want {
		t.Errorf("got  %s\nwant %s", got, want)
	}

	if _, err := ToJSON(Parse([]byte("a: *nowhere\n")), ""); err == nil {
		t.Error("an undefined alias should be an error")
	}
}

func TestFromJSON(t *testing.T) {
	got, err := FromJSON([]byte(`{"b": 1, "a": {"version": "1.10", "on": true, "list": ["x", 2]}}`))
	if err != nil {
		t.Fatal(err)
	}
	want := "b: 1\na:\n  version: \"1.10\"\n  on: true\n  list:\n    - x\n    - 2\n"
	if string(got) != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
	if _, err := FromJSON([]byte("{\n  \"a\": ,\n}")); err == nil || !strings.Contains(err.Error(), "line 2") {
		t.Errorf("invalid JSON error should carry a position, got %v", err)
	}
}

func TestFlattenRoundTrip(t *testing.T) {
	src := `image:
  repository: nginx
  tag: "1.27"
ports: [80, 443]
labels:
  app.kubernetes.io/name: api
note: "a: b"
empty: []
`
	flat, err := FlattenText(Parse([]byte(src)))
	if err != nil {
		t.Fatal(err)
	}
	want := `image.repository: nginx
image.tag: "1.27"
ports[0]: 80
ports[1]: 443
labels["app.kubernetes.io/name"]: api
note: "a: b"
empty: []
`
	if string(flat) != want {
		t.Fatalf("flatten got:\n%s\nwant:\n%s", flat, want)
	}

	back, err := Unflatten(flat)
	if err != nil {
		t.Fatal(err)
	}
	// Same data, even if the layout differs.
	a, _ := ToJSON(Parse([]byte(src)), "")
	b, _ := ToJSON(Parse(back), "")
	if string(a) != string(b) {
		t.Errorf("round trip changed the data:\n%s\nvs\n%s", a, b)
	}
}
