package web

import (
	"encoding/json"
	"net/http"
	"testing"
)

func TestTextAPI(t *testing.T) {
	s := New(testOptions(false))
	defer s.Close()
	rec := call(s, "POST", "/api/v2/text/case", "127.0.0.1:8765", s.Token(), "", `{"input":"getHTTPResponse"}`)
	var out struct {
		Words  []string `json:"words"`
		Cases  []struct{ Name, Value string }
		Checks []struct{ Rule, Fixed string }
	}
	json.Unmarshal(rec.Body.Bytes(), &out)
	if rec.Code != 200 || len(out.Words) != 3 || out.Cases[2].Value != "get_http_response" || len(out.Checks) != 5 {
		t.Errorf("case: %d %+v", rec.Code, out)
	}
	if rec := call(s, "POST", "/api/v2/text/case", "127.0.0.1:8765", s.Token(), "", `{"input":""}`); rec.Code != 200 || json.Valid(rec.Body.Bytes()) == false {
		t.Errorf("empty: %d %s", rec.Code, rec.Body)
	}
	if rec := call(s, "POST", "/api/v2/text/rot13", "127.0.0.1:8765", s.Token(), "", `{}`); rec.Code != http.StatusNotFound {
		t.Errorf("unknown kind: %d", rec.Code)
	}
}

func TestQuoteAPI(t *testing.T) {
	s := New(testOptions(false))
	defer s.Close()
	code, v := send(s, "POST", "/api/v2/text/quote", "application/json", `{"value":"it's","target":"bash","layers":[{"target":"bash","before":"ssh h "}]}`)
	forms := v["forms"].([]any)
	steps := v["steps"].([]any)
	if code != 200 || forms[0].(map[string]any)["value"] != `'it'\''s'` || steps[0].(map[string]any)["value"] != `ssh h 'it'\''s'` {
		t.Errorf("quote: %d %v", code, v)
	}
	if code, v := send(s, "POST", "/api/v2/text/quote", "application/json", `{"value":"'it'\\''s'","target":"bash","unquote":true}`); code != 200 || v["value"] != "it's" {
		t.Errorf("unquote: %d %v", code, v)
	}
	if code, _ := send(s, "POST", "/api/v2/text/quote", "application/json", `{"value":"x","target":"nope"}`); code != 422 {
		t.Errorf("unknown target: %d", code)
	}
	if code, v := send(s, "GET", "/api/v2/text/quote/targets", "", ""); code != 200 || len(v["targets"].([]any)) < 10 {
		t.Errorf("targets: %d %v", code, v)
	}
}
