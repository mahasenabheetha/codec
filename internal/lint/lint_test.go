package lint

import (
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/mahasenabheetha/codec/v2/internal/yamlkit"
)

// run lints src and returns "code:line" for each finding, sorted.
func run(t *testing.T, src, typ string, cfg Config) []string {
	t.Helper()
	f := yamlkit.Parse([]byte(src))
	var got []string
	for _, d := range Apply(append(slices.Clone(f.Diagnostics), Run(Input{YAML: f, Content: []byte(src), Type: typ}, cfg)...), cfg) {
		got = append(got, fmt.Sprintf("%s:%d", d.Code, d.Range.Start.Line))
	}
	slices.Sort(got)
	return got
}

// on turns the given rules on (as warnings) on top of the defaults.
func on(ids ...string) Config {
	c := Config{Levels: map[string]string{}}
	for _, id := range ids {
		c.Levels[id] = "warning"
	}
	return c
}

const goodPod = `apiVersion: apps/v1
kind: Deployment
metadata:
  name: api
spec:
  template:
    spec:
      securityContext:
        runAsNonRoot: true
      containers:
        - name: api
          image: nginx:1.27
          resources:
            requests: {cpu: 100m}
            limits: {memory: 128Mi}
          readinessProbe:
            httpGet: {path: /, port: 80}
`

func TestRules(t *testing.T) {
	tests := []struct {
		name string
		src  string
		typ  string
		cfg  Config
		want []string
	}{
		// --- style ---
		{"truthy values", "a: yes\nb: 'yes'\nc: true\nd: On\non: x\n", "", Config{}, []string{"truthy:1", "truthy:4"}},
		{"octal", "mode: 0755\nq: '0755'\nzero: 0\nbad: 089\nnew: 0o17\nn: 755\n", "", Config{}, []string{"octal:1", "octal:4", "octal:5"}},
		{"empty value", "a:\nb: null\nc: ~\nd: {}\n", "", Config{}, []string{"empty-value:1"}},
		{"empty value skipped in templates", "a:\n{{- toYaml .Values.x | nindent 2 }}\n", "helm-template", Config{}, nil},
		{"GitHub events may be empty", "on:\n  workflow_dispatch:\njobs:\n  a:\n    runs-on:\n", "github-actions", Config{}, []string{"empty-value:5"}},
		{"inventory hosts may be empty", "all:\n  children:\n    web:\n      hosts:\n        a.example.com:\n      vars:\n        port:\n", "ansible-inventory", Config{}, []string{"empty-value:7"}},
		{"trailing spaces", "a: 1  \nb: 2\n", "", Config{}, []string{"trailing-spaces:1"}},
		{"document start off by default", "a: 1\n", "", Config{}, nil},
		{"document start", "# comment\na: 1\n", "", on("document-start"), []string{"document-start:2"}},
		{"document start present", "---\na: 1\n", "", on("document-start"), nil},
		{"line length", "a: " + strings.TrimSpace(strings.Repeat("word ", 30)) + "\nurl: https://" + strings.Repeat("x", 130) + "\n", "", on("line-length"), []string{"line-length:1"}},
		{"line length option", "a: 1 2 3 4\n", "", Config{Levels: map[string]string{"line-length": "info"}, LineLength: 5}, []string{"line-length:1"}},
		{"indent consistent", "a:\n  b:\n    c: 1\n", "", Config{}, nil},
		{"indent mixed", "a:\n  b: 1\nc:\n    d: 1\ne:\n  f: 1\n", "", Config{}, []string{"indent:4"}},
		{"list style mixed", "a:\n- 1\nb:\n- 2\nc:\n  - 3\n", "", Config{}, []string{"indent:6"}},
		{"duplicate key level", "a: 1\na: 2\n", "", Config{Levels: map[string]string{"duplicate-key": "off"}}, nil},
		{"rule off", "a: yes\n", "", Config{Levels: map[string]string{"truthy": "off"}}, nil},

		// --- kubernetes ---
		{"good workload", goodPod, "kubernetes", Config{}, nil},
		{"image latest and untagged", strings.Replace(goodPod, "nginx:1.27", "nginx:latest", 1), "kubernetes", Config{}, []string{"image-tag:12"}},
		{"image untagged with registry port", strings.Replace(goodPod, "nginx:1.27", "reg:5000/nginx", 1), "kubernetes", Config{}, []string{"image-tag:12"}},
		{"image digest", strings.Replace(goodPod, "nginx:1.27", "nginx@sha256:abc", 1), "kubernetes", Config{}, nil},
		{"no limits", strings.Replace(goodPod, "            limits: {memory: 128Mi}\n", "", 1), "kubernetes", Config{}, []string{"resources:11"}},
		{"no probes", strings.Replace(goodPod, "          readinessProbe:\n            httpGet: {path: /, port: 80}\n", "", 1), "kubernetes", Config{}, []string{"probes:11"}},
		{"privileged, hostNetwork, hostPath", strings.Replace(goodPod, "      containers:\n", "      hostNetwork: true\n      volumes:\n        - name: d\n          hostPath: {path: /var}\n      containers:\n", 1) +
			"          securityContext:\n            privileged: true\n", "kubernetes", Config{}, []string{"host-network:10", "host-path:13", "privileged:23"}},
		{"root", strings.Replace(goodPod, "runAsNonRoot: true", "fsGroup: 1", 1), "kubernetes", Config{}, []string{"run-as-non-root:11"}},
		{"runAsUser counts as non-root", strings.Replace(goodPod, "runAsNonRoot: true", "runAsUser: 1000", 1), "kubernetes", Config{}, nil},
		{"job needs no probes", "apiVersion: batch/v1\nkind: Job\nspec:\n  template:\n    spec:\n      securityContext: {runAsNonRoot: true}\n      containers:\n        - name: j\n          image: busybox:1\n          resources: {requests: {cpu: 1}, limits: {memory: 1Gi}}\n", "kubernetes", Config{}, nil},
		{"helm test hooks are skipped", "apiVersion: v1\nkind: Pod\nmetadata:\n  annotations:\n    helm.sh/hook: test\nspec:\n  containers:\n    - name: t\n      image: busybox\n", "kubernetes", Config{}, nil},
		{"templates are skipped", strings.Replace(goodPod, "nginx:1.27", "nginx:latest", 1), "helm-template", Config{}, nil},

		// --- deprecations ---
		{"removed api", "apiVersion: extensions/v1beta1\nkind: Ingress\n", "kubernetes", Config{}, []string{"api-removed:1"}},
		{"removed after target", "apiVersion: batch/v1beta1\nkind: CronJob\n", "kubernetes", Config{K8sVersion: "1.21"}, []string{"api-deprecated:1"}},
		{"not yet deprecated for target", "apiVersion: batch/v1beta1\nkind: CronJob\n", "kubernetes", Config{K8sVersion: "1.20"}, nil},
		{"deprecated only", "apiVersion: v1\nkind: Endpoints\n", "kubernetes", Config{K8sVersion: "1.35"}, []string{"api-deprecated:1"}},
		{"current api", "apiVersion: networking.k8s.io/v1\nkind: Ingress\n", "kubernetes", Config{}, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := run(t, tt.src, tt.typ, tt.cfg)
			if !slices.Equal(got, tt.want) {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFindingExplains(t *testing.T) {
	f := yamlkit.Parse([]byte("a: yes\n"))
	ds := Run(Input{YAML: f, Content: []byte("a: yes\n")}, Config{})
	if len(ds) != 1 {
		t.Fatalf("got %d findings", len(ds))
	}
	d := ds[0]
	if d.Why == "" || d.Hint == "" || d.Source != "style" || d.Severity != yamlkit.SeverityWarning {
		t.Errorf("incomplete finding: %+v", d)
	}
	if d.Range.Start.Col != 4 || d.Range.End.Col != 7 {
		t.Errorf("range %+v, want cols 4-7", d.Range)
	}
}
