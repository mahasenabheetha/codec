package argo

import (
	"os"
	"testing"
)

// FuzzExpressions: a depends expression and a when condition are read
// or refused, never fail hard.
func FuzzExpressions(f *testing.F) {
	for _, s := range []string{"a && !(b.Failed || c)", "a.Succeeded || b.Errored", "{{steps.x.outputs.result}} == 3", "'a' =~ 'b.*'", "1 < 2 && (3 >= 4 || !false)"} {
		f.Add(s)
	}
	f.Fuzz(func(t *testing.T, s string) {
		ParseDepends(s)
		EvalWhen(s)
	})
}

// FuzzReadStatus: any pasted text is read as a run or refused; a run's
// nodes each appear once, each after a shallower one.
func FuzzReadStatus(f *testing.F) {
	if b, err := os.ReadFile("testdata/status.yaml"); err == nil {
		f.Add(b)
	}
	f.Add([]byte(`{"status":{"nodes":{"a":{"id":"a","name":"a","children":["b","a"]},"b":{"id":"b","children":["a"]}}}}`))
	f.Fuzz(func(t *testing.T, in []byte) {
		r, err := ReadStatus(in)
		if err != nil {
			return
		}
		seen := map[string]bool{}
		for k, n := range r.Nodes {
			if seen[n.ID] {
				t.Fatalf("node %q twice", n.ID)
			}
			seen[n.ID] = true
			if k > 0 && n.Depth > r.Nodes[k-1].Depth+1 || n.Depth < 0 {
				t.Fatalf("node %q at depth %d after %d", n.ID, n.Depth, r.Nodes[k-1].Depth)
			}
		}
	})
}
