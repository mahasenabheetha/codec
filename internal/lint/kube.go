package lint

import (
	"fmt"
	"strings"

	"github.com/mahasenabheetha/codec/v2/internal/yamlkit"
)

// kubernetes runs the object checks on every document that is a
// Kubernetes object (apiVersion + kind).
func (c *checker) kubernetes() {
	for _, d := range c.in.YAML.Docs {
		root := d.Root
		apiVersion, kind := root.Get("apiVersion"), root.Get("kind")
		if apiVersion.Str() == "" || kind.Str() == "" || apiVersion.Templated || kind.Templated {
			continue
		}
		c.deprecation(apiVersion, kind.Str())
		// Helm test hooks are one-off pods run by `helm test`, not
		// workloads: resource and security advice is noise there.
		if strings.Contains(root.Get("metadata").Get("annotations").Get("helm.sh/hook").Str(), "test") {
			continue
		}
		if spec, longRunning := podSpec(root, kind.Str()); spec != nil && spec.Kind == yamlkit.KindMap {
			c.pod(spec, longRunning)
		}
	}
}

// podSpec finds the pod template's spec of a workload. longRunning is
// true for controllers that keep pods serving (probes matter there,
// not for Jobs or one-off Pods such as Helm tests).
func podSpec(root *yamlkit.Node, kind string) (spec *yamlkit.Node, longRunning bool) {
	get := func(n *yamlkit.Node, path ...string) *yamlkit.Node {
		for _, k := range path {
			n = n.Get(k)
		}
		return n
	}
	switch kind {
	case "Pod":
		return root.Get("spec"), false
	case "Deployment", "StatefulSet", "DaemonSet", "ReplicaSet", "ReplicationController", "Rollout":
		return get(root, "spec", "template", "spec"), true
	case "Job":
		return get(root, "spec", "template", "spec"), false
	case "CronJob":
		return get(root, "spec", "jobTemplate", "spec", "template", "spec"), false
	}
	return nil, false
}

// pod checks one pod spec and its containers.
func (c *checker) pod(spec *yamlkit.Node, longRunning bool) {
	if v := spec.Get("hostNetwork"); isTrue(v) {
		c.report("host-network", v.Range, "The pod uses the node's network (hostNetwork: true)",
			"Remove hostNetwork unless this is a node-level agent that needs it.")
	}
	for _, vol := range items(spec.Get("volumes")) {
		if hp := vol.Pair("hostPath"); hp != nil {
			path := hp.Value.Get("path").Str()
			c.report("host-path", hp.Key.Range,
				fmt.Sprintf("Volume %q mounts the node's %s", vol.Get("name").Str(), orText(path, "filesystem")),
				"Use a PersistentVolumeClaim, emptyDir, ConfigMap or Secret instead.")
		}
	}
	podNonRoot := nonRoot(spec.Get("securityContext"))

	check := func(ctr *yamlkit.Node, init bool) {
		if ctr == nil || ctr.Kind != yamlkit.KindMap {
			return
		}
		name := ctr.Get("name").Str()
		at := ctr.Range // the item; narrowed to its name when present
		if n := ctr.Pair("name"); n != nil {
			at = yamlkit.Range{Start: n.Key.Range.Start, End: n.Value.Range.End}
		}
		c.image(ctr.Get("image"))
		c.resources(ctr, name, at)
		if longRunning && !init {
			c.probes(ctr, name, at)
		}
		sc := ctr.Get("securityContext")
		if v := sc.Get("privileged"); isTrue(v) {
			c.report("privileged", v.Range, fmt.Sprintf("Container %q runs privileged", name),
				"Remove privileged: true and add only the capabilities it needs (securityContext.capabilities.add).")
		}
		if !podNonRoot && !nonRoot(sc) {
			c.report("run-as-non-root", at, fmt.Sprintf("Container %q may run as root", name),
				"Set securityContext.runAsNonRoot: true (with a non-zero runAsUser) on the pod or the container.")
		}
	}
	for _, ctr := range items(spec.Get("initContainers")) {
		check(ctr, true)
	}
	for _, ctr := range items(spec.Get("containers")) {
		check(ctr, false)
	}
}

func (c *checker) image(img *yamlkit.Node) {
	ref := img.Str()
	if ref == "" || img.Templated || strings.Contains(ref, "@") {
		return // a digest pins it
	}
	tag := ""
	if i := strings.LastIndex(ref, ":"); i > strings.LastIndex(ref, "/") {
		tag = ref[i+1:]
	}
	switch tag {
	case "":
		c.report("image-tag", img.Range, fmt.Sprintf("Image %q has no tag, so it means :latest", ref),
			fmt.Sprintf("Pin a version (%s:1.2.3) or a digest (@sha256:…).", ref))
	case "latest":
		c.report("image-tag", img.Range, fmt.Sprintf("Image %q uses the moving latest tag", ref),
			"Pin a version tag or a digest (@sha256:…).")
	}
}

func (c *checker) resources(ctr *yamlkit.Node, name string, at yamlkit.Range) {
	res := ctr.Get("resources")
	if res != nil && res.Templated {
		return
	}
	requests, limits := hasEntries(res.Get("requests")), hasEntries(res.Get("limits"))
	var msg string
	switch {
	case !requests && !limits:
		msg = "has no resource requests or limits"
	case !requests:
		msg = "has no resource requests"
	case !limits:
		msg = "has no resource limits"
	default:
		return
	}
	c.report("resources", at, fmt.Sprintf("Container %q %s", name, msg),
		"Set resources.requests (cpu, memory) and at least a memory limit.")
}

func (c *checker) probes(ctr *yamlkit.Node, name string, at yamlkit.Range) {
	if ctr.Get("readinessProbe") != nil {
		return
	}
	msg := fmt.Sprintf("Container %q has no readiness probe", name)
	if ctr.Get("livenessProbe") == nil {
		msg = fmt.Sprintf("Container %q has no readiness or liveness probe", name)
	}
	c.report("probes", at, msg, "Add a readinessProbe (and a livenessProbe if the app can hang).")
}

// nonRoot reports whether a securityContext keeps the process off root:
// runAsNonRoot: true, or an explicit non-zero runAsUser.
func nonRoot(sc *yamlkit.Node) bool {
	if isTrue(sc.Get("runAsNonRoot")) {
		return true
	}
	u := sc.Get("runAsUser")
	return u != nil && u.Tag == yamlkit.TagInt && strings.TrimLeft(u.Value, "0+") != ""
}

func isTrue(n *yamlkit.Node) bool {
	return n != nil && n.Kind == yamlkit.KindScalar && n.Tag == yamlkit.TagBool && strings.EqualFold(n.Value, "true")
}

func items(n *yamlkit.Node) []*yamlkit.Node {
	if n == nil || n.Kind != yamlkit.KindSeq {
		return nil
	}
	return n.Items
}

func hasEntries(n *yamlkit.Node) bool {
	return n != nil && n.Kind == yamlkit.KindMap && len(n.Pairs) > 0
}

func orText(s, fallback string) string {
	if s == "" {
		return fallback
	}
	return s
}
