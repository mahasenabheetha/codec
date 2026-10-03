package web

import (
	"encoding/json"
	"testing"
)

func TestRegexAPI(t *testing.T) {
	s := New(testOptions(false))
	defer s.Close()
	post := func(body string) map[string]any {
		rec := call(s, "POST", "/api/v2/regex", "127.0.0.1:8765", s.Token(), "", body)
		if rec.Code != 200 {
			t.Fatalf("%d %s", rec.Code, rec.Body.String())
		}
		var out map[string]any
		json.Unmarshal(rec.Body.Bytes(), &out)
		return out
	}
	out := post(`{"pattern":"(\\d+) ms","all":true,"text":"120 ms, 5 ms","replace":"${1}ms"}`)
	if m := out["result"].(map[string]any)["matches"].([]any); len(m) != 2 || out["replaced"] != "120ms, 5ms" || len(out["explain"].([]any)) == 0 {
		t.Errorf("run: %v", out)
	}
	// A pattern RE2 refuses: an answer with the reason, a hint and the explanation.
	out = post(`{"pattern":"a(?=b)","text":"ab"}`)
	e, _ := out["error"].(map[string]any)
	if e == nil || e["hint"] == "" || out["result"] != nil || len(out["explain"].([]any)) != 2 {
		t.Errorf("re2 refusal: %v", out)
	}
	if out = post(`{"pattern":"a(?=b)","style":"pcre","text":"ab"}`); out["error"] != nil {
		t.Errorf("pcre lookahead: %v", out)
	}
}
