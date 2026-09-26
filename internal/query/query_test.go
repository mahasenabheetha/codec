package query

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"testing"

	"github.com/mahasenabheetha/codec/v2/internal/yamlkit"
)

const src = `apiVersion: apps/v1
kind: Deployment
metadata:
  name: api
spec:
  replicas: 2
  template:
    spec:
      containers:
        - name: api
          image: api:1.0
        - name: sidecar
          image: proxy:2
---
apiVersion: v1
kind: Service
metadata:
  name: api
`

func TestRun(t *testing.T) {
	f := yamlkit.Parse([]byte(src))
	tests := []struct {
		expr string
		want []string // "doc:line path = value"
	}{
		{".spec.replicas", []string{"0:6 .spec.replicas = 2", "1:15 .spec.replicas = null"}},
		{".spec.template.spec.containers[].image", []string{"0:11 .spec.template.spec.containers[0].image = api:1.0", "0:13 .spec.template.spec.containers[1].image = proxy:2"}},
		{`select(.kind == "Service") | .metadata.name`, []string{"1:18 .metadata.name = api"}},
		{`.. | objects | select(.name == "sidecar") | .image`, []string{"0:13 .spec.template.spec.containers[1].image = proxy:2"}},
		{`.spec.template.spec.containers | length`, []string{"0:1  = 2", "1:15  = 0"}},
	}
	for _, tt := range tests {
		t.Run(tt.expr, func(t *testing.T) {
			q, err := Compile(tt.expr)
			if err != nil {
				t.Fatal(err)
			}
			rs, err := q.Run(context.Background(), f, 100)
			if err != nil {
				t.Fatal(err)
			}
			var got []string
			for _, r := range rs {
				got = append(got, fmt.Sprintf("%d:%d %s = %s", r.Doc, r.Range.Start.Line, r.Path, r.Value))
			}
			if !slices.Equal(got, tt.want) {
				t.Errorf("got\n  %q\nwant\n  %q", got, tt.want)
			}
		})
	}
}

func TestErrors(t *testing.T) {
	if _, err := Compile(".spec[ "); err == nil {
		t.Error("want parse error")
	}
	q, _ := Compile(`$ENV.PATH`)
	rs, _ := q.Run(context.Background(), yamlkit.Parse([]byte("a: 1\n")), 10)
	if len(rs) != 1 || rs[0].Value != "null" {
		t.Errorf("environment leaked: %+v", rs)
	}
	q, _ = Compile(".[]")
	if _, err := q.Run(context.Background(), yamlkit.Parse([]byte("[1,2,3]\n")), 2); !errors.Is(err, ErrLimit) {
		t.Errorf("limit: got %v", err)
	}
}
