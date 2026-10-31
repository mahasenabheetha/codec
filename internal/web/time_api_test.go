package web

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestTimeAPI(t *testing.T) {
	s := New(testOptions(false))
	defer s.Close()
	post := func(kind, body string) (int, map[string]any) {
		rec := call(s, "POST", "/api/v2/time/"+kind, "127.0.0.1:8765", s.Token(), "", body)
		var out map[string]any
		json.Unmarshal(rec.Body.Bytes(), &out)
		return rec.Code, out
	}
	if code, out := post("timestamp", `{"input":"1770120309123","zone":"Europe/Stockholm"}`); code != 200 || out["kind"] != "milliseconds" || out["local"] != "2026-02-03T13:05:09.123+01:00" {
		t.Errorf("timestamp: %d %v", code, out)
	}
	if code, _ := post("timestamp", `{"input":"1","zone":"Mars/Base"}`); code != 422 {
		t.Errorf("bad zone: %d", code)
	}
	code, out := post("cron", `{"expr":"*/15 2-6 * * 1-5","count":3}`)
	if code != 200 || out["zone"] != "UTC" || len(out["runs"].([]any)) != 3 || !strings.HasPrefix(out["description"].(string), "Every 15 minutes") {
		t.Fatalf("cron: %d %v", code, out)
	}
	if f := out["fields"].([]any)[4].(map[string]any); f["meaning"] != "Monday to Friday" {
		t.Errorf("field meaning: %v", f)
	}
	if code, out := post("cron", `{"expr":"CRON_TZ=Asia/Tokyo @daily"}`); code != 200 || out["zone"] != "Asia/Tokyo" || len(out["runs"].([]any)) != 10 || out["form"] == nil {
		t.Errorf("CRON_TZ and default count: %d %v", code, out)
	}
	if code, out := post("cron", `{"expr":"0 0 12 * * ?"}`); code != 422 || out["dialect"] != "Quartz" {
		t.Errorf("quartz: %d %v", code, out)
	}
	if code, out := post("build", `{"form":{"kind":"weekly","time":"09:00","days":[1,2,3,4,5]}}`); code != 200 || out["expr"] != "0 9 * * 1-5" {
		t.Errorf("build: %d %v", code, out)
	}
	// The first zone is the one the input is read in.
	if code, out := post("zones", `{"input":"2026-10-31T12:00:00Z","zone":"Europe/Stockholm","zones":["Asia/Kolkata"]}`); code != 200 || len(out["zones"].([]any)) != 2 || out["zones"].([]any)[1].(map[string]any)["time"] != "17:30" {
		t.Errorf("zones: %d %v", code, out)
	}
	if code, _ := post("zones", `{"zones":["Mars/Olympus"]}`); code != 422 {
		t.Errorf("unknown zone: %d", code)
	}
	if code, out := post("formats", `{"input":"1793455509","zone":"UTC"}`); code != 200 || len(out["formats"].([]any)) < 10 || len(out["patterns"].([]any)) < 5 {
		t.Errorf("formats: %d %v", code, out)
	}
}
