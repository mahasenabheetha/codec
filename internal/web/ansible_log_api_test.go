package web

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

func TestAnsibleLog(t *testing.T) {
	opts := testOptions(false)
	opts.SampleDir = filepath.Join(t.TempDir(), "sample")
	s := New(opts)
	defer s.Close()
	post := func(body string) (int, map[string]any) {
		rec := call(s, "POST", "/api/v2/ansible/log", "127.0.0.1:8765", s.Token(), "", body)
		var v map[string]any
		json.Unmarshal(rec.Body.Bytes(), &v)
		return rec.Code, v
	}

	// No folder needed; the raw text is the body.
	code, v := post("$ ansible-playbook site.yml\nPLAY [web] ***\nTASK [ping] ***\nfatal: [web-1]: FAILED! => {\"msg\": \"down\"}\nPLAY RECAP ***\nweb-1 : ok=0 changed=0 unreachable=0 failed=1\n")
	if code != 200 {
		t.Fatalf("status %d: %v", code, v)
	}
	sum := v["summary"].(map[string]any)
	if sum["runs"] != 1.0 || sum["failed"] != 1.0 || sum["firstFailure"] == nil || len(v["blocks"].([]any)) != 2 {
		t.Errorf("analysis: %v", v)
	}

	if code, _ := post("abc\x00def"); code != 422 {
		t.Errorf("binary: %d", code)
	}
	if code, _ := post(strings.Repeat("x", maxLog+1)); code != 413 {
		t.Errorf("too big: %d", code)
	}
}
