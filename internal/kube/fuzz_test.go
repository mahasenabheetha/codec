package kube

import (
	"testing"

	"github.com/mahasenabheetha/codec/v2/internal/yamlkit"
)

// FuzzNeat: a neat copy of anything is made or refused, never fails
// hard, and what it makes is still YAML that reads.
func FuzzNeat(f *testing.F) {
	f.Add([]byte("apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: c # keep me\n  uid: 1234\n  managedFields: [{manager: kubectl}]\ndata:\n  a: \"1\"\nstatus: {}\n"))
	f.Add([]byte("---\nkind: List\nitems:\n- kind: Pod\n  metadata: {name: p, resourceVersion: \"1\"}\n"))
	f.Fuzz(func(t *testing.T, in []byte) {
		out, err := Neat(in)
		if err != nil || yamlkit.Parse(in).HasErrors() {
			return
		}
		if yamlkit.Parse(out).HasErrors() {
			t.Fatalf("neat copy of valid YAML doesn't parse:\n%s", out)
		}
	})
}
