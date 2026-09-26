package kube

import (
	"fmt"
	"strings"

	"github.com/mahasenabheetha/codec/v2/internal/provider"
	"github.com/mahasenabheetha/codec/v2/internal/yamlkit"
)

// Provider is the Kubernetes lens in the provider registry: the
// built-in detection, plus an outline that says what things are
// ("Container app · nginx:1.27 · 2 ports") and decoded Secret values
// on hover.
type Provider struct {
	provider.Provider // the built-in detector it replaces
}

// Register installs the lens in reg, replacing the built-in
// "kubernetes" detector.
func Register(reg *provider.Registry) {
	if base := reg.Lookup("kubernetes"); base != nil {
		if _, done := base.(Provider); !done {
			reg.Register(Provider{base})
		}
	}
}

func init() { Register(provider.Default) }

// Symbols is the generic outline with Kubernetes meaning added.
func (p Provider) Symbols(d *yamlkit.Document) []provider.Symbol {
	syms := provider.Outline(d)
	if d == nil || d.Root == nil {
		return syms
	}
	enrich(syms, d.Root, "")
	return syms
}

// enrich walks the outline alongside the tree it was built from,
// relabelling entries whose meaning is known from their key.
func enrich(syms []provider.Symbol, n *yamlkit.Node, key string) {
	switch n.Kind {
	case yamlkit.KindMap:
		for i, pr := range n.Pairs {
			if i >= len(syms) || pr.Value == nil {
				continue
			}
			k := pr.Key.Value
			s := &syms[i]
			switch {
			case k == "metadata" && pr.Value.Kind == yamlkit.KindMap:
				name := pr.Value.Get("name").Str()
				if ns := pr.Value.Get("namespace").Str(); ns != "" {
					name = ns + "/" + name
				}
				if name != "" {
					s.Detail = name
				}
			case k == "replicas":
				s.Detail = pr.Value.Str() + " pods"
			case k == "selector" && pr.Value.Kind == yamlkit.KindMap:
				if m := stringMap(pr.Value.Get("matchLabels")); len(m) > 0 {
					s.Detail = selectorText(m)
				} else if m := stringMap(pr.Value); len(m) > 0 && pr.Value.Get("matchLabels") == nil {
					s.Detail = selectorText(m)
				}
			}
			enrich(s.Children, pr.Value, k)
		}
	case yamlkit.KindSeq:
		for i, it := range n.Items {
			if i >= len(syms) || it == nil {
				continue
			}
			s := &syms[i]
			if name, detail, ok := itemMeaning(key, it); ok {
				s.Name, s.Detail = name, detail
			}
			enrich(s.Children, it, "")
		}
	}
}

// itemMeaning names list items of well-known lists.
func itemMeaning(list string, it *yamlkit.Node) (name, detail string, ok bool) {
	if it.Kind != yamlkit.KindMap {
		return "", "", false
	}
	nm := it.Get("name").Str()
	switch list {
	case "containers", "initContainers", "ephemeralContainers":
		label := "Container "
		if list == "initContainers" {
			label = "Init container "
		}
		parts := []string{it.Get("image").Str()}
		if n := len(items(it.Get("ports"))); n > 0 {
			parts = append(parts, plural(n, "port"))
		}
		return label + nm, strings.Join(nonEmpty(parts), " · "), true
	case "ports":
		if cp := it.Get("containerPort").Str(); cp != "" {
			return "Port " + orText(nm, cp), cp + "/" + orTCP(it), true
		}
		if p := it.Get("port").Str(); p != "" {
			d := p
			if t := it.Get("targetPort").Str(); t != "" && t != p {
				d += " → " + t
			}
			return "Port " + orText(nm, p), d + "/" + orTCP(it), true
		}
	case "env":
		vf := it.Get("valueFrom")
		switch {
		case vf.Get("secretKeyRef") != nil:
			r := vf.Get("secretKeyRef")
			return "Env " + nm, "from Secret " + r.Get("name").Str() + "/" + r.Get("key").Str(), true
		case vf.Get("configMapKeyRef") != nil:
			r := vf.Get("configMapKeyRef")
			return "Env " + nm, "from ConfigMap " + r.Get("name").Str() + "/" + r.Get("key").Str(), true
		case vf.Get("fieldRef") != nil:
			return "Env " + nm, "from " + vf.Get("fieldRef").Get("fieldPath").Str(), true
		}
		return "Env " + nm, it.Get("value").Str(), true
	case "envFrom":
		if r := it.Get("configMapRef"); r != nil {
			return "Env from ConfigMap", r.Get("name").Str(), true
		}
		if r := it.Get("secretRef"); r != nil {
			return "Env from Secret", r.Get("name").Str(), true
		}
	case "volumes":
		for _, t := range []struct{ key, label, field string }{
			{"configMap", "ConfigMap", "name"}, {"secret", "Secret", "secretName"},
			{"persistentVolumeClaim", "PVC", "claimName"}, {"emptyDir", "emptyDir", ""},
			{"hostPath", "hostPath", "path"}, {"projected", "projected", ""},
		} {
			if v := it.Get(t.key); v != nil {
				return "Volume " + nm, strings.TrimSpace(t.label + " " + v.Get(t.field).Str()), true
			}
		}
		return "Volume " + nm, "", true
	case "volumeMounts":
		return "Mount " + nm, it.Get("mountPath").Str(), true
	case "rules":
		if h := it.Get("host").Str(); h != "" {
			return "Rule " + h, plural(len(items(it.Get("http").Get("paths"))), "path"), true
		}
	}
	return "", "", false
}

// Hover shows a Secret's data value decoded, and base64 data counts.
func (p Provider) Hover(f *provider.File, pos yamlkit.Pos) *provider.Hover {
	d := f.YAML.DocAt(pos)
	if d == nil || d.Root.Get("kind").Str() != "Secret" {
		return nil
	}
	data := d.Root.Get("data")
	if data == nil || data.Kind != yamlkit.KindMap {
		return nil
	}
	for _, pr := range data.Pairs {
		if pr.Value == nil || (!pr.Value.Range.Contains(pos) && !pr.Key.Range.Contains(pos)) {
			continue
		}
		for _, sv := range SecretValues(d.Root) {
			if sv.Key != pr.Key.Value {
				continue
			}
			h := &provider.Hover{Range: pr.Value.Range, Title: "Secret data: " + sv.Key}
			switch {
			case sv.Error != "":
				h.Rows = []provider.HoverRow{{Label: "Decoded", Value: sv.Error}}
			case sv.Binary:
				h.Rows = []provider.HoverRow{{Label: "Decoded", Value: fmt.Sprintf("binary, %d bytes", sv.Size)}}
			default:
				h.Rows = []provider.HoverRow{{Label: "Bytes", Value: fmt.Sprint(sv.Size)}}
				h.Code = sv.Value
			}
			return h
		}
	}
	return nil
}

func plural(n int, word string) string {
	if n == 1 {
		return "1 " + word
	}
	return fmt.Sprintf("%d %ss", n, word)
}

func nonEmpty(s []string) []string {
	var out []string
	for _, x := range s {
		if x != "" {
			out = append(out, x)
		}
	}
	return out
}
