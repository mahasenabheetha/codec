package web

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/mahasenabheetha/codec/v2/internal/config"
)

func TestSampleAndSettings(t *testing.T) {
	opts := testOptions(false)
	opts.SampleDir = filepath.Join(t.TempDir(), "sample")
	opts.Config.Update(func(st *config.Settings) {
		st.RecentFolders = []string{"/repos/a"}
		st.Helm = map[string]config.HelmChart{
			"/gone/chart": {Profiles: map[string]config.HelmProfile{"dev": {}, "prod": {}}, Active: "prod"},
		}
	})
	s := New(opts)
	defer s.Close()
	tok := s.Token()
	post := func(target, body string, out any) {
		t.Helper()
		rec := call(s, "POST", target, "127.0.0.1:8765", tok, "", body)
		if rec.Code != 200 {
			t.Fatalf("%s: %d %s", target, rec.Code, rec.Body)
		}
		json.Unmarshal(rec.Body.Bytes(), out)
	}

	// The sample opens like a folder but stays out of the recent list.
	var ws workspaceView
	post("/api/v2/workspace/sample", `{}`, &ws)
	if !ws.Open || !ws.Sample || ws.Root != opts.SampleDir || len(ws.Recent) != 1 {
		t.Fatalf("sample: %+v", ws)
	}
	rec := call(s, "GET", "/api/v2/files/content?path=charts/shop/Chart.yaml", "127.0.0.1:8765", tok, "", "")
	if rec.Code != 200 {
		t.Fatalf("sample file: %d %s", rec.Code, rec.Body)
	}

	var v settingsView
	json.Unmarshal(call(s, "GET", "/api/v2/settings", "127.0.0.1:8765", tok, "", "").Body.Bytes(), &v)
	if len(v.Folders) != 4 || v.Folders[3].ID != "sample" || !v.Folders[3].Exists || len(v.Helm) != 1 || v.Helm[0].Exists {
		t.Fatalf("settings: %+v", v)
	}

	// Forgetting the active profile clears it; the last one takes the chart.
	v = settingsView{} // Unmarshal merges into existing maps
	post("/api/v2/settings/forget", `{"helm":{"chart":"/gone/chart","profile":"prod"}}`, &v)
	if h := v.Helm[0]; len(h.Profiles) != 1 || h.Active != "" {
		t.Errorf("after forgetting prod: %+v", h)
	}
	post("/api/v2/settings/forget", `{"helm":{"chart":"/gone/chart","profile":"dev"}}`, &v)
	v = settingsView{}
	post("/api/v2/settings/forget", `{"recent":true}`, &v)
	if len(v.Helm) != 0 || len(v.Recent) != 0 {
		t.Errorf("after forgetting everything: %+v", v)
	}
}
