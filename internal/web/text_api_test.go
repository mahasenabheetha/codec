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
