package yamlkit

import (
	"fmt"
	"slices"
	"strings"
	"testing"
)

func TestLayeredMerge(t *testing.T) {
	base := Parse([]byte("a:\n  x: 1\n  list: [1, 2]\nb: keep\n")).Docs[0].Root
	over := Parse([]byte("a:\n  x: 2\n  list: [2, 3]\n")).Docs[0].Root
	o := Origins{}
	rules := &MergeRules{List: func(p []string) func(*Node) string {
		if slices.Equal(p, []string{"a", "list"}) {
			return InlineText // unique items, like Compose
		}
		return nil
	}}
	merged := o.Merge(o.Own(base, "base.yaml"), o.Own(over, "over.yaml"), "override", rules)
	var got []string
	for _, l := range o.Emit(merged, Origin{}) {
		at := ""
		if l.At != nil {
			at = fmt.Sprintf("%s:%d", l.Origin.File, l.At.Range.Start.Line)
		}
		got = append(got, strings.TrimSpace(l.Text)+" @"+at)
	}
	// A merged key shows the overriding side's file AND line.
	want := []string{"a: @over.yaml:1", "x: 2 @over.yaml:2", "list: @over.yaml:3", "- 1 @base.yaml:3", "- 2 @over.yaml:3", "- 3 @over.yaml:3", "b: keep @base.yaml:4"}
	if !slices.Equal(got, want) {
		t.Errorf("merged:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
	// The inputs are unchanged.
	if base.Get("a").Get("x").Value != "1" || len(base.Get("a").Get("list").Items) != 2 {
		t.Error("Merge changed its input")
	}
}
