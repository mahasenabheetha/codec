package kube

import (
	"fmt"
	"slices"
	"strings"

	"github.com/mahasenabheetha/codec/v2/internal/yamlkit"
)

// Node is an object in the graph, or a referenced one that isn't in
// the scope (Missing).
type Node struct {
	ID        string  `json:"id"`
	Kind      string  `json:"kind"`
	Name      string  `json:"name"`
	Namespace string  `json:"namespace,omitempty"`
	Missing   bool    `json:"missing,omitempty"`
	Source    *Source `json:"source,omitempty"`
}

// Edge is a relationship: From uses, selects or targets To.
type Edge struct {
	From  string `json:"from"`
	To    string `json:"to"`
	Kind  string `json:"kind"`            // selects, routes, config, secret, volume, account, pulls, tls, scales, protects, grants, binds, applies
	Label string `json:"label,omitempty"` // e.g. "port 80", "envFrom"
}

// Finding is a relationship problem. References that point outside the
// scope are warnings, not errors: the target may come from another
// chart or already exist in the cluster.
type Finding struct {
	Severity string `json:"severity"` // warning, info
	Code     string `json:"code"`
	Message  string `json:"message"`
	Hint     string `json:"hint,omitempty"`
	Object   string `json:"object"`
	Source   Source `json:"source"` // Line points at the reference itself
}

// Graph is how the objects of a scope connect.
type Graph struct {
	Nodes    []Node    `json:"nodes"`
	Edges    []Edge    `json:"edges"`
	Findings []Finding `json:"findings"`
}

// Relate works out the relationships between objects.
func Relate(objs []Object) Graph {
	r := &relater{byID: map[string]*Object{}, byKindName: map[string][]*Object{}, nodes: map[string]bool{}, edges: map[string]bool{}}
	for i := range objs {
		o := &objs[i]
		r.byID[o.ID()] = o
		r.byKindName[o.Kind+"/"+o.Name] = append(r.byKindName[o.Kind+"/"+o.Name], o)
		r.g.Nodes = append(r.g.Nodes, Node{ID: o.ID(), Kind: o.Kind, Name: o.Name, Namespace: o.Namespace, Source: &o.Source})
		r.nodes[o.ID()] = true
	}
	for i := range objs {
		o := &objs[i]
		r.workload(o)
		switch o.Kind {
		case "Service":
			r.service(o, objs)
		case "Ingress":
			r.ingress(o)
		case "HorizontalPodAutoscaler":
			ref := o.Root.Get("spec").Get("scaleTargetRef")
			r.link(o, ref.Get("kind").Str(), ref.Get("name").Str(), "scales", "", ref, false)
		case "PodDisruptionBudget":
			r.selector(o, o.Root.Get("spec").Get("selector"), objs, "protects")
		case "NetworkPolicy":
			r.selector(o, o.Root.Get("spec").Get("podSelector"), objs, "applies")
		case "RoleBinding", "ClusterRoleBinding":
			r.binding(o)
		}
	}
	if r.g.Edges == nil {
		r.g.Edges = []Edge{}
	}
	if r.g.Findings == nil {
		r.g.Findings = []Finding{}
	}
	return r.g
}

type relater struct {
	g          Graph
	byID       map[string]*Object
	byKindName map[string][]*Object
	nodes      map[string]bool
	edges      map[string]bool
}

// find resolves a reference from an object in namespace ns. An unset
// namespace on either side matches (renders often leave it empty).
func (r *relater) find(kind, ns, name string) *Object {
	if clusterScoped[kind] {
		if os := r.byKindName[kind+"/"+name]; len(os) > 0 {
			return os[0]
		}
		return nil
	}
	if o := r.byID[ID(kind, ns, name)]; o != nil {
		return o
	}
	for _, o := range r.byKindName[kind+"/"+name] {
		if ns == "" || o.Namespace == "" {
			return o
		}
	}
	return nil
}

func (r *relater) edge(from, to, kind, label string) {
	key := from + "\x00" + to + "\x00" + kind
	if r.edges[key] {
		return
	}
	r.edges[key] = true
	r.g.Edges = append(r.g.Edges, Edge{From: from, To: to, Kind: kind, Label: label})
}

// link adds an edge to the referenced object, or a missing node and a
// finding when it isn't in scope (unless the reference is optional).
func (r *relater) link(from *Object, kind, name, edgeKind, label string, at *yamlkit.Node, optional bool) {
	r.linkNS(from, from.Namespace, kind, name, edgeKind, label, at, optional)
}

// linkNS is link with the namespace to look the target up in (a
// RoleBinding subject names its own).
func (r *relater) linkNS(from *Object, ns, kind, name, edgeKind, label string, at *yamlkit.Node, optional bool) {
	if kind == "" || name == "" || (at != nil && at.Templated) {
		return
	}
	if clusterScoped[kind] {
		ns = ""
	}
	if to := r.find(kind, ns, name); to != nil {
		r.edge(from.ID(), to.ID(), edgeKind, label)
		return
	}
	id := ID(kind, ns, name)
	if !r.nodes[id] {
		r.nodes[id] = true
		r.g.Nodes = append(r.g.Nodes, Node{ID: id, Kind: kind, Name: name, Namespace: ns, Missing: true})
	}
	r.edge(from.ID(), id, edgeKind, label)
	if optional || wellKnown(kind, name) {
		return
	}
	severity := "warning"
	if edgeKind == "tls" {
		severity = "info" // TLS secrets are usually made by cert-manager
	}
	r.find1(from, at, severity, "missing-ref",
		fmt.Sprintf("%s %s refers to %s %s, which isn't in this scope", from.Kind, from.Name, kind, name),
		"Fine if it is created elsewhere (another chart, an operator, or already in the cluster); otherwise check the name.")
}

// wellKnown lists objects every cluster has.
func wellKnown(kind, name string) bool {
	switch kind {
	case "ClusterRole":
		return name == "cluster-admin" || name == "admin" || name == "edit" || name == "view" || strings.HasPrefix(name, "system:")
	case "ConfigMap":
		return name == "kube-root-ca.crt"
	case "ServiceAccount":
		return name == "default"
	}
	return false
}

func (r *relater) find1(o *Object, at *yamlkit.Node, severity, code, msg, hint string) {
	src := o.Source
	if at != nil && at.Range.Start.Line > 0 {
		src.Line = at.Range.Start.Line
	}
	r.g.Findings = append(r.g.Findings, Finding{Severity: severity, Code: code, Message: msg, Hint: hint, Object: o.ID(), Source: src})
}

// workload links a pod template to what it uses.
func (r *relater) workload(o *Object) {
	tpl, _ := podTemplate(*o)
	if tpl == nil {
		return
	}
	spec := tpl.Get("spec")
	if sa := spec.Get("serviceAccountName"); sa.Str() != "" {
		r.link(o, "ServiceAccount", sa.Value, "account", "", sa, false)
	} else if sa := spec.Get("serviceAccount"); sa.Str() != "" {
		r.link(o, "ServiceAccount", sa.Value, "account", "", sa, false)
	}
	for _, s := range items(spec.Get("imagePullSecrets")) {
		r.link(o, "Secret", s.Get("name").Str(), "pulls", "imagePullSecrets", s.Get("name"), false)
	}
	for _, v := range items(spec.Get("volumes")) {
		vol := v.Get("name").Str()
		if cm := v.Get("configMap"); cm != nil {
			r.link(o, "ConfigMap", cm.Get("name").Str(), "config", "volume "+vol, cm.Get("name"), isTrue(cm.Get("optional")))
		}
		if s := v.Get("secret"); s != nil {
			r.link(o, "Secret", s.Get("secretName").Str(), "secret", "volume "+vol, s.Get("secretName"), isTrue(s.Get("optional")))
		}
		if pvc := v.Get("persistentVolumeClaim"); pvc != nil {
			r.link(o, "PersistentVolumeClaim", pvc.Get("claimName").Str(), "volume", "volume "+vol, pvc.Get("claimName"), false)
		}
		for _, src := range items(v.Get("projected").Get("sources")) {
			if cm := src.Get("configMap"); cm != nil {
				r.link(o, "ConfigMap", cm.Get("name").Str(), "config", "volume "+vol, cm.Get("name"), isTrue(cm.Get("optional")))
			}
			if s := src.Get("secret"); s != nil {
				r.link(o, "Secret", s.Get("name").Str(), "secret", "volume "+vol, s.Get("name"), isTrue(s.Get("optional")))
			}
		}
	}
	for _, c := range containers(spec) {
		for _, ef := range items(c.Get("envFrom")) {
			if cm := ef.Get("configMapRef"); cm != nil {
				r.link(o, "ConfigMap", cm.Get("name").Str(), "config", "envFrom", cm.Get("name"), isTrue(cm.Get("optional")))
			}
			if s := ef.Get("secretRef"); s != nil {
				r.link(o, "Secret", s.Get("name").Str(), "secret", "envFrom", s.Get("name"), isTrue(s.Get("optional")))
			}
		}
		for _, e := range items(c.Get("env")) {
			vf := e.Get("valueFrom")
			if k := vf.Get("configMapKeyRef"); k != nil {
				r.link(o, "ConfigMap", k.Get("name").Str(), "config", "env "+e.Get("name").Str(), k.Get("name"), isTrue(k.Get("optional")))
			}
			if k := vf.Get("secretKeyRef"); k != nil {
				r.link(o, "Secret", k.Get("name").Str(), "secret", "env "+e.Get("name").Str(), k.Get("name"), isTrue(k.Get("optional")))
			}
		}
	}
}

// service links a Service to the workloads its selector picks, and
// checks named target ports exist on them.
func (r *relater) service(o *Object, objs []Object) {
	spec := o.Root.Get("spec")
	selNode := spec.Get("selector")
	sel := stringMap(selNode)
	if len(sel) == 0 || spec.Get("type").Str() == "ExternalName" || templatedMap(selNode) {
		return
	}
	var matched []*Object
	for i := range objs {
		w := &objs[i]
		if labels, ok := podLabels(*w); ok && sameNS(o, w) && matchLabels(sel, labels) {
			matched = append(matched, w)
			r.edge(o.ID(), w.ID(), "selects", "")
		}
	}
	if len(matched) == 0 {
		r.find1(o, selNode, "warning", "selector-no-match",
			fmt.Sprintf("Service %s selects no pods in this scope (selector %s)", o.Name, selectorText(sel)),
			"Check that the selector matches the pod template labels of a workload (spec.template.metadata.labels).")
		return
	}
	// A named targetPort must be declared by the selected containers.
	for _, p := range items(spec.Get("ports")) {
		tp := p.Get("targetPort")
		if tp.Str() == "" || tp.Templated || tp.Tag == yamlkit.TagInt {
			continue
		}
		found := false
		for _, w := range matched {
			tpl, _ := podTemplate(*w)
			for _, c := range containers(tpl.Get("spec")) {
				for _, cp := range items(c.Get("ports")) {
					if cp.Get("name").Str() == tp.Value {
						found = true
					}
				}
			}
		}
		if !found {
			r.find1(o, tp, "warning", "target-port",
				fmt.Sprintf("Service %s targets port %q, which no selected container names", o.Name, tp.Value),
				"Name the container port (ports[].name) or use the port number.")
		}
	}
}

// ingress links an Ingress to the Services (and ports) it routes to.
func (r *relater) ingress(o *Object) {
	spec := o.Root.Get("spec")
	check := func(backend *yamlkit.Node, label string) {
		if backend == nil {
			return
		}
		svc := backend.Get("service")
		name, port := svc.Get("name"), svc.Get("port")
		portText := port.Get("number").Str()
		if portText == "" {
			portText = port.Get("name").Str()
		}
		if svc == nil { // networking.k8s.io/v1beta1 and extensions/v1beta1
			name, portText = backend.Get("serviceName"), backend.Get("servicePort").Str()
		}
		if name.Str() == "" || name.Templated {
			return
		}
		r.link(o, "Service", name.Value, "routes", strings.TrimSpace(label+" → "+portText), name, false)
		target := r.find("Service", o.Namespace, name.Value)
		if target == nil || portText == "" {
			return
		}
		for _, p := range items(target.Root.Get("spec").Get("ports")) {
			if p.Get("port").Str() == portText || p.Get("name").Str() == portText {
				return
			}
		}
		r.find1(o, name, "warning", "ingress-port",
			fmt.Sprintf("Ingress %s routes to port %s of Service %s, which doesn't expose it", o.Name, portText, name.Value),
			"Use one of the Service's ports (spec.ports[].port or its name).")
	}
	check(spec.Get("defaultBackend"), "default")
	check(spec.Get("backend"), "default")
	for _, rule := range items(spec.Get("rules")) {
		host := rule.Get("host").Str()
		for _, p := range items(rule.Get("http").Get("paths")) {
			check(p.Get("backend"), host+p.Get("path").Str())
		}
	}
	for _, t := range items(spec.Get("tls")) {
		r.link(o, "Secret", t.Get("secretName").Str(), "tls", "tls", t.Get("secretName"), false)
	}
}

// selector links an object with a LabelSelector to the workloads whose
// pods it selects.
func (r *relater) selector(o *Object, sel *yamlkit.Node, objs []Object, kind string) {
	if sel == nil {
		return
	}
	n := 0
	for i := range objs {
		w := &objs[i]
		if labels, ok := podLabels(*w); ok && sameNS(o, w) && matchSelector(sel, labels) {
			r.edge(o.ID(), w.ID(), kind, "")
			n++
		}
	}
	if n == 0 && (len(stringMap(sel.Get("matchLabels"))) > 0 || len(items(sel.Get("matchExpressions"))) > 0) {
		r.find1(o, sel, "info", "selector-no-match",
			fmt.Sprintf("%s %s selects no pods in this scope", o.Kind, o.Name), "")
	}
}

// binding links a RoleBinding to its role and service-account subjects.
func (r *relater) binding(o *Object) {
	ref := o.Root.Get("roleRef")
	r.link(o, ref.Get("kind").Str(), ref.Get("name").Str(), "grants", "", ref.Get("name"), false)
	for _, s := range items(o.Root.Get("subjects")) {
		if s.Get("kind").Str() != "ServiceAccount" {
			continue
		}
		ns := s.Get("namespace").Str()
		if ns == "" {
			ns = o.Namespace
		}
		r.linkNS(o, ns, "ServiceAccount", s.Get("name").Str(), "binds", "", s.Get("name"), false)
	}
}

func sameNS(a, b *Object) bool {
	return a.Namespace == "" || b.Namespace == "" || a.Namespace == b.Namespace
}

func templatedMap(n *yamlkit.Node) bool {
	if n == nil {
		return false
	}
	for _, p := range n.Pairs {
		if p.Value != nil && p.Value.Templated {
			return true
		}
	}
	return false
}

func selectorText(sel map[string]string) string {
	var parts []string
	for k, v := range sel {
		parts = append(parts, k+"="+v)
	}
	slices.Sort(parts)
	return strings.Join(parts, ",")
}
