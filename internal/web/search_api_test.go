package web

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

func TestSearchAPI(t *testing.T) {
	opts := testOptions(false)
	opts.SampleDir = filepath.Join(t.TempDir(), "sample")
	s := New(opts)
	defer s.Close()
	post := func(body string) (int, string) {
		rec := call(s, "POST", "/api/v2/search", "127.0.0.1:8765", s.Token(), "", body)
		return rec.Code, rec.Body.String()
	}
	if code, body := post(`{"query":"shop"}`); code != 409 {
		t.Fatalf("no folder: %d %s", code, body)
	}
	call(s, "POST", "/api/v2/workspace/sample", "127.0.0.1:8765", s.Token(), "", `{}`)
	if code, body := post(`{"query":"(","regex":true}`); code != 422 {
		t.Errorf("bad regex: %d %s", code, body)
	}

	code, body := post(`{"query":"name: shop","globs":["charts/**"]}`)
	var res struct {
		Matches []struct {
			Path string
			Line int
		}
		Files int
	}
	json.Unmarshal([]byte(body), &res)
	if code != 200 || res.Files == 0 || !strings.HasPrefix(res.Matches[0].Path, "charts/") {
		t.Fatalf("search: %d %s", code, body)
	}
	for _, m := range res.Matches {
		if !strings.HasPrefix(m.Path, "charts/") || m.Line < 1 {
			t.Errorf("match outside the glob or without a line: %+v", m)
		}
	}
}
