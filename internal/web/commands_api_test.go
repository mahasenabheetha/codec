package web

import (
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
}
