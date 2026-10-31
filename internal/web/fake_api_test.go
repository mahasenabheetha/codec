package web

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestFakeAPI(t *testing.T) {
	s := New(testOptions(false))
	defer s.Close()
	do := func(method, path, body string) (int, map[string]any) {
		rec := call(s, method, path, "127.0.0.1:8765", s.Token(), "", body)
		var out map[string]any
		json.Unmarshal(rec.Body.Bytes(), &out)
		return rec.Code, out
	}
	code, out := do("POST", "/api/v2/fake/lorem", `{"unit":"words","count":3,"classic":true}`)
	if code != 200 || out["output"] != "lorem ipsum dolor" || out["seed"].(float64) == 0 {
		t.Errorf("lorem: %d %v", code, out)
	}
	body := `{"columns":[{"name":"id","kind":"id"},{"name":"email","kind":"email"}],"count":2,"format":"csv","seed":7}`
	_, a := do("POST", "/api/v2/fake/rows", body)
	_, b := do("POST", "/api/v2/fake/rows", body)
	if a["output"] != b["output"] || !strings.HasPrefix(a["output"].(string), "id,email\n1,") || a["seed"] != 7.0 {
		t.Errorf("rows: %v %v", a, b)
	}
	if code, _ := do("POST", "/api/v2/fake/rows", `{"columns":[{"name":"x","kind":"nope"}],"count":1}`); code != 422 {
		t.Errorf("bad kind: %d", code)
	}
	if code, out := do("GET", "/api/v2/fake/kinds", ""); code != 200 || len(out["kinds"].([]any)) < 20 || out["presets"].(map[string]any)["users"] == nil {
		t.Errorf("kinds: %d %v", code, out)
	}
	if code, _ := do("POST", "/api/v2/fake/nope", `{}`); code != 404 {
		t.Errorf("unknown job: %d", code)
	}
}
