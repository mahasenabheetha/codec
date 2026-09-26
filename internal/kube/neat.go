package kube

import (
	"bytes"
	"errors"
	"io"

	yaml "go.yaml.in/yaml/v3"
)

// ErrNeatTemplated is returned for templates: re-emitting them would
// break their expressions.
var ErrNeatTemplated = errors.New("the file contains template expressions; render it first")

// noise lists what `kubectl get -o yaml` adds that nobody wrote: cluster
// bookkeeping, not configuration.
var (
	metaNoise = []string{"managedFields", "uid", "resourceVersion", "generation", "creationTimestamp",
		"selfLink", "deletionTimestamp", "deletionGracePeriodSeconds", "ownerReferences"}
	annotationNoise = []string{"kubectl.kubernetes.io/last-applied-configuration",
		"deployment.kubernetes.io/revision", "meta.helm.sh/release-name", "meta.helm.sh/release-namespace"}
)

// Neat strips cluster noise from manifests (status, managedFields,
// uid, resourceVersion, timestamps, last-applied annotations) so what
// remains reads like the file someone wrote. Comments and key order are
// kept. For display and copying only; codec never writes it back.
func Neat(content []byte) ([]byte, error) {
	if bytes.Contains(content, []byte("{{")) {
		return nil, ErrNeatTemplated
	}
	dec := yaml.NewDecoder(bytes.NewReader(content))
	var out bytes.Buffer
	enc := yaml.NewEncoder(&out)
	enc.SetIndent(2)
	for {
		var doc yaml.Node
		if err := dec.Decode(&doc); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, err
		}
		if len(doc.Content) == 1 {
			neatObject(doc.Content[0])
		}
		if err := enc.Encode(&doc); err != nil {
			return nil, err
		}
	}
	enc.Close()
	return out.Bytes(), nil
}

func neatObject(n *yaml.Node) {
	if n.Kind != yaml.MappingNode {
		return
	}
	if kind := value(n, "kind"); kind != nil && kind.Value == "List" {
		if list := value(n, "items"); list != nil {
			for _, it := range list.Content {
				neatObject(it)
			}
		}
		return
	}
	remove(n, "status")
	meta := value(n, "metadata")
	if meta == nil || meta.Kind != yaml.MappingNode {
		return
	}
	for _, k := range metaNoise {
		remove(meta, k)
	}
	if ann := value(meta, "annotations"); ann != nil && ann.Kind == yaml.MappingNode {
		for _, k := range annotationNoise {
			remove(ann, k)
		}
		if len(ann.Content) == 0 {
			remove(meta, "annotations")
		}
	}
}

// value returns the value node of key in a mapping.
func value(m *yaml.Node, key string) *yaml.Node {
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Value == key {
			return m.Content[i+1]
		}
	}
	return nil
}

func remove(m *yaml.Node, key string) {
	for i := 0; i+1 < len(m.Content); i += 2 {
		if m.Content[i].Value == key {
			m.Content = append(m.Content[:i], m.Content[i+2:]...)
			return
		}
	}
}
