package kube

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"k8s.io/apimachinery/pkg/api/resource"

	"github.com/mahasenabheetha/codec/v2/internal/codec"
	"github.com/mahasenabheetha/codec/v2/internal/yamlkit"
)

// Inventory is what a set of manifests contains, at a glance.
type Inventory struct {
	Objects   int         `json:"objects"`
	Kinds     []KindCount `json:"kinds"`
	Images    []Image     `json:"images"`
	Ports     []Port      `json:"ports"`
	Config    []ConfigRef `json:"config"`
	Resources Resources   `json:"resources"`
}

// KindCount is how many objects of a kind there are.
type KindCount struct {
	Kind  string `json:"kind"`
	Count int    `json:"count"`
}

// Image is one container image and who runs it.
type Image struct {
	Ref    string   `json:"ref"`
	Repo   string   `json:"repo"`
	Tag    string   `json:"tag"` // "" = none (latest); digest when pinned by one
	Pinned bool     `json:"pinned"`
	UsedBy []string `json:"usedBy"` // "Deployment api (app)"
}

// Port is a container or Service port.
type Port struct {
	Object   string `json:"object"` // object id
	Kind     string `json:"kind"`   // container, service
	Name     string `json:"name,omitempty"`
	Port     string `json:"port"`
	Target   string `json:"target,omitempty"` // service targetPort
	Protocol string `json:"protocol"`
	NodePort string `json:"nodePort,omitempty"`
}

// ConfigRef is a ConfigMap or Secret and who uses it.
type ConfigRef struct {
	Kind   string   `json:"kind"`
	Name   string   `json:"name"`
	Found  bool     `json:"found"` // defined in this scope
	UsedBy []string `json:"usedBy"`
}

// Resources totals what the workloads ask for.
type Resources struct {
	Rows  []ResourceRow `json:"rows"`
	Total ResourceRow   `json:"total"` // per-pod values × replicas, summed
}

// ResourceRow is one workload: per-pod requests/limits and replicas.
type ResourceRow struct {
	Object      string `json:"object"`
	Replicas    int    `json:"replicas"`
	RequestsCPU string `json:"requestsCpu"`
	RequestsMem string `json:"requestsMemory"`
	LimitsCPU   string `json:"limitsCpu"`
	LimitsMem   string `json:"limitsMemory"`
	Missing     bool   `json:"missing,omitempty"` // some container sets none
}

// Summarize builds the inventory of objs (g gives the config usage).
func Summarize(objs []Object, g Graph) Inventory {
	inv := Inventory{Objects: len(objs), Kinds: []KindCount{}, Images: []Image{}, Ports: []Port{}, Config: []ConfigRef{}}
	kinds := map[string]int{}
	images := map[string]*Image{}
	var total [4]resource.Quantity
	for _, o := range objs {
		kinds[o.Kind]++
		if o.Kind == "Service" {
			for _, p := range items(o.Root.Get("spec").Get("ports")) {
				inv.Ports = append(inv.Ports, Port{Object: o.ID(), Kind: "service", Name: p.Get("name").Str(),
					Port: p.Get("port").Str(), Target: p.Get("targetPort").Str(), Protocol: orTCP(p), NodePort: p.Get("nodePort").Str()})
			}
		}
		tpl, _ := podTemplate(o)
		if tpl == nil {
			continue
		}
		row := ResourceRow{Object: o.ID(), Replicas: replicas(o)}
		var sum [4]resource.Quantity
		for _, c := range containers(tpl.Get("spec")) {
			name := c.Get("name").Str()
			if img := c.Get("image"); img.Str() != "" && !img.Templated {
				im := images[img.Value]
				if im == nil {
					repo, tag, pinned := splitImage(img.Value)
					im = &Image{Ref: img.Value, Repo: repo, Tag: tag, Pinned: pinned}
					images[img.Value] = im
				}
				im.UsedBy = append(im.UsedBy, fmt.Sprintf("%s %s (%s)", o.Kind, o.Name, name))
			}
			for _, p := range items(c.Get("ports")) {
				inv.Ports = append(inv.Ports, Port{Object: o.ID(), Kind: "container", Name: p.Get("name").Str(),
					Port: p.Get("containerPort").Str(), Protocol: orTCP(p)})
			}
			res := c.Get("resources")
			vals := []*yamlkit.Node{res.Get("requests").Get("cpu"), res.Get("requests").Get("memory"), res.Get("limits").Get("cpu"), res.Get("limits").Get("memory")}
			for i, v := range vals {
				q, err := resource.ParseQuantity(v.Str())
				if v.Str() == "" || err != nil {
					row.Missing = true
					continue
				}
				sum[i].Add(q)
			}
		}
		row.RequestsCPU, row.RequestsMem, row.LimitsCPU, row.LimitsMem = qty(sum[0]), qty(sum[1]), qty(sum[2]), qty(sum[3])
		inv.Resources.Rows = append(inv.Resources.Rows, row)
		for i := range sum {
			for range max(row.Replicas, 0) {
				total[i].Add(sum[i])
			}
		}
	}
	for k, n := range kinds {
		inv.Kinds = append(inv.Kinds, KindCount{k, n})
	}
	sort.Slice(inv.Kinds, func(i, j int) bool {
		if inv.Kinds[i].Count != inv.Kinds[j].Count {
			return inv.Kinds[i].Count > inv.Kinds[j].Count
		}
		return inv.Kinds[i].Kind < inv.Kinds[j].Kind
	})
	for _, im := range images {
		inv.Images = append(inv.Images, *im)
	}
	sort.Slice(inv.Images, func(i, j int) bool { return inv.Images[i].Ref < inv.Images[j].Ref })
	inv.Resources.Total = ResourceRow{Object: "total", RequestsCPU: qty(total[0]), RequestsMem: qty(total[1]), LimitsCPU: qty(total[2]), LimitsMem: qty(total[3])}
	for _, r := range inv.Resources.Rows {
		inv.Resources.Total.Replicas += r.Replicas
		inv.Resources.Total.Missing = inv.Resources.Total.Missing || r.Missing
	}
	if inv.Resources.Rows == nil {
		inv.Resources.Rows = []ResourceRow{}
	}
	inv.Config = configRefs(g)
	return inv
}

// configRefs groups the config/secret edges of g by target.
func configRefs(g Graph) []ConfigRef {
	nodes := map[string]Node{}
	for _, n := range g.Nodes {
		nodes[n.ID] = n
	}
	refs := map[string]*ConfigRef{}
	var order []string
	for _, e := range g.Edges {
		switch e.Kind {
		case "config", "secret", "pulls", "tls":
		default:
			continue
		}
		to, from := nodes[e.To], nodes[e.From]
		ref := refs[e.To]
		if ref == nil {
			ref = &ConfigRef{Kind: to.Kind, Name: to.Name, Found: !to.Missing}
			refs[e.To] = ref
			order = append(order, e.To)
		}
		ref.UsedBy = append(ref.UsedBy, strings.TrimSpace(from.Kind+" "+from.Name+" ("+e.Label+")"))
	}
	out := []ConfigRef{}
	for _, id := range order {
		out = append(out, *refs[id])
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Kind+out[i].Name < out[j].Kind+out[j].Name })
	return out
}

func replicas(o Object) int {
	switch o.Kind {
	case "Deployment", "StatefulSet", "ReplicaSet", "ReplicationController", "Rollout":
		r := o.Root.Get("spec").Get("replicas")
		if n, err := strconv.Atoi(r.Str()); err == nil {
			return n
		}
		return 1 // the Kubernetes default
	case "DaemonSet":
		return 1 // per node; the count depends on the cluster
	}
	return 1
}

func qty(q resource.Quantity) string {
	if q.IsZero() {
		return ""
	}
	return q.String()
}

func orTCP(p *yamlkit.Node) string {
	if s := p.Get("protocol").Str(); s != "" {
		return s
	}
	return "TCP"
}

// splitImage splits "registry:5000/app:1.2@sha256:…" into repo and tag.
func splitImage(ref string) (repo, tag string, pinned bool) {
	if i := strings.Index(ref, "@"); i >= 0 {
		return ref[:i], ref[i+1:], true
	}
	if i := strings.LastIndex(ref, ":"); i > strings.LastIndex(ref, "/") {
		return ref[:i], ref[i+1:], ref[i+1:] != "latest"
	}
	return ref, "", false
}

// --- Secrets ---

// SecretValue is one decoded Secret entry.
type SecretValue struct {
	Key    string `json:"key"`
	Value  string `json:"value"`            // decoded text ("" when binary)
	Binary bool   `json:"binary,omitempty"` // not text; Size bytes
	Size   int    `json:"size"`
	Error  string `json:"error,omitempty"` // not valid base64
	Line   int    `json:"line"`
}

// SecretValues decodes a Secret's data (base64) and stringData (plain).
func SecretValues(root *yamlkit.Node) []SecretValue {
	var out []SecretValue
	if d := root.Get("data"); d != nil && d.Kind == yamlkit.KindMap {
		for _, p := range d.Pairs {
			sv := SecretValue{Key: p.Key.Value, Line: p.Key.Range.Start.Line}
			raw, err := codec.Decode(strings.TrimSpace(p.Value.Str()), codec.VariantStd)
			switch {
			case p.Value.Templated:
				sv.Error = "templated: known after rendering"
			case err != nil:
				sv.Error = "not valid base64"
			case isText(raw):
				sv.Value, sv.Size = string(raw), len(raw)
			default:
				sv.Binary, sv.Size = true, len(raw)
			}
			out = append(out, sv)
		}
	}
	if d := root.Get("stringData"); d != nil && d.Kind == yamlkit.KindMap {
		for _, p := range d.Pairs {
			out = append(out, SecretValue{Key: p.Key.Value, Value: p.Value.Str(), Size: len(p.Value.Str()), Line: p.Key.Range.Start.Line})
		}
	}
	return out
}

func isText(b []byte) bool {
	if !utf8.Valid(b) {
		return false
	}
	for _, r := range string(b) {
		if unicode.IsControl(r) && r != '\n' && r != '\r' && r != '\t' {
			return false
		}
	}
	return true
}
