package query

import (
	"context"
	"testing"
	"time"

	"github.com/mahasenabheetha/codec/v2/internal/yamlkit"
)

// FuzzQuery: any expression compiles or is refused, and one that
// compiles runs over the test documents without failing hard or
// outliving its deadline by much.
func FuzzQuery(f *testing.F) {
	for _, s := range []string{".spec.replicas", `select(.kind == "Service") | .metadata.name`, ".. | objects | .image", "[.[] | length]", "def f: f; f", "range(1e9)"} {
		f.Add(s)
	}
	file := yamlkit.Parse([]byte(src))
	f.Fuzz(func(t *testing.T, expr string) {
		q, err := Compile(expr)
		if err != nil {
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
		defer cancel()
		start := time.Now()
		q.Run(ctx, file, 100)
		if d := time.Since(start); d > 3*time.Second {
			t.Fatalf("%q ran %v past a 200 ms deadline", expr, d)
		}
	})
}
