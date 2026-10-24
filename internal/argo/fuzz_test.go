package argo

import "testing"

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
