package scaffold

import (
	"os"
	"strings"
	"testing"

	"github.com/mahasenabheetha/codec/v2/internal/yamlkit"
)

func read(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// The acceptance check: a cloned WorkflowTemplate keeps nothing of the
// old name except what is listed for review (its image).
func TestCloneWorkflowTemplate(t *testing.T) {
	src := read(t, "workflowtemplate.yaml")
	res, err := Clone(src, CloneOptions{Doc: -1, To: "build-web"})
	if err != nil {
		t.Fatal(err)
	}
	if res.From != "build-api" {
		t.Errorf("from = %q", res.From)
	}
	var left []string
	for i, line := range strings.Split(res.Text, "\n") {
		if len(wordHits(line, "build-api")) > 0 && !strings.HasPrefix(strings.TrimSpace(line), "#") {
			left = append(left, strings.TrimSpace(line))
			if !suggested(res.Changes, i+1) {
				t.Errorf("line %d still says build-api and isn't listed for review: %s", i+1, line)
			}
		}
	}
	// The repository URL and the image are other things' names.
	if len(left) != 2 {
		t.Errorf("left for review: %q", left)
	}
	for _, want := range []string{
		"name: build-web\n",
		"entrypoint: build-web-main",
		"- name: build-web-main",
		"depends: clone-build-web",
		"templateRef:\n              name: build-web",
		"{{tasks.clone-build-web.outputs.parameters.dir}}",
		`"build-web-{{workflow.uid}}"`,
		"app.kubernetes.io/name: build-web",
		"# Builds the api image.", // comments and layout are kept
		"serviceAccountName: argo-runner",
		"BUILD_API_DEBUG", // upper case is a different word
	} {
		if !strings.Contains(res.Text, want) {
			t.Errorf("missing %q in:\n%s", want, res.Text)
		}
	}
	if f := yamlkit.Parse([]byte(res.Text)); f.HasErrors() {
		t.Errorf("result doesn't parse: %v", f.Diagnostics)
	}

	// Taking both suggestions leaves no trace of the old name.
	var flip []string
	for _, c := range res.Changes {
		if c.Kind == "suggest" {
			flip = append(flip, c.ID)
		}
	}
	res, _ = Clone(src, CloneOptions{Doc: -1, To: "build-web", Flip: flip})
	body := strings.SplitN(res.Text, "\n", 2)[1]
	if strings.Contains(body, "build-api") {
		t.Errorf("suggestions taken, still has build-api:\n%s", body)
	}
}

func suggested(cs []Change, line int) bool {
	for _, c := range cs {
		if c.Line == line && c.Kind == "suggest" && !c.Applied {
			return true
		}
	}
	return false
}

func TestCloneReferences(t *testing.T) {
	src := read(t, "app.yaml")

	// One document: references to the other documents are suggestions.
	res, err := Clone(src, CloneOptions{Doc: 1, To: "cart"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(res.Text, "apiVersion: apps/v1\nkind: Deployment\nmetadata:\n  name: cart\n") || strings.Contains(res.Text, "kind: Service") {
		t.Errorf("doc 1:\n%s", res.Text)
	}
	byPath := map[string]Change{}
	for _, c := range res.Changes {
		byPath[c.Path] = c
	}
	for path, want := range map[string]string{
		"metadata.name":                                                       "rename",
		"spec.template.spec.containers[0].name":                               "rename",
		"spec.selector.matchLabels.app":                                       "rename",
		"spec.template.spec.serviceAccountName":                               "suggest",
		"spec.template.spec.containers[0].envFrom[0].configMapRef.name":       "suggest",
		"spec.template.spec.containers[0].envFrom[1].secretRef.name":          "suggest",
		"spec.template.spec.containers[0].image":                              "suggest",
		"spec.template.spec.containers[0].env[0].valueFrom.secretKeyRef.name": "check",
	} {
		if byPath[path].Kind != want {
			t.Errorf("%s: %q, want %s", path, byPath[path].Kind, want)
		}
	}
	if c := byPath["spec.template.spec.containers[0].envFrom[0].configMapRef.name"]; c.After != "cart-config" || c.Applied || !strings.Contains(res.Text, "name: shop-config") {
		t.Errorf("configMapRef: %+v", c)
	}
	if c := byPath["metadata.name"]; c.Line != 4 {
		t.Errorf("metadata.name line %d", c.Line)
	}

	// The whole file: the ConfigMap is copied too, so its reference follows.
	res, _ = Clone(src, CloneOptions{Doc: -1, To: "cart"})
	for _, want := range []string{"name: cart-config", "cart.properties: |", "name=cart", "name: shop-secrets"} {
		if !strings.Contains(res.Text, want) {
			t.Errorf("whole file: missing %q", want)
		}
	}

	// Leaving a rename out.
	var id string
	for _, c := range res.Changes {
		if c.Path == "data.shop.properties" && c.Key {
			id = c.ID
		}
	}
	res, _ = Clone(src, CloneOptions{Doc: -1, To: "cart", Flip: []string{id}})
	if !strings.Contains(res.Text, "shop.properties: |") || !strings.Contains(res.Text, "name=cart") {
		t.Errorf("flipped key:\n%s", res.Text)
	}
}

func TestCloneErrors(t *testing.T) {
	for _, tc := range []struct {
		src  string
		opts CloneOptions
		want string
	}{
		{"", CloneOptions{To: "x"}, "empty"},
		{"a: 1\n", CloneOptions{To: "x"}, "no name found"},
		{"name: a\n", CloneOptions{To: "a"}, "same"},
		{"name: a\n", CloneOptions{To: " "}, "new name"},
		{"name: a\n", CloneOptions{Doc: 3, To: "b"}, "1 documents"},
	} {
		if _, err := Clone([]byte(tc.src), tc.opts); err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%q %+v: %v, want %q", tc.src, tc.opts, err, tc.want)
		}
	}
	// A top-level name and an explicit From work without metadata.
	res, err := Clone([]byte("name: ci\njobs:\n  ci-test:\n    runs-on: ubuntu-latest\n"), CloneOptions{Doc: -1, To: "release"})
	if err != nil || res.Text != "name: release\njobs:\n  release-test:\n    runs-on: ubuntu-latest\n" {
		t.Errorf("%v\n%s", err, res.Text)
	}
	res, _ = Clone([]byte("build:\n  script: make\nbuild-docs:\n  needs: [build]\n"), CloneOptions{Doc: -1, From: "build", To: "pack"})
	if res.Text != "pack:\n  script: make\npack-docs:\n  needs: [pack]\n" {
		t.Errorf("from:\n%s", res.Text)
	}
}

func TestOldName(t *testing.T) {
	if got := OldName(read(t, "app.yaml"), -1); got != "shop" {
		t.Errorf("whole file: %q", got)
	}
	if got := OldName(read(t, "app.yaml"), 0); got != "shop-config" {
		t.Errorf("doc 0: %q", got)
	}
	helm := []byte("apiVersion: v1\nkind: Service\nmetadata:\n  name: {{ include \"x.fullname\" . }}\n")
	if got := OldName(helm, -1); got != "" {
		t.Errorf("templated name: %q", got)
	}
}
