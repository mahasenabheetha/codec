package web

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

func TestHTTPAPI(t *testing.T) {
	s := New(testOptions(false))
	defer s.Close()
	post := func(kind, input string) (int, map[string]any) {
		body, _ := json.Marshal(map[string]string{"input": input})
		rec := call(s, "POST", "/api/v2/http/"+kind, "127.0.0.1:8765", s.Token(), "", string(body))
		var out map[string]any
		json.Unmarshal(rec.Body.Bytes(), &out)
		return rec.Code, out
	}
	if code, out := post("curl", `curl -X PUT https://x.io -d a=1`); code != 200 || !strings.Contains(out["output"].(string), `method: "PUT"`) {
		t.Errorf("curl: %d %v", code, out)
	}
	if code, out := post("fetch", `fetch("https://x.io", { method: "DELETE" })`); code != 200 || !strings.Contains(out["output"].(string), "-X 'DELETE'") {
		t.Errorf("fetch: %d %v", code, out)
	}
	// Empty lists are [], not null: the UI counts them.
	if _, out := post("curl", "curl https://x.io"); out["request"].(map[string]any)["notes"] == nil || out["request"].(map[string]any)["headers"] == nil {
		t.Errorf("null lists: %v", out)
	}
	if code, out := post("status", "429"); code != 200 || len(out["statuses"].([]any)) != 1 {
		t.Errorf("status: %d %v", code, out)
	}
	if code, _ := post("curl", "wget x"); code != http.StatusUnprocessableEntity {
		t.Errorf("not curl: %d", code)
	}
	if code, _ := post("soap", ""); code != http.StatusNotFound {
		t.Errorf("unknown kind: %d", code)
	}
}
