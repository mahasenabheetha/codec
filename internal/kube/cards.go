package kube

import (
	"fmt"
	"strings"

	"github.com/mahasenabheetha/codec/v2/internal/yamlkit"
)

// Card is one object explained in a few facts, for the resources view.
type Card struct {
	ID         string        `json:"id"`
	Kind       string        `json:"kind"`
	Name       string        `json:"name"`
	Namespace  string        `json:"namespace,omitempty"`
	Source     Source        `json:"source"`
	Facts      []Fact        `json:"facts"`
	Containers []Container   `json:"containers,omitempty"`
	Secret     []SecretValue `json:"secret,omitempty"` // decoded; the UI masks them
}

// Fact is one labelled line of a card.
type Fact struct {
	Label string `json:"label"`
	Value string `json:"value"`
}

// Container is a container of a workload card.
type Container struct {
	Name  string   `json:"name"`
	Image string   `json:"image"`
	Ports []string `json:"ports,omitempty"`
	Init  bool     `json:"init,omitempty"`
}

// Cards describes every object; g supplies who uses and selects what.
func Cards(objs []Object, g Graph) []Card {
	in := map[string][]Edge{}
	out := map[string][]Edge{}
	for _, e := range g.Edges {
		in[e.To] = append(in[e.To], e)
		out[e.From] = append(out[e.From], e)
	}
	names := map[string]string{}
	for _, n := range g.Nodes {
		names[n.ID] = n.Kind + " " + n.Name
	}
	cards := make([]Card, 0, len(objs))
	seen := map[string]bool{}
	for _, o := range objs {
		// Card ids must be unique, so a second definition of the same
		// object (a base and an overlay's patch) is told apart by file.
		id := o.ID()
		if seen[id] {
			id += fmt.Sprintf("@%s#%d", o.Source.File, o.Source.Doc)
		}
		seen[o.ID()] = true
		c := Card{ID: id,Kind: o.Kind, Name: o.Name, Namespace: o.Namespace, Source: o.Source, Facts: []Fact{}}
		add := func(label, value string) {
			if value != "" {
				c.Facts = append(c.Facts, Fact{label, value})
			}
		}
		spec := o.Root.Get("spec")
		if tpl, _ := podTemplate(o); tpl != nil {
			switch o.Kind {
			case "DaemonSet":
				add("Replicas", "one per node")
			case "Job", "Pod":
			case "CronJob":
				add("Schedule", spec.Get("schedule").Str())
			default:
				add("Replicas", fmt.Sprint(replicas(o)))
			}
			ps := tpl.Get("spec")
			for i, ctr := range containers(ps) {
				ct := Container{Name: ctr.Get("name").Str(), Image: ctr.Get("image").Str(), Init: i < len(items(ps.Get("initContainers")))}
				for _, p := range items(ctr.Get("ports")) {
					ct.Ports = append(ct.Ports, portText(p.Get("containerPort").Str(), p.Get("name").Str(), orTCP(p)))
				}
				c.Containers = append(c.Containers, ct)
			}
			add("Service account", ps.Get("serviceAccountName").Str())
		}
		switch o.Kind {
		case "Service":
			add("Type", orText(spec.Get("type").Str(), "ClusterIP"))
			var ports []string
			for _, p := range items(spec.Get("ports")) {
				s := p.Get("port").Str()
				if t := p.Get("targetPort").Str(); t != "" && t != s {
					s += " → " + t
				}
				ports = append(ports, strings.TrimSpace(s+"/"+orTCP(p)+" "+p.Get("name").Str()))
			}
			add("Ports", strings.Join(ports, ", "))
			if sel := stringMap(spec.Get("selector")); len(sel) > 0 {
				add("Selector", selectorText(sel))
			}
		case "Ingress":
			add("Class", spec.Get("ingressClassName").Str())
			var hosts []string
			for _, r := range items(spec.Get("rules")) {
				if h := r.Get("host").Str(); h != "" {
					hosts = append(hosts, h)
				}
			}
			add("Hosts", strings.Join(hosts, ", "))
		case "ConfigMap":
			add("Keys", keys(o.Root.Get("data"), o.Root.Get("binaryData")))
		case "Secret":
			add("Type", orText(o.Root.Get("type").Str(), "Opaque"))
			c.Secret = SecretValues(o.Root)
		case "PersistentVolumeClaim":
			add("Size", spec.Get("resources").Get("requests").Get("storage").Str())
			var modes []string
			for _, m := range items(spec.Get("accessModes")) {
				modes = append(modes, m.Str())
			}
			add("Access", strings.Join(modes, ", "))
			add("Storage class", spec.Get("storageClassName").Str())
		case "HorizontalPodAutoscaler":
			ref := spec.Get("scaleTargetRef")
			add("Target", ref.Get("kind").Str()+" "+ref.Get("name").Str())
			add("Replicas", spec.Get("minReplicas").Str()+"–"+spec.Get("maxReplicas").Str())
		case "Role", "ClusterRole":
			add("Rules", fmt.Sprint(len(items(o.Root.Get("rules")))))
		}
		// Relationships: what selects or uses this object, and what it uses.
		var by, uses []string
		for _, e := range in[o.ID()] {
			by = append(by, names[e.From])
		}
		for _, e := range out[o.ID()] {
			uses = append(uses, names[e.To])
		}
		if o.Kind == "Service" {
			if len(uses) == 0 && len(stringMap(spec.Get("selector"))) > 0 {
				add("Selects", "no pods in this scope")
			}
			add("Selects", strings.Join(dedupe(uses), ", "))
		} else {
			add("Used by", strings.Join(dedupe(by), ", "))
			add("Uses", strings.Join(dedupe(uses), ", "))
		}
		cards = append(cards, c)
	}
	return cards
}

func portText(port, name, proto string) string {
	s := port + "/" + proto
	if name != "" {
		s += " " + name
	}
	return s
}

func keys(maps ...*yamlkit.Node) string {
	var out []string
	for _, m := range maps {
		if m == nil || m.Kind != yamlkit.KindMap {
			continue
		}
		for _, p := range m.Pairs {
			out = append(out, p.Key.Value)
		}
	}
	if len(out) > 8 {
		return strings.Join(out[:8], ", ") + fmt.Sprintf(" … (%d)", len(out))
	}
	return strings.Join(out, ", ")
}

func dedupe(s []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, x := range s {
		if !seen[x] {
			seen[x] = true
			out = append(out, x)
		}
	}
	return out
}

func orText(s, fallback string) string {
	if s == "" {
		return fallback
	}
	return s
}
