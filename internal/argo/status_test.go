package argo

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
)

func treeOf(r *Run) string {
	var b strings.Builder
	for _, n := range r.Nodes {
		fmt.Fprintf(&b, "%s%s %s %s\n", strings.Repeat("  ", n.Depth), n.DisplayName, n.Type, n.Phase)
	}
	return b.String()
}

func TestReadStatus(t *testing.T) {
	data, err := os.ReadFile("testdata/status.yaml")
	if err != nil {
		t.Fatal(err)
	}
	r, err := ReadStatus(data)
	if err != nil {
		t.Fatal(err)
	}
	if r.Name != "release-qx7vb" || r.Phase != "Failed" || r.FinishedAt.Sub(r.StartedAt).Seconds() != 30 {
		t.Errorf("run %+v", r)
	}
	// Each node once, under its first parent, in the order listed.
	want := `release-qx7vb DAG Failed
  fetch Pod Succeeded
    migrate Pod Succeeded
      deploy Retry Failed
        deploy(0) Pod Failed
        deploy(1) Pod Failed
          notify Pod Omitted
`
	if got := treeOf(r); got != want {
		t.Errorf("tree:\n%s", got)
	}
	byName := map[string]RunNode{}
	for _, n := range r.Nodes {
		byName[n.DisplayName] = n
	}
	if n := byName["deploy(1)"]; n.ExitCode != "137" || n.Message != "OOMKilled (exit code 137)" || n.Template != "deploy-app" {
		t.Errorf("deploy(1) %+v", n)
	}
	// A template reference names the template; pods are named as Argo's
	// "v2" names them; a Retry node has no pod.
	if n := byName["migrate"]; n.Template != "migrate-db" || !strings.HasPrefix(n.Pod, "release-qx7vb-migrate-db-") {
		t.Errorf("migrate %+v", n)
	}
	if byName["deploy"].Pod != "" || byName["fetch"].Pod != podName("release-qx7vb", byName["fetch"], true) {
		t.Errorf("pods: %q %q", byName["deploy"].Pod, byName["fetch"].Pod)
	}

	// JSON, and a bare status, read the same; v1 pod names are node IDs.
	j := `{"metadata":{"name":"w","annotations":{"workflows.argoproj.io/pod-name-format":"v1"}},"status":{"phase":"Succeeded","nodes":{"w":{"id":"w","name":"w","type":"Steps","children":["w-1"]},"w-1":{"id":"w-1","name":"w[0].a","displayName":"a","type":"Pod","templateName":"a","phase":"Succeeded"}}}}`
	r, err = ReadStatus([]byte(j))
	if err != nil || len(r.Nodes) != 2 || r.Nodes[1].Pod != "w-1" {
		t.Errorf("json: %v %+v", err, r)
	}
	r, err = ReadStatus([]byte("nodes:\n  w: {id: w, name: w, type: Steps, children: [w-1]}\n  w-1: {id: w-1, name: \"w[0].a\", type: Pod, templateName: a}\n"))
	if err != nil || r.Name != "w" || len(r.Nodes) != 2 || r.Nodes[1].DisplayName != "w[0].a" {
		t.Errorf("bare status: %v %+v", err, r)
	}

	for _, s := range []string{"", "kind: Deployment\nmetadata: {name: x}\n", "not: [valid"} {
		if _, err := ReadStatus([]byte(s)); !errors.Is(err, ErrNoStatus) {
			t.Errorf("%q: %v", s, err)
		}
	}
}

func TestPodName(t *testing.T) {
	n := RunNode{ID: "wf-123", Name: "wf.build", Template: "build"}
	got := podName("wf", n, true)
	if !strings.HasPrefix(got, "wf-build-") || podName("wf", n, false) != "wf-123" || podName("wf", RunNode{Name: "wf"}, true) != "wf" {
		t.Errorf("pod names: %s", got)
	}
}
