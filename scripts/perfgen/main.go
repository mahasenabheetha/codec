// Command perfgen writes the performance fixtures scripts/perf.sh times:
// a 10k-file repo, a 5 MB YAML file and a chart that renders 500
// documents (budgets: design/architecture.md, "Performance").
//
//	go run ./scripts/perfgen DIR
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func must(err error) {
	if err != nil {
		panic(err)
	}
}

func write(p, s string) {
	must(os.MkdirAll(filepath.Dir(p), 0o755))
	must(os.WriteFile(p, []byte(s), 0o644))
}

func deployment(name string, i int) string {
	return fmt.Sprintf(`apiVersion: apps/v1
kind: Deployment
metadata:
  name: %[1]s
  labels:
    app.kubernetes.io/name: %[1]s
    team: team-%[2]d
spec:
  replicas: %[3]d
  selector:
    matchLabels:
      app.kubernetes.io/name: %[1]s
  template:
    metadata:
      labels:
        app.kubernetes.io/name: %[1]s
    spec:
      securityContext:
        runAsNonRoot: true
      containers:
        - name: app
          image: registry.example.com/%[1]s:1.%[2]d.0
          ports:
            - name: http
              containerPort: 8080
          env:
            - name: LOG_LEVEL
              value: info
            - name: SERVICE_NAME
              value: %[1]s
          readinessProbe:
            httpGet:
              path: /healthz
              port: http
          resources:
            requests:
              cpu: 100m
              memory: 128Mi
            limits:
              memory: 256Mi
`, name, i%50, 1+i%5)
}

func service(name string) string {
	return fmt.Sprintf(`apiVersion: v1
kind: Service
metadata:
  name: %[1]s
spec:
  selector:
    app.kubernetes.io/name: %[1]s
  ports:
    - name: http
      port: 80
      targetPort: http
`, name)
}

func main() {
	root := os.Args[1]

	// 10k files: 1,000 services × 10 files (6 YAML, 4 other).
	repo := filepath.Join(root, "repo10k")
	for i := range 1000 {
		name := fmt.Sprintf("svc-%04d", i)
		d := filepath.Join(repo, "services", fmt.Sprintf("group-%02d", i/40), name)
		write(filepath.Join(d, "deploy", "deployment.yaml"), deployment(name, i))
		write(filepath.Join(d, "deploy", "service.yaml"), service(name))
		write(filepath.Join(d, "deploy", "kustomization.yaml"), "resources:\n  - deployment.yaml\n  - service.yaml\n")
		write(filepath.Join(d, "config.yaml"), fmt.Sprintf("name: %s\nfeatures:\n  a: true\n  b: false\nlimits:\n  rps: %d\n", name, i))
		write(filepath.Join(d, ".github", "workflows", "ci.yml"), "on: [push]\njobs:\n  test:\n    runs-on: ubuntu-latest\n    steps:\n      - uses: actions/checkout@v5\n      - run: make test\n")
		write(filepath.Join(d, "compose.yaml"), fmt.Sprintf("services:\n  %s:\n    image: %s:dev\n    ports:\n      - \"8080\"\n", name, name))
		write(filepath.Join(d, "README.md"), "# "+name+"\n\nA service.\n")
		write(filepath.Join(d, "main.go"), "package main\n\nfunc main() {}\n")
		write(filepath.Join(d, "go.mod"), "module example.com/"+name+"\n\ngo 1.26\n")
		write(filepath.Join(d, "Makefile"), "test:\n\tgo test ./...\n")
	}

	// A 5 MB file: Deployments and Services in one multi-document file.
	var b strings.Builder
	for i := 0; b.Len() < 5<<20; i++ {
		name := fmt.Sprintf("big-%05d", i)
		if i > 0 {
			b.WriteString("---\n")
		}
		b.WriteString(deployment(name, i))
		b.WriteString("---\n")
		b.WriteString(service(name))
	}
	write(filepath.Join(root, "big", "big.yaml"), b.String())

	// A chart rendering 500 documents: a Deployment and a Service for
	// each of 250 values entries.
	chart := filepath.Join(root, "chart500", "many")
	write(filepath.Join(chart, "Chart.yaml"), "apiVersion: v2\nname: many\nversion: 1.0.0\n")
	var v strings.Builder
	v.WriteString("image: registry.example.com/app\nservices:\n")
	for i := range 250 {
		fmt.Fprintf(&v, "  - name: app-%03d\n    replicas: %d\n    port: %d\n", i, 1+i%3, 8000+i)
	}
	write(filepath.Join(chart, "values.yaml"), v.String())
	write(filepath.Join(chart, "templates", "apps.yaml"), `{{- range .Values.services }}
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: {{ .name }}
  labels:
    app.kubernetes.io/name: {{ .name }}
    app.kubernetes.io/instance: {{ $.Release.Name }}
spec:
  replicas: {{ .replicas }}
  selector:
    matchLabels:
      app.kubernetes.io/name: {{ .name }}
  template:
    metadata:
      labels:
        app.kubernetes.io/name: {{ .name }}
    spec:
      containers:
        - name: app
          image: "{{ $.Values.image }}:{{ $.Chart.Version }}"
          ports:
            - name: http
              containerPort: {{ .port }}
          resources:
            requests: {cpu: 50m, memory: 64Mi}
            limits: {memory: 128Mi}
---
apiVersion: v1
kind: Service
metadata:
  name: {{ .name }}
spec:
  selector:
    app.kubernetes.io/name: {{ .name }}
  ports:
    - name: http
      port: 80
      targetPort: http
{{- end }}
`)
	fmt.Println("done")
}
