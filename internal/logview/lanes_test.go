package logview

import (
	"fmt"
	"testing"
	"time"
)

func TestLanes(t *testing.T) {
	// An Argo run: a lane per step, in the order they start; a red cell
	// goes to its error, not the bucket's first entry.
	l := buildFixtures(t, "argo.log")
	ln := l.Lanes(nil, 120)
	if ln.By != "step" || len(ln.Lanes) != 2 || ln.Lanes[0].Name != "checkout" || ln.Lanes[1].Name != "test" || ln.N == 0 {
		t.Fatalf("lanes %+v", ln)
	}
	test := ln.Lanes[1]
	var red [5]int
	for _, c := range test.Cells {
		if c[3] > 0 {
			red = c
		}
	}
	if test.Errors == 0 || red[3] == 0 || l.Entry(red[4]).Level < Error {
		t.Errorf("test lane %+v", test)
	}

	// Elsewhere a lane per source; a query narrows them.
	var text string
	for i := range 6 {
		text += fmt.Sprintf("web-%d | {\"time\":\"2026-10-22T10:00:0%dZ\",\"level\":\"info\",\"msg\":\"x\"}\n", i%2, i)
	}
	l = buildText(t, text)
	if ln := l.Lanes(nil, 120); ln.By != "source" || len(ln.Lanes) != 2 || ln.Lanes[0].Entries != 3 {
		t.Errorf("sources %+v", ln)
	}
	m, _ := l.Match("source:web-1", time.Time{}, time.Time{})
	if ln := l.Lanes(m.IDs, 120); len(ln.Lanes) != 1 || ln.Lanes[0].Name != "web-1" {
		t.Errorf("a query: %+v", ln)
	}

	// No times: no lanes.
	if ln := buildText(t, "a\nb\n").Lanes(nil, 120); len(ln.Lanes) != 0 || ln.N != 0 {
		t.Errorf("untimed %+v", ln)
	}
}
