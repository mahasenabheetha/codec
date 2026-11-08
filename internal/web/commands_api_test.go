package web

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestCommandsAPI(t *testing.T) {
	s := New(testOptions(false))
	defer s.Close()

	code, v := send(s, "GET", "/api/v2/commands/tools", "", "")
	if code != 200 || len(v["tools"].([]any)) < 2 {
		t.Fatalf("tools: %d %v", code, v)
	}
	code, v = send(s, "GET", "/api/v2/commands/tools/git", "", "")
	if code != 200 || v["name"] != "git" || v["group"] != "git" || len(v["commands"].([]any)) == 0 {
		t.Errorf("git: %d %v", code, v["name"])
	}
	if code, _ := send(s, "GET", "/api/v2/commands/tools/nope", "", ""); code != 404 {
		t.Errorf("unknown tool: %d", code)
	}

	// "é" is one UTF-16 unit and two bytes: --amend starts at 18 in
	// units (19 in bytes).
	code, v = send(s, "POST", "/api/v2/commands/explain", "application/json", `{"line":"git commit -m \"é\" --amend"}`)
	parts := v["parts"].([]any)
	last := parts[len(parts)-1].(map[string]any)
	seg := v["segments"].([]any)[0].(map[string]any)
	if code != 200 || len(parts) != 5 || last["start"] != 18.0 || last["kind"] != "flag" || last["danger"] == nil || seg["summary"] == "" {
		t.Errorf("explain: %d %v", code, v)
	}
	if code, _ := send(s, "POST", "/api/v2/commands/explain", "application/json", `{"line":"`+strings.Repeat("x", 70000)+`"}`); code != 422 {
		t.Errorf("long line: %d", code)
	}
	// The builder's reading, in the same units: "é" again.
	_, v = send(s, "POST", "/api/v2/commands/explain", "application/json", `{"line":"A=é git -C r log"}`)
	u := v["segments"].([]any)[0].(map[string]any)["use"].(map[string]any)
	if u["toolStart"] != 4.0 || u["end"] != 16.0 || u["flags"].([]any)[0].(map[string]any)["value"] != "r" {
		t.Errorf("use: %v", u)
	}
}

func TestCommandSuggest(t *testing.T) {
	opts := testOptions(false)
	opts.SampleDir = filepath.Join(t.TempDir(), "sample") // not the user's own sample folder
	s := New(opts)
	defer s.Close()
	if _, v := send(s, "GET", "/api/v2/commands/suggest", "", ""); len(v["namespaces"].([]any)) != 0 {
		t.Errorf("no folder: %v", v)
	}
	if err := s.OpenSample(); err != nil {
		t.Fatal(err)
	}
	_, v := send(s, "GET", "/api/v2/commands/suggest", "", "")
	if len(v["resources"].([]any)) == 0 || len(v["charts"].([]any)) == 0 || v["charts"].([]any)[0] != "./charts/shop" {
		t.Errorf("sample: %v", v)
	}
}

func TestCommandSearch(t *testing.T) {
	s := New(testOptions(false))
	defer s.Close()
	code, v := send(s, "GET", "/api/v2/commands/search?q=undo+my+last+commit", "", "")
	res := v["results"].([]any)
	first := res[0].(map[string]any)
	if code != 200 || first["kind"] != "recipe" || first["tool"] != "git" || len(first["why"].([]any)) == 0 {
		t.Errorf("search: %d %v", code, v)
	}
	if _, v := send(s, "GET", "/api/v2/commands/search?tool=kubectl", "", ""); len(v["results"].([]any)) == 0 {
		t.Errorf("browse: %v", v)
	}
	if _, v := send(s, "GET", "/api/v2/commands/search?q=zzzqqq", "", ""); len(v["results"].([]any)) != 0 {
		t.Errorf("nothing: %v", v)
	}
	if code, _ := send(s, "GET", "/api/v2/commands/search?q="+strings.Repeat("x", 600), "", ""); code != 422 {
		t.Errorf("long query: %d", code)
	}
}
