package yamlkit

import "testing"

const deploy = `apiVersion: apps/v1
kind: Deployment
metadata:
  labels:
    app.kubernetes.io/name: api
spec:
  template:
    spec:
      containers:
        - name: api
          image: nginx:1.27
          env:
            - name: MODE
              value: fast
        - name: sidecar
          ports: [80, 443]
`

func TestPathAt(t *testing.T) {
	doc := Parse([]byte(deploy)).Docs[0]
	tests := []struct {
		name      string
		line, col int
		want      string
	}{
		{"top-level key", 1, 3, "apiVersion"},
		{"indentation belongs to the entry", 5, 1, `metadata.labels["app.kubernetes.io/name"]`},
		{"on a list item's dash", 10, 9, "spec.template.spec.containers[0].name"},
		{"scalar value", 11, 20, "spec.template.spec.containers[0].image"},
		{"nested list", 14, 20, "spec.template.spec.containers[0].env[0].value"},
		{"second item", 15, 15, "spec.template.spec.containers[1].name"},
		{"flow sequence element", 16, 23, "spec.template.spec.containers[1].ports[1]"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path, _ := doc.PathAt(Pos{Line: tt.line, Col: tt.col})
			if got := path.String(); got != tt.want {
				t.Errorf("PathAt(%d:%d) = %q, want %q", tt.line, tt.col, got, tt.want)
			}
		})
	}
}

func TestPathFormats(t *testing.T) {
	p := Path{{Key: "metadata"}, {Key: "labels"}, {Key: "app.kubernetes.io/name"}}
	q := Path{{Key: "image"}, {Key: "tags"}, {Index: 0, IsIndex: true}}
	tests := []struct {
		path  Path
		style PathStyle
		want  string
	}{
		{p, PathDot, `metadata.labels["app.kubernetes.io/name"]`},
		{p, PathYQ, `.metadata.labels["app.kubernetes.io/name"]`},
		{p, PathJSONPath, `$.metadata.labels['app.kubernetes.io/name']`},
		{p, PathHelm, `(index .Values.metadata.labels "app.kubernetes.io/name")`},
		{p, PathSet, `metadata.labels.app\.kubernetes\.io/name=`},
		{q, PathDot, "image.tags[0]"},
		{q, PathYQ, ".image.tags[0]"},
		{q, PathHelm, "(index .Values.image.tags 0)"},
		{q, PathSet, "image.tags[0]="},
		{Path{{Key: "my-key"}}, PathYQ, `.["my-key"]`},
		{Path{{Key: "my-key"}}, PathDot, "my-key"},
	}
	for _, tt := range tests {
		t.Run(string(tt.style)+" "+tt.want, func(t *testing.T) {
			if got := tt.path.Format(tt.style); got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestParsePathRoundTrip(t *testing.T) {
	for _, s := range []string{
		"spec.containers[0].image",
		`metadata.labels["app.kubernetes.io/name"]`,
		"a[1][2].b",
	} {
		p, err := ParsePath(s)
		if err != nil {
			t.Fatalf("ParsePath(%q): %v", s, err)
		}
		if got := p.String(); got != s {
			t.Errorf("round trip %q → %q", s, got)
		}
	}
	if p, _ := ParsePath("$.a['b.c']"); p.String() != `a["b.c"]` {
		t.Errorf("JSONPath input parsed to %q", p.String())
	}
	if _, err := ParsePath("a[x"); err == nil {
		t.Error("expected an error for a malformed path")
	}
}

func TestNodeAt(t *testing.T) {
	doc := Parse([]byte(deploy)).Docs[0]
	p, _ := ParsePath("spec.template.spec.containers[1].ports[0]")
	if n := doc.NodeAt(p); n == nil || n.Value != "80" || n.Tag != TagInt {
		t.Errorf("NodeAt = %+v", n)
	}
	missing, _ := ParsePath("spec.nope[3]")
	if doc.NodeAt(missing) != nil {
		t.Error("missing path should give nil")
	}
}
