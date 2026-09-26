package lint

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/mahasenabheetha/codec/v2/internal/yamlkit"
)

// deprecation is one API that Kubernetes deprecated or removed. The
// table is small and embedded on purpose: it changes a few times a
// year and must work offline. Source: the Kubernetes deprecation guide.
type deprecation struct {
	apiVersion  string
	kinds       []string // nil = every kind of that apiVersion
	deprecated  string   // "" = long before removal
	removed     string   // "" = not scheduled yet
	replacement string   // e.g. "apps/v1"; "" = none
}

var deprecations = []deprecation{
	{"extensions/v1beta1", []string{"Deployment", "DaemonSet", "ReplicaSet"}, "", "1.16", "apps/v1"},
	{"extensions/v1beta1", []string{"NetworkPolicy"}, "", "1.16", "networking.k8s.io/v1"},
	{"extensions/v1beta1", []string{"PodSecurityPolicy"}, "", "1.16", "policy/v1beta1"},
	{"extensions/v1beta1", []string{"Ingress"}, "1.14", "1.22", "networking.k8s.io/v1"},
	{"apps/v1beta1", nil, "", "1.16", "apps/v1"},
	{"apps/v1beta2", nil, "", "1.16", "apps/v1"},
	{"networking.k8s.io/v1beta1", []string{"Ingress", "IngressClass"}, "1.19", "1.22", "networking.k8s.io/v1"},
	{"rbac.authorization.k8s.io/v1beta1", nil, "1.17", "1.22", "rbac.authorization.k8s.io/v1"},
	{"apiextensions.k8s.io/v1beta1", nil, "1.16", "1.22", "apiextensions.k8s.io/v1"},
	{"admissionregistration.k8s.io/v1beta1", nil, "1.16", "1.22", "admissionregistration.k8s.io/v1"},
	{"apiregistration.k8s.io/v1beta1", nil, "1.19", "1.22", "apiregistration.k8s.io/v1"},
	{"authentication.k8s.io/v1beta1", nil, "1.19", "1.22", "authentication.k8s.io/v1"},
	{"authorization.k8s.io/v1beta1", nil, "1.19", "1.22", "authorization.k8s.io/v1"},
	{"certificates.k8s.io/v1beta1", nil, "1.19", "1.22", "certificates.k8s.io/v1"},
	{"coordination.k8s.io/v1beta1", nil, "1.19", "1.22", "coordination.k8s.io/v1"},
	{"scheduling.k8s.io/v1beta1", nil, "1.14", "1.22", "scheduling.k8s.io/v1"},
	{"storage.k8s.io/v1beta1", []string{"CSIDriver", "CSINode", "StorageClass", "VolumeAttachment"}, "1.19", "1.22", "storage.k8s.io/v1"},
	{"batch/v1beta1", []string{"CronJob"}, "1.21", "1.25", "batch/v1"},
	{"discovery.k8s.io/v1beta1", []string{"EndpointSlice"}, "1.21", "1.25", "discovery.k8s.io/v1"},
	{"events.k8s.io/v1beta1", []string{"Event"}, "1.19", "1.25", "events.k8s.io/v1"},
	{"autoscaling/v2beta1", []string{"HorizontalPodAutoscaler"}, "1.22", "1.25", "autoscaling/v2"},
	{"policy/v1beta1", []string{"PodDisruptionBudget"}, "1.21", "1.25", "policy/v1"},
	{"policy/v1beta1", []string{"PodSecurityPolicy"}, "1.21", "1.25", ""},
	{"node.k8s.io/v1beta1", []string{"RuntimeClass"}, "1.20", "1.25", "node.k8s.io/v1"},
	{"autoscaling/v2beta2", []string{"HorizontalPodAutoscaler"}, "1.23", "1.26", "autoscaling/v2"},
	{"flowcontrol.apiserver.k8s.io/v1beta1", nil, "1.23", "1.26", "flowcontrol.apiserver.k8s.io/v1"},
	{"storage.k8s.io/v1beta1", []string{"CSIStorageCapacity"}, "1.24", "1.27", "storage.k8s.io/v1"},
	{"flowcontrol.apiserver.k8s.io/v1beta2", nil, "1.26", "1.29", "flowcontrol.apiserver.k8s.io/v1"},
	{"flowcontrol.apiserver.k8s.io/v1beta3", nil, "1.29", "1.32", "flowcontrol.apiserver.k8s.io/v1"},
	{"v1", []string{"Endpoints"}, "1.33", "", "discovery.k8s.io/v1 EndpointSlice"},
}

// deprecation reports a removed or deprecated apiVersion/kind for the
// target Kubernetes version.
func (c *checker) deprecation(apiVersion *yamlkit.Node, kind string) {
	target := minor(c.cfg.Version())
	for _, d := range deprecations {
		if d.apiVersion != apiVersion.Value || (d.kinds != nil && !slices.Contains(d.kinds, kind)) {
			continue
		}
		hint := "There is no direct replacement; see the Kubernetes deprecation guide."
		if d.replacement != "" {
			hint = fmt.Sprintf("Use %s; check the field changes in the Kubernetes deprecation guide.", d.replacement)
		}
		switch {
		case d.removed != "" && target >= minor(d.removed):
			c.report("api-removed", apiVersion.Range,
				fmt.Sprintf("%s %s was removed in Kubernetes %s (target %s)", d.apiVersion, kind, d.removed, c.cfg.Version()), hint)
		case d.deprecated == "" || target >= minor(d.deprecated):
			when := "is deprecated"
			if d.deprecated != "" {
				when = "is deprecated since Kubernetes " + d.deprecated
			}
			if d.removed != "" {
				when += " and removed in " + d.removed
			}
			c.report("api-deprecated", apiVersion.Range, fmt.Sprintf("%s %s %s", d.apiVersion, kind, when), hint)
		}
		return
	}
}

// Removed reports whether apiVersion/kind no longer exists in the
// target version (so there is no schema to check it against either).
func Removed(apiVersion, kind, target string) bool {
	for _, d := range deprecations {
		if d.apiVersion == apiVersion && (d.kinds == nil || slices.Contains(d.kinds, kind)) {
			return d.removed != "" && minor(target) >= minor(d.removed)
		}
	}
	return false
}

// minor turns "1.25" into 25 (0 if unparseable).
func minor(v string) int {
	_, m, _ := strings.Cut(strings.TrimPrefix(v, "v"), ".")
	m, _, _ = strings.Cut(m, ".")
	n, _ := strconv.Atoi(m)
	return n
}
