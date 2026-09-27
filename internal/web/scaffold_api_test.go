package web

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestScaffold(t *testing.T) {
	// Personal templates come from the settings folder.
	cfgDir := t.TempDir()
	t.Setenv("CODEC_CONFIG_DIR", cfgDir)
	os.MkdirAll(filepath.Join(cfgDir, "templates"), 0o755)
	os.WriteFile(filepath.Join(cfgDir, "templates", "note.yaml"), []byte("note: <% .text %>\n"), 0o644)

	s := New(testOptions(false))
	tok := s.Token()
	post := func(target, body string) []byte {
		t.Helper()
		rec := call(s, "POST", target, "127.0.0.1:8765", tok, "", body)
		if rec.Code != 200 {
			t.Fatalf("%s: %d %s", target, rec.Code, rec.Body)
		}
		return rec.Body.Bytes()
	}

	var list struct {
		Starters []struct {
			ID       string `json:"id"`
			Personal bool   `json:"personal"`
		} `json:"starters"`
		PersonalDir string `json:"personalDir"`
	}
	rec := call(s, "GET", "/api/v2/scaffold/starters", "127.0.0.1:8765", tok, "", "")
	json.Unmarshal(rec.Body.Bytes(), &list)
	last := list.Starters[len(list.Starters)-1]
	if len(list.Starters) < 10 || last.ID != "personal/note.yaml" || !last.Personal || list.PersonalDir != filepath.Join(cfgDir, "templates") {
		t.Errorf("starters: %+v", list)
	}

	// A chart is rendered as well as linted.
	var res checkResult
	json.Unmarshal(post("/api/v2/scaffold/render", `{"id":"helm-chart","values":{"name":"web"}}`), &res)
	if len(res.Files) != 7 || len(res.Charts) != 1 || res.Charts[0].Chart != "web" || !strings.Contains(res.Charts[0].Manifest, "kind: Deployment") {
		t.Fatalf("chart: %+v", res)
	}
	for _, f := range res.Files {
		if len(f.Diagnostics) > 0 {
			t.Errorf("%s: %+v", f.Path, f.Diagnostics)
		}
	}
	if len(res.Charts[0].Diagnostics) > 0 {
		t.Errorf("render: %+v", res.Charts[0].Diagnostics)
	}

	// Form problems come back instead of files.
	var bad struct {
		FieldErrors []struct{ Field string } `json:"fieldErrors"`
	}
	json.Unmarshal(post("/api/v2/scaffold/render", `{"id":"k8s-app","values":{"name":"Bad Name"}}`), &bad)
	if len(bad.FieldErrors) != 1 || bad.FieldErrors[0].Field != "name" {
		t.Errorf("field errors: %+v", bad)
	}

	// An edited file is checked again: a broken template shows up.
	res = checkResult{}
	json.Unmarshal(post("/api/v2/scaffold/check", `{"files":[{"path":"c/Chart.yaml","content":"apiVersion: v2\nname: c\nversion: 0.1.0\n"},{"path":"c/templates/x.yaml","content":"a: {{ .Values.nope.deeper }}\n"}]}`), &res)
	if len(res.Charts) != 1 || len(res.Charts[0].Diagnostics) != 2 || res.Charts[0].Diagnostics[1].File != "c/templates/x.yaml" {
		t.Errorf("edited chart: %+v", res.Charts)
	}

	// The zip holds the files with their folders.
	rec = call(s, "POST", "/api/v2/scaffold/zip", "127.0.0.1:8765", tok, "", `{"name":"web","files":[{"path":"web/Chart.yaml","content":"name: web\n"}]}`)
	zr, err := zip.NewReader(bytes.NewReader(rec.Body.Bytes()), int64(rec.Body.Len()))
	if err != nil || len(zr.File) != 1 || zr.File[0].Name != "web/Chart.yaml" || rec.Header().Get("Content-Disposition") != `attachment; filename="web.zip"` {
		t.Errorf("zip: %v %+v", err, rec.Header())
	}
	if rec := call(s, "POST", "/api/v2/scaffold/zip", "127.0.0.1:8765", tok, "", `{"files":[{"path":"../x","content":""}]}`); rec.Code != 400 {
		t.Errorf("zip with ../: %d", rec.Code)
	}

	// Clone works on a buffer and checks the copy.
	var cl struct {
		Result struct {
			Text string `json:"text"`
		} `json:"result"`
		Docs []struct{ Index int } `json:"docs"`
		Type string                `json:"type"`
	}
	json.Unmarshal(post("/api/v2/scaffold/clone", `{"path":"a.yaml","content":"apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: a\ndata:\n  x: a-b\n","doc":-1,"to":"z"}`), &cl)
	if cl.Result.Text != "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: z\ndata:\n  x: z-b\n" || cl.Type != "kubernetes" || len(cl.Docs) != 1 {
		t.Errorf("clone: %+v", cl)
	}
}
