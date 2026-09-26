// Package kube is the Kubernetes lens: it reads manifests as objects
// and explains them — an outline with meaning, an inventory (images,
// ports, config references, resource totals), the relationships
// between objects (Service → pods, Ingress → Service, workload →
// ConfigMap …) with broken references, "neat" copies without cluster
// noise, decoded Secrets, and in-process Kustomize builds.
//
// Like yamlkit it is pure: parsed documents in, facts out. Kustomize
// builds run on an in-memory filesystem the caller fills.
package kube

import (
	"strings"

	"github.com/mahasenabheetha/codec/v2/internal/yamlkit"
)

// Source says where an object is written.
type Source struct {
	File string `json:"file,omitempty"` // workspace path; "" for a render
	Doc  int    `json:"doc"`            // 0-based document index
	Line int    `json:"line"`           // first line of the object
}

// Object is one Kubernetes object from a manifest.
type Object struct {
	APIVersion string            `json:"apiVersion"`
	Kind       string            `json:"kind"`
	Name       string            `json:"name"`
	Namespace  string            `json:"namespace,omitempty"`
	Labels     map[string]string `json:"labels,omitempty"`
	Source     Source            `json:"source"`
	Root       *yamlkit.Node     `json:"-"`
}

// ID names an object uniquely in a scope: Kind/namespace/name, with an
// empty namespace kept as "Kind//name".
func (o Object) ID() string { return ID(o.Kind, o.Namespace, o.Name) }

// ID builds an object id.
func ID(kind, namespace, name string) string { return kind + "/" + namespace + "/" + name }

// Collect reads the objects of a parsed file. Documents without a kind
// and name, or whose identity is templated, are skipped.
func Collect(file string, f *yamlkit.File) []Object {
	var out []Object
	for i, d := range f.Docs {
		r := d.Root
		kind, meta := r.Get("kind"), r.Get("metadata")
		name := meta.Get("name")
		if kind.Str() == "" || name.Str() == "" || kind.Templated || name.Templated {
			continue
		}
		// kubectl's "kind: List" wraps objects in items.
		if kind.Value == "List" {
			for _, it := range items(r.Get("items")) {
				if o, ok := object(it, file, i); ok {
					out = append(out, o)
				}
			}
			continue
		}
		if o, ok := object(r, file, i); ok {
			out = append(out, o)
		}
	}
	return out
}

func object(r *yamlkit.Node, file string, doc int) (Object, bool) {
	meta := r.Get("metadata")
	o := Object{
		APIVersion: r.Get("apiVersion").Str(),
		Kind:       r.Get("kind").Str(),
		Name:       meta.Get("name").Str(),
		Namespace:  meta.Get("namespace").Str(),
		Labels:     stringMap(meta.Get("labels")),
		Source:     Source{File: file, Doc: doc, Line: r.Range.Start.Line},
		Root:       r,
	}
	if o.Kind == "" || o.Name == "" {
		return o, false
	}
	return o, true
}

// clusterScoped kinds have no namespace, so references to them ignore it.
var clusterScoped = map[string]bool{
	"Namespace": true, "Node": true, "PersistentVolume": true, "StorageClass": true,
	"ClusterRole": true, "ClusterRoleBinding": true, "CustomResourceDefinition": true,
	"PriorityClass": true, "IngressClass": true, "MutatingWebhookConfiguration": true,
	"ValidatingWebhookConfiguration": true, "APIService": true, "RuntimeClass": true,
	"ClusterIssuer": true, "CSIDriver": true, "VolumeSnapshotClass": true,
}

// podTemplate returns a workload's pod template (metadata + spec), or
// the Pod itself. longRunning is true for controllers that keep pods up.
func podTemplate(o Object) (tpl *yamlkit.Node, longRunning bool) {
	r := o.Root
	switch o.Kind {
	case "Pod":
		return r, false
	case "Deployment", "StatefulSet", "DaemonSet", "ReplicaSet", "ReplicationController", "Rollout":
		return r.Get("spec").Get("template"), true
	case "Job":
		return r.Get("spec").Get("template"), false
	case "CronJob":
		return r.Get("spec").Get("jobTemplate").Get("spec").Get("template"), false
	}
	return nil, false
}

// podLabels are the labels a workload's pods get.
func podLabels(o Object) (map[string]string, bool) {
	tpl, _ := podTemplate(o)
	if tpl == nil {
		return nil, false
	}
	return stringMap(tpl.Get("metadata").Get("labels")), true
}

// containers lists a pod spec's init and app containers.
func containers(spec *yamlkit.Node) []*yamlkit.Node {
	return append(items(spec.Get("initContainers")), items(spec.Get("containers"))...)
}

// --- label selectors ---

// matchLabels reports whether labels carry every selector pair. An
// empty selector matches nothing here: for a Service it means "no
// selector" (endpoints managed by hand), not "everything".
func matchLabels(sel, labels map[string]string) bool {
	if len(sel) == 0 {
		return false
	}
	for k, v := range sel {
		if labels[k] != v {
			return false
		}
	}
	return true
}

// matchSelector evaluates a LabelSelector (matchLabels plus
// matchExpressions with In, NotIn, Exists, DoesNotExist).
func matchSelector(sel *yamlkit.Node, labels map[string]string) bool {
	if sel == nil || sel.Kind != yamlkit.KindMap {
		return false
	}
	ml := stringMap(sel.Get("matchLabels"))
	exprs := items(sel.Get("matchExpressions"))
	if len(ml) == 0 && len(exprs) == 0 {
		return false
	}
	for k, v := range ml {
		if labels[k] != v {
			return false
		}
	}
	for _, e := range exprs {
		key, op := e.Get("key").Str(), e.Get("operator").Str()
		var values []string
		for _, v := range items(e.Get("values")) {
			values = append(values, v.Str())
		}
		val, has := labels[key]
		switch op {
		case "In":
			if !has || !contains(values, val) {
				return false
			}
		case "NotIn":
			if has && contains(values, val) {
				return false
			}
		case "Exists":
			if !has {
				return false
			}
		case "DoesNotExist":
			if has {
				return false
			}
		default:
			return false
		}
	}
	return true
}

// --- small node helpers ---

func items(n *yamlkit.Node) []*yamlkit.Node {
	if n == nil || n.Kind != yamlkit.KindSeq {
		return nil
	}
	return n.Items
}

func stringMap(n *yamlkit.Node) map[string]string {
	if n == nil || n.Kind != yamlkit.KindMap {
		return nil
	}
	m := make(map[string]string, len(n.Pairs))
	for _, p := range n.Pairs {
		if p.Key != nil && p.Value != nil && p.Value.Kind == yamlkit.KindScalar {
			m[p.Key.Value] = p.Value.Value
		}
	}
	return m
}

func contains(s []string, v string) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}

func isTrue(n *yamlkit.Node) bool {
	return n != nil && n.Kind == yamlkit.KindScalar && strings.EqualFold(n.Value, "true")
}
