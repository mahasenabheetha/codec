package web

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Schedules in the sample workspace explain themselves on hover.
func TestCronHover(t *testing.T) {
	opts := testOptions(false)
	opts.SampleDir = filepath.Join(t.TempDir(), "sample")
	s := New(opts)
	defer s.Close()
	call(s, "POST", "/api/v2/workspace/sample", "127.0.0.1:8765", s.Token(), "", `{}`)
	hover := func(path, find string) map[string]string {
		t.Helper()
		data, err := os.ReadFile(filepath.Join(opts.SampleDir, filepath.FromSlash(path)))
		if err != nil {
			t.Fatal(err)
		}
		content := string(data)
		line := 0
		for i, l := range strings.Split(content, "\n") {
			if strings.Contains(l, find) {
				line = i + 1
				break
			}
		}
		col := strings.Index(strings.Split(content, "\n")[line-1], find) + 2
		body, _ := json.Marshal(map[string]any{"path": path, "line": line, "col": col})
		rec := call(s, "POST", "/api/v2/yaml/hover", "127.0.0.1:8765", s.Token(), "", string(body))
		var out struct {
			Hover *struct {
				Title string
				Rows  []struct{ Label, Value string }
			}
		}
		json.Unmarshal(rec.Body.Bytes(), &out)
		if out.Hover == nil || out.Hover.Title != "Cron schedule" {
			t.Fatalf("%s: %s", path, rec.Body.String())
		}
		rows := map[string]string{}
		for _, r := range out.Hover.Rows {
			if _, seen := rows[r.Label]; !seen {
				rows[r.Label] = r.Value
			}
		}
		return rows
	}
	r := hover("k8s/base/cleanup.yaml", "*/30 1-5")
	if r["Runs"] != "Every 30 minutes, from 01:00 to 05:59, Monday to Friday" || !strings.HasPrefix(r["Time zone"], "Europe/Stockholm · from spec.timeZone") || r["Next"] == "" {
		t.Errorf("CronJob: %v", r)
	}
	r = hover(".github/workflows/ci.yml", "0 3 * * 1-5")
	if !strings.HasPrefix(r["Time zone"], "UTC · GitHub") || !strings.Contains(r["Note"], "default branch") {
		t.Errorf("GitHub: %v", r)
	}
	r = hover("azure-pipelines.yml", "0 3 * * 0")
	if r["Runs"] != "At 03:00, on Sunday" || !strings.Contains(r["Time zone"], "Azure") {
		t.Errorf("Azure: %v", r)
	}
	r = hover("argo/nightly.yaml", "0 2 * * *")
	if !strings.HasPrefix(r["Time zone"], "Europe/Stockholm") {
		t.Errorf("Argo: %v", r)
	}
}
