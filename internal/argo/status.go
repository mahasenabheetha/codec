package argo

import (
	"errors"
	"fmt"
	"hash/fnv"
	"slices"
	"strings"
	"time"

	"github.com/mahasenabheetha/codec/v2/internal/yamlkit"
)

// A workflow's status, as `argo get <wf> -o yaml|json` or `kubectl get
// wf <wf> -o yaml` prints it: the run's step tree with each node's
// phase, times, exit code and message, for reading beside its logs
// (decision 87). Steps that failed before they logged still show.

// Run is a workflow run's status.
type Run struct {
	Name       string    `json:"name"`
	Phase      string    `json:"phase,omitempty"` // Running, Succeeded, Failed, Error
	StartedAt  time.Time `json:"startedAt,omitzero"`
	FinishedAt time.Time `json:"finishedAt,omitzero"`
	Message    string    `json:"message,omitempty"`
	// Nodes in tree order, each after its parent; Depth says how deep.
	Nodes []RunNode `json:"nodes"`
}

// RunNode is one node of the step tree.
type RunNode struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`        // full: "wf.deploy(0:eu)"
	DisplayName string    `json:"displayName"` // "deploy(0:eu)"
	Type        string    `json:"type"`        // Pod, Steps, StepGroup, DAG, TaskGroup, Retry, Skipped, Suspend
	Template    string    `json:"template,omitempty"`
	Phase       string    `json:"phase,omitempty"` // Pending, Running, Succeeded, Failed, Error, Skipped, Omitted
	StartedAt   time.Time `json:"startedAt,omitzero"`
	FinishedAt  time.Time `json:"finishedAt,omitzero"`
	Message     string    `json:"message,omitempty"`
	ExitCode    string    `json:"exitCode,omitempty"`
	Pod         string    `json:"pod,omitempty"` // for a Pod node: its pod's name
	Depth       int       `json:"depth"`
}

// ErrNoStatus means the text holds no workflow status.
var ErrNoStatus = errors.New("no workflow status here: give the output of `argo get <workflow> -o yaml` (or -o json)")

// ReadStatus reads a workflow's status from YAML or JSON: a whole
// Workflow, a List of them (the first with a status), or just its
// status. Nodes are linked into a tree from the workflow's own node.
func ReadStatus(content []byte) (*Run, error) {
	f := yamlkit.Parse(content)
	for _, d := range f.Docs {
		if d == nil || d.Root == nil {
			continue
		}
		for _, wf := range candidates(d.Root) {
			if run := readRun(wf); run != nil {
				return run, nil
			}
		}
	}
	if f.HasErrors() {
		return nil, fmt.Errorf("%w (the text doesn't read as YAML or JSON)", ErrNoStatus)
	}
	return nil, ErrNoStatus
}

// candidates are the mappings that may hold a status: the document, a
// List's items, or the document as a bare status.
func candidates(root *yamlkit.Node) []*yamlkit.Node {
	if items := root.Get("items"); items != nil && items.Kind == yamlkit.KindSeq {
		return items.Items
	}
	return []*yamlkit.Node{root}
}

func readRun(wf *yamlkit.Node) *Run {
	status := wf.Get("status")
	if status == nil {
		status = wf // a bare status
	}
	nodes := status.Get("nodes")
	if nodes == nil || nodes.Kind != yamlkit.KindMap || len(nodes.Pairs) == 0 {
		return nil
	}
	run := &Run{
		Name:       wf.Get("metadata").Get("name").Str(),
		Phase:      status.Get("phase").Str(),
		StartedAt:  when(status.Get("startedAt")),
		FinishedAt: when(status.Get("finishedAt")),
		Message:    status.Get("message").Str(),
	}
	v2 := wf.Get("metadata").Get("annotations").Get("workflows.argoproj.io/pod-name-format").Str() != "v1"

	type raw struct {
		node     RunNode
		children []string
	}
	byID := map[string]*raw{}
	var order []string
	for _, p := range nodes.Pairs {
		n := p.Value
		if n == nil || n.Kind != yamlkit.KindMap {
			continue
		}
		id := n.Get("id").Str()
		if id == "" {
			id = p.Key.Value
		}
		r := &raw{node: RunNode{
			ID: id, Name: n.Get("name").Str(), DisplayName: n.Get("displayName").Str(), Type: n.Get("type").Str(),
			Template: n.Get("templateName").Str(), Phase: n.Get("phase").Str(), Message: n.Get("message").Str(),
			StartedAt: when(n.Get("startedAt")), FinishedAt: when(n.Get("finishedAt")),
			ExitCode: n.Get("outputs").Get("exitCode").Str(),
		}}
		if r.node.Template == "" {
			r.node.Template = n.Get("templateRef").Get("template").Str()
		}
		if r.node.DisplayName == "" {
			r.node.DisplayName = r.node.Name
		}
		if c := n.Get("children"); c != nil {
			for _, it := range c.Items {
				r.children = append(r.children, it.Str())
			}
		}
		byID[id] = r
		order = append(order, id)
	}
	if run.Name == "" {
		// A bare status: the workflow's own node is the one nobody
		// points at; its name is the workflow's.
		pointed := map[string]bool{}
		for _, r := range byID {
			for _, c := range r.children {
				pointed[c] = true
			}
		}
		for _, id := range order {
			if !pointed[id] {
				run.Name = byID[id].node.Name
				break
			}
		}
	}
	for _, r := range byID {
		if r.node.Type == "Pod" {
			r.node.Pod = podName(run.Name, r.node, v2)
		}
	}

	// Depth first from the roots (the workflow's node, then any node
	// nothing points at), children in the order listed, each node once.
	pointed := map[string]bool{}
	for _, r := range byID {
		for _, c := range r.children {
			pointed[c] = true
		}
	}
	seen := map[string]bool{}
	var walk func(id string, depth int)
	walk = func(id string, depth int) {
		r := byID[id]
		if r == nil || seen[id] {
			return
		}
		seen[id] = true
		r.node.Depth = depth
		run.Nodes = append(run.Nodes, r.node)
		for _, c := range r.children {
			walk(c, depth+1)
		}
	}
	roots := slices.DeleteFunc(slices.Clone(order), func(id string) bool { return pointed[id] })
	slices.SortStableFunc(roots, func(a, b string) int { // the workflow's own node first
		return boolCmp(byID[b].node.Name == run.Name, byID[a].node.Name == run.Name)
	})
	for _, id := range roots {
		walk(id, 0)
	}
	for _, id := range order { // nodes only reachable through a cycle
		walk(id, 0)
	}
	return run
}

func boolCmp(a, b bool) int {
	switch {
	case a == b:
		return 0
	case a:
		return 1
	}
	return -1
}

func when(n *yamlkit.Node) time.Time {
	t, _ := time.Parse(time.RFC3339, n.Str())
	return t
}

// podName is the pod Argo made for a Pod node. Pod names "v2" (the
// default since Argo 3.4): "<workflow>-<template>-<fnv32a of the node's
// name>"; "v1": the node's ID.
func podName(workflow string, n RunNode, v2 bool) string {
	if !v2 || workflow == "" {
		return n.ID
	}
	if workflow == n.Name {
		return workflow
	}
	prefix := workflow
	if n.Template != "" {
		prefix += "-" + n.Template
	}
	if len(prefix) > 242 { // 253, less the hash and its dash
		prefix = prefix[:242]
	}
	h := fnv.New32a()
	h.Write([]byte(n.Name))
	return fmt.Sprintf("%s-%d", strings.TrimRight(prefix, "-"), h.Sum32())
}
