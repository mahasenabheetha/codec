package web

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// multipartBody builds form data: name → content; an empty name is a
// pasted part (no filename).
func multipartBody(t *testing.T, parts [][2]string) (string, string) {
	var b bytes.Buffer
	mw := multipart.NewWriter(&b)
	for _, p := range parts {
		var w interface{ Write([]byte) (int, error) }
		var err error
		if p[0] == "" {
			w, err = mw.CreateFormField("text")
		} else {
			w, err = mw.CreateFormFile("file", p[0])
		}
		if err != nil {
			t.Fatal(err)
		}
		w.Write([]byte(p[1]))
	}
	mw.Close()
	return b.String(), mw.FormDataContentType()
}

func send(s *Server, method, target, ctype, body string) (int, map[string]any) {
	req := httptest.NewRequest(method, target, strings.NewReader(body))
	req.Host = "127.0.0.1:8765"
	req.Header.Set("X-Codec-Token", s.Token())
	if ctype != "" {
		req.Header.Set("Content-Type", ctype)
	}
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)
	var v map[string]any
	json.Unmarshal(rec.Body.Bytes(), &v)
	return rec.Code, v
}

const apiLog = `{"@timestamp":"2026-10-22T10:00:00Z","log.level":"info","message":"started","service.name":"api","http.status":200}
{"@timestamp":"2026-10-22T10:00:01Z","log.level":"error","message":"order 7 failed","error.message":"timeout","service.name":"api","http.status":500}
{"@timestamp":"2026-10-22T10:00:02Z","log.level":"error","message":"order 8 failed","error.message":"timeout","service.name":"api","http.status":500}
{"@timestamp":"2026-10-22T10:00:03Z","log.level":"info","message":"line one\nline two","service.name":"web"}
`

func openLog(t *testing.T, s *Server, parts [][2]string) string {
	body, ct := multipartBody(t, parts)
	code, v := send(s, "POST", "/api/v2/logs", ct, body)
	if code != 200 {
		t.Fatalf("open: %d %v", code, v)
	}
	return v["id"].(string)
}

func TestLogsAPI(t *testing.T) {
	s := New(testOptions(false))
	defer s.Close()
	body, ct := multipartBody(t, [][2]string{{"app.log", apiLog}, {"", "2026-10-22T10:00:01.5Z pasted line\n"}})
	code, v := send(s, "POST", "/api/v2/logs", ct, body)
	if code != 200 {
		t.Fatalf("open: %d %v", code, v)
	}
	id := v["id"].(string)
	sum := v["summary"].(map[string]any)
	files := v["files"].([]any)
	if sum["entries"] != 5.0 || sum["groups"] != 1.0 || len(files) != 2 || files[1].(map[string]any)["name"] != "Pasted" {
		t.Errorf("opened: %v", v)
	}

	entries := func(req string) (int, map[string]any) {
		return send(s, "POST", "/api/v2/logs/"+id+"/entries", "application/json", req)
	}
	code, v = entries(`{"q":"level:error http.status>=500","columns":["service.name"],"histogram":60}`)
	rows, _ := v["entries"].([]any)
	if code != 200 || v["total"] != 2.0 || len(rows) != 2 || v["histogram"] == nil {
		t.Fatalf("query: %d %v", code, v)
	}
	r0 := rows[0].(map[string]any)
	if r0["message"] != "order 7 failed" || r0["level"] != "error" || r0["cols"].(map[string]any)["service.name"] != "api" || r0["file"] != 0.0 {
		t.Errorf("row: %v", r0)
	}
	// Merged by time: the pasted line comes between the first two errors.
	code, v = entries(`{"q":"","offset":1,"limit":2}`)
	rows = v["entries"].([]any)
	if code != 200 || v["total"] != 5.0 || v["offset"] != 1.0 || len(rows) != 2 || rows[1].(map[string]any)["source"] != "Pasted" {
		t.Errorf("page: %v", v)
	}
	code, v = entries(`{"q":"","limit":2,"at":4}`)
	if code != 200 || v["offset"] != 4.0 {
		t.Errorf("at: %d %v", code, v)
	}
	code, v = entries(`{"q":"web"}`)
	// A row carries its message's lines, for reading it wrapped.
	if rows = v["entries"].([]any); code != 200 || len(rows) != 1 || rows[0].(map[string]any)["more"] != nil || rows[0].(map[string]any)["message"] != "line one\nline two" {
		t.Errorf("lines: %v", v)
	}
	code, v = entries(`{"q":"\"open"}`)
	if code != 422 || v["error"] != "this quote is not closed" || v["pos"] != 0.0 {
		t.Errorf("bad query: %d %v", code, v)
	}
	code, v = entries(`{"q":"","from":"yesterday"}`)
	if code != 422 {
		t.Errorf("bad time: %d %v", code, v)
	}
	code, v = entries(`{"from":"2026-10-22T10:00:01Z","to":"2026-10-22T10:00:02Z"}`)
	if code != 200 || v["total"] != 2.0 {
		t.Errorf("range: %d %v", code, v)
	}

	code, v = send(s, "GET", "/api/v2/logs/"+id+"/entries/1?before=1&after=1", "", "")
	e := v["entry"].(map[string]any)
	if code != 200 || e["error"] != "timeout" || e["file"] != "app.log" || len(e["fields"].([]any)) != 6 || len(v["around"].([]any)) != 2 {
		t.Errorf("entry: %d %v", code, v)
	}
	if code, _ := send(s, "GET", "/api/v2/logs/"+id+"/entries/99", "", ""); code != 404 {
		t.Errorf("no entry: %d", code)
	}

	code, v = send(s, "POST", "/api/v2/logs/"+id+"/groups", "application/json", `{"q":""}`)
	g := v["groups"].([]any)
	if code != 200 || len(g) != 1 || g[0].(map[string]any)["count"] != 2.0 || g[0].(map[string]any)["text"] != "order <n> failed · timeout" {
		t.Errorf("groups: %v", v)
	}
	code, v = send(s, "POST", "/api/v2/logs/"+id+"/fields", "application/json", `{"q":"service.name:api"}`)
	if code != 200 || v["total"] != 3.0 {
		t.Errorf("fields: %v", v)
	}

	exp := func(format string) (int, string, string) {
		req := httptest.NewRequest("POST", "/api/v2/logs/"+id+"/export", strings.NewReader(`{"q":"level:error","format":"`+format+`","columns":["http.status"]}`))
		req.Host = "127.0.0.1:8765"
		req.Header.Set("X-Codec-Token", s.Token())
		rec := httptest.NewRecorder()
		s.Handler().ServeHTTP(rec, req)
		return rec.Code, rec.Header().Get("Content-Disposition"), rec.Body.String()
	}
	if code, cd, out := exp("csv"); code != 200 || cd != `attachment; filename=log-filtered.csv` ||
		out != "time,level,source,message,http.status\n2026-10-22T10:00:01Z,error,app.log,order 7 failed,500\n2026-10-22T10:00:02Z,error,app.log,order 8 failed,500\n" {
		t.Errorf("csv: %d %q %q", code, cd, out)
	}
	if code, _, out := exp("ndjson"); code != 200 || !strings.Contains(out, `"fields":{"@timestamp":"2026-10-22T10:00:01Z","log.level":"error"`) || !strings.Contains(out, `"http.status":500}`) || strings.Count(out, "\n") != 2 {
		t.Errorf("ndjson: %d %s", code, out)
	}
	if code, _, out := exp("text"); code != 200 || strings.Count(out, "\n") != 2 || !strings.HasPrefix(out, `{"@timestamp":"2026-10-22T10:00:01Z"`) {
		t.Errorf("text: %d %s", code, out)
	}
	if code, _, _ := exp("xml"); code != 400 {
		t.Errorf("bad format: %d", code)
	}

	if code, _ := send(s, "DELETE", "/api/v2/logs/"+id, "", ""); code != 204 {
		t.Errorf("delete: %d", code)
	}
	code, v = entries(`{}`)
	if code != 410 || v["expired"] != true {
		t.Errorf("after delete: %d %v", code, v)
	}
}

func TestLogsView(t *testing.T) {
	s := New(testOptions(false))
	defer s.Close()
	id := openLog(t, s, [][2]string{{"run.log", strings.Join([]string{
		`{"level":"info","msg":"start","time":"2026-10-22T10:00:00Z"}`,
		"plain a", "plain b", "plain c",
		`{"level":"error","msg":"boom","time":"2026-10-22T10:00:04Z"}`,
		`{"level":"warn","msg":"retry in 5ms","time":"2026-10-22T10:00:05Z"}`,
		`{"level":"warn","msg":"retry in 7ms","time":"2026-10-22T10:00:06Z"}`,
		`{"level":"info","msg":"done","time":"2026-10-22T10:00:07Z"}`,
	}, "\n")}})
	entries := func(req string) map[string]any {
		code, v := send(s, "POST", "/api/v2/logs/"+id+"/entries", "application/json", req)
		if code != 200 {
			t.Fatalf("%s: %d %v", req, code, v)
		}
		return v
	}
	kinds := func(v map[string]any) string {
		var out []string
		for _, e := range v["entries"].([]any) {
			r := e.(map[string]any)
			k, _ := r["kind"].(string)
			if n, ok := r["n"].(float64); ok {
				k += fmt.Sprintf("×%d", int(n))
			}
			out = append(out, fmt.Sprintf("%v%s", r["i"], map[bool]string{true: ":" + k, false: ""}[k != ""]))
		}
		return strings.Join(out, " ")
	}

	v := entries(`{"q":"boom","view":{"context":1}}`)
	if kinds(v) != "0:gap×3 3:context 4 5:context 6:gap×2" || v["total"] != 1.0 || v["rows"] != 5.0 {
		t.Errorf("context: %s %v", kinds(v), v)
	}
	v = entries(`{"view":{"output":true,"repeats":"similar"},"highlights":["retry","\"open"],"markers":7,"pins":[7]}`)
	if kinds(v) != "0 1:output×3 4 5:repeat×2 7" || v["rows"] != 5.0 || v["total"] != 8.0 {
		t.Errorf("folds: %s", kinds(v))
	}
	rows := v["entries"].([]any)
	if r := rows[3].(map[string]any); r["hl"] != 1.0 || r["lastI"] != 6.0 || r["lastTime"] != "2026-10-22T10:00:06Z" {
		t.Errorf("repeat row: %v", r)
	}
	if he := v["highlightErrors"].([]any); len(he) != 2 || he[0] != "" || he[1] == "" {
		t.Errorf("highlight errors: %v", he)
	}
	if m := v["markers"].([]any); len(m) != 5 || m[2] != 1.0 || m[3] != float64(2|8) || m[4] != 4.0 {
		t.Errorf("markers: %v", m)
	}
	// Entry 2 is inside the output fold: its row is found.
	v = entries(`{"view":{"output":true},"at":2,"limit":2}`)
	if v["atRow"] != 1.0 || v["offset"] != 0.0 {
		t.Errorf("at in a fold: %v %v", v["atRow"], v["offset"])
	}
	// An opened fold is shown whole.
	v = entries(`{"view":{"output":true,"expanded":[1]}}`)
	if kinds(v) != "0 1 2 3 4 5 6 7" {
		t.Errorf("opened: %s", kinds(v))
	}
}

func TestLogsOpenErrors(t *testing.T) {
	s := New(testOptions(false))
	defer s.Close()
	body, ct := multipartBody(t, [][2]string{{"bin.dat", "abc\x00def"}})
	if code, v := send(s, "POST", "/api/v2/logs", ct, body); code != 422 {
		t.Errorf("binary: %d %v", code, v)
	}
	body, ct = multipartBody(t, nil)
	if code, _ := send(s, "POST", "/api/v2/logs", ct, body); code != 400 {
		t.Errorf("empty: %d", code)
	}
	if code, _ := send(s, "POST", "/api/v2/logs", "text/plain", "x"); code != 400 {
		t.Errorf("not form data: %d", code)
	}
	if code, _ := send(s, "POST", "/api/v2/logs", "application/json", `{"paths":["a.log"]}`); code != 409 {
		t.Errorf("no folder: %d", code)
	}
	// A gzip file is read as is.
	var gz bytes.Buffer
	zw := gzip.NewWriter(&gz)
	zw.Write([]byte(apiLog))
	zw.Close()
	body, ct = multipartBody(t, [][2]string{{"app.log.gz", gz.String()}})
	code, v := send(s, "POST", "/api/v2/logs", ct, body)
	if code != 200 || v["summary"].(map[string]any)["entries"] != 4.0 || v["files"].([]any)[0].(map[string]any)["encoding"].([]any)[0] != "gzip" {
		t.Errorf("gzip: %d %v", code, v)
	}
}

func TestLogsFromWorkspace(t *testing.T) {
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "run.log"), []byte(apiLog), 0o644)
	os.MkdirAll(filepath.Join(root, "sub"), 0o755)
	s := New(testOptions(false))
	defer s.Close()
	if err := s.OpenWorkspace(root); err != nil {
		t.Fatal(err)
	}
	code, v := send(s, "POST", "/api/v2/logs", "application/json", `{"paths":["run.log"]}`)
	if code != 200 || v["summary"].(map[string]any)["entries"] != 4.0 || v["files"].([]any)[0].(map[string]any)["name"] != "run.log" {
		t.Errorf("workspace log: %d %v", code, v)
	}
	for p, want := range map[string]int{"../x.log": 403, "nope.log": 404, "sub": 422} {
		if code, v := send(s, "POST", "/api/v2/logs", "application/json", fmt.Sprintf(`{"paths":[%q]}`, p)); code != want {
			t.Errorf("%s: %d %v", p, code, v)
		}
	}

	// A folder file and a dropped one together, in form data.
	var b bytes.Buffer
	mw := multipart.NewWriter(&b)
	mw.WriteField("path", "run.log")
	fw, _ := mw.CreateFormFile("file", "extra.log")
	fw.Write([]byte(`{"@timestamp":"2026-10-22T10:00:09Z","log.level":"info","message":"extra"}` + "\n"))
	mw.Close()
	code, v = send(s, "POST", "/api/v2/logs", mw.FormDataContentType(), b.String())
	if code != 200 || v["summary"].(map[string]any)["entries"] != 5.0 || len(v["files"].([]any)) != 2 {
		t.Errorf("a folder file and a dropped one: %d %v", code, v)
	}
	mw = multipart.NewWriter(&b)
	b.Reset()
	mw.WriteField("path", "../x.log")
	mw.Close()
	if code, _ := send(s, "POST", "/api/v2/logs", mw.FormDataContentType(), b.String()); code != 403 {
		t.Errorf("a path part outside the folder: %d", code)
	}
}

func TestLogsCompare(t *testing.T) {
	s := New(testOptions(false))
	defer s.Close()
	failed := openLog(t, s, [][2]string{{"today.log", apiLog}})
	passed := openLog(t, s, [][2]string{{"yesterday.log", strings.Replace(apiLog, "timeout", "refused", 2) + `{"@timestamp":"2026-10-22T10:00:04Z","log.level":"info","message":"done"}` + "\n"}})

	code, v := send(s, "POST", "/api/v2/logs/"+failed+"/compare", "application/json", `{"other":"`+passed+`"}`)
	if code != 200 {
		t.Fatalf("%d %v", code, v)
	}
	first := v["firstRows"].([]any)
	if v["first"].([]any)[0].(float64) != 1 || first[0].(map[string]any)["message"] != "order 7 failed" || first[1].(map[string]any)["message"] != "order 7 failed" {
		t.Errorf("first differences: %v %v", v["first"], first)
	}
	if n, m := len(v["new"].([]any)), len(v["missing"].([]any)); n != 1 || m != 2 {
		t.Errorf("%d new, %d missing: %v", n, m, v)
	}
	if v["sources"] != nil {
		t.Errorf("sources of logs without any: %v", v["sources"])
	}

	if code, v := send(s, "POST", "/api/v2/logs/"+failed+"/compare", "application/json", `{"other":"nope"}`); code != 410 || v["other"] != true {
		t.Errorf("a gone other log: %d %v", code, v)
	}
	if code, v := send(s, "POST", "/api/v2/logs/nope/compare", "application/json", `{"other":"`+passed+`"}`); code != 410 || v["other"] != nil {
		t.Errorf("a gone log: %d %v", code, v)
	}
	if code, _ := send(s, "POST", "/api/v2/logs/"+failed+"/compare", "application/json", `{}`); code != 400 {
		t.Errorf("nothing to compare with: %d", code)
	}
}

func TestLogLanes(t *testing.T) {
	s := New(testOptions(false))
	defer s.Close()
	id := openLog(t, s, [][2]string{{"a.log", apiLog}, {"b.log", apiLog}})
	code, v := send(s, "POST", "/api/v2/logs/"+id+"/lanes", "application/json", `{"q":"","buckets":60}`)
	if code != 200 || v["by"] != "source" || len(v["lanes"].([]any)) != 2 {
		t.Errorf("%d %v", code, v)
	}
	if code, _ := send(s, "POST", "/api/v2/logs/"+id+"/lanes", "application/json", `{"q":"\"open"}`); code != 422 {
		t.Errorf("bad query: %d", code)
	}
}

func TestArgoStatus(t *testing.T) {
	s := New(testOptions(false))
	defer s.Close()
	code, v := send(s, "POST", "/api/v2/argo/status", "application/json", `{"text":"status:\n  phase: Failed\n  nodes:\n    w: {id: w, name: w, type: Steps, phase: Failed}\n"}`)
	if code != 200 || v["name"] != "w" || v["phase"] != "Failed" || len(v["nodes"].([]any)) != 1 {
		t.Errorf("%d %v", code, v)
	}
	if code, _ := send(s, "POST", "/api/v2/argo/status", "application/json", `{"text":"kind: Pod"}`); code != 422 {
		t.Errorf("not a status: %d", code)
	}
}

func TestLogStore(t *testing.T) {
	st := newLogStore()
	st.idle = 20 * time.Millisecond
	s := New(testOptions(false))
	defer s.Close()
	id := openLog(t, s, [][2]string{{"a.log", apiLog}})
	h := s.logs.get(id)
	held, err := st.add(h.log)
	if err != nil {
		t.Fatal(err)
	}
	if st.get(held.id) == nil {
		t.Fatal("not held")
	}
	time.Sleep(100 * time.Millisecond)
	if st.get(held.id) != nil || st.total != 0 {
		t.Errorf("an idle log should be dropped: total %d", st.total)
	}
}
