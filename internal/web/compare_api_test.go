package web

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mahasenabheetha/codec/v2/internal/textdiff"
)

type compareView struct {
	Mode    string         `json:"mode"`
	Note    string         `json:"note"`
	Diff    string         `json:"diff"`
	Rows    []textdiff.Row `json:"rows"`
	Changes []struct {
		Kind string `json:"kind"`
		Path string `json:"path"`
	} `json:"changes"`
	Left struct {
		Title string `json:"title"`
	} `json:"left"`
}

func TestComparePaste(t *testing.T) {
	opts := testOptions(false)
	opts.SampleDir = filepath.Join(t.TempDir(), "sample")
	s := New(opts)
	defer s.Close()
	compare := func(body string) compareView {
		t.Helper()
		rec := call(s, "POST", "/api/v2/compare", "127.0.0.1:8765", s.Token(), "", body)
		if rec.Code != 200 {
			t.Fatalf("%s: %d %s", body, rec.Code, rec.Body)
		}
		var v compareView
		json.Unmarshal(rec.Body.Bytes(), &v)
		return v
	}
	paste := func(text string) string {
		b, _ := json.Marshal(map[string]string{"kind": "paste", "content": text})
		return string(b)
	}

	// Two pasted sides need no folder; YAML and JSON mix.
	v := compare(`{"left":` + paste("a: 1\nb: [x, y]\n") + `,"right":` + paste(`{"b":["y","x"],"a":2}`) + `}`)
	if v.Mode != "semantic" || len(v.Changes) != 1 || v.Changes[0].Path != "a" || v.Left.Title != "Pasted" {
		t.Errorf("paste × paste: %+v", v)
	}

	// Plain text is compared as text, not as one changed scalar.
	v = compare(`{"left":` + paste("first line\nsecond line\n") + `,"right":` + paste("first line\nSecond  line\n") + `}`)
	if v.Mode != "text" || !strings.Contains(v.Diff, "+Second  line") {
		t.Errorf("scalar → text: %+v", v)
	}
	if r := v.Rows; len(r) != 2 || r[1].Kind != "change" || len(r[1].Right.Spans) == 0 {
		t.Errorf("side-by-side rows: %+v", r)
	}
	v = compare(`{"left":` + paste("first line\nsecond line\n") + `,"right":` + paste("first line\nSecond  line\n") + `,"ignoreSpace":true,"ignoreCase":true}`)
	if v.Mode != "text" || v.Diff != "" || v.Rows == nil || len(v.Rows) != 0 {
		t.Errorf("ignore space and case: %+v", v)
	}

	// Forced modes.
	if v = compare(`{"left":` + paste("a: 1\n") + `,"right":` + paste("a: 2\n") + `,"mode":"text"}`); v.Mode != "text" || v.Diff == "" {
		t.Errorf("mode text: %+v", v)
	}
	if v = compare(`{"left":` + paste("one") + `,"right":` + paste("two") + `,"mode":"structure"}`); v.Mode != "semantic" || len(v.Changes) != 1 {
		t.Errorf("mode structure: %+v", v)
	}

	// A file side needs a folder.
	file := `{"kind":"file","path":"charts/shop/Chart.yaml"}`
	if rec := call(s, "POST", "/api/v2/compare", "127.0.0.1:8765", s.Token(), "", `{"left":`+file+`,"right":`+paste("a: 1")+`}`); rec.Code != 409 {
		t.Errorf("file with no folder: %d %s", rec.Code, rec.Body)
	}
	if rec := call(s, "POST", "/api/v2/workspace/sample", "127.0.0.1:8765", s.Token(), "", `{}`); rec.Code != 200 {
		t.Fatalf("sample: %d %s", rec.Code, rec.Body)
	}
	if v = compare(`{"left":` + file + `,"right":` + paste("apiVersion: v2\nname: renamed\n") + `}`); v.Mode != "semantic" || len(v.Changes) == 0 {
		t.Errorf("paste × file: %+v", v)
	}
}
