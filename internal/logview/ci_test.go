package logview

import (
	"archive/zip"
	"bytes"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestReadCI(t *testing.T) {
	tests := []struct {
		line string
		want ciMark
	}{
		{"##[section]Starting: Build", ciMark{kind: ciSectionStart, name: "Build", msg: "Starting: Build", tool: true}},
		{"##[error]boom", ciMark{kind: ciLine, level: Error, msg: "boom"}},
		{"##[warning]careful", ciMark{kind: ciLine, level: Warn, msg: "careful"}},
		{"##vso[task.logissue type=warning;sourcepath=a/b.cs;linenumber=3;]old API", ciMark{kind: ciLine, level: Warn, msg: "old API", caller: "a/b.cs:3"}},
		{"::error file=x.ts,line=1,col=2::bad", ciMark{kind: ciLine, level: Error, msg: "bad", caller: "x.ts:1:2"}},
		{"::warning::no location", ciMark{kind: ciLine, level: Warn, msg: "no location"}},
		{"::group::Install", ciMark{kind: ciGroupStart, name: "Install", msg: "Install"}},
		{"[Pipeline] { (Test)", ciMark{kind: ciSectionStart, name: "Test", msg: "Stage Test", tool: true}},
		{"[Pipeline] // stage", ciMark{kind: ciSectionEnd, msg: "[Pipeline] // stage", tool: true}},
	}
	for _, tc := range tests {
		if got, ok := readCI(tc.line); !ok || got != tc.want {
			t.Errorf("readCI(%q) = %+v, want %+v", tc.line, got, tc.want)
		}
	}
	for _, s := range []string{"## heading", "::", "[INFO] build", "#5 [2/7] RUN make", "[Pipelines] x"} {
		if m, ok := readCI(s); ok {
			t.Errorf("readCI(%q) = %+v, want no CI command", s, m)
		}
	}
}

func TestCISteps(t *testing.T) {
	data, err := os.ReadFile("testdata/azure-pipelines.log")
	if err != nil {
		t.Fatal(err)
	}
	l, err := Build([]File{{Name: "run.log", Data: data}}, BuildOptions{Options: testOpts})
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, s := range l.Summary().Steps {
		got = append(got, s.Name+" "+strconv.Itoa(s.Entries)+"/"+strconv.Itoa(s.Errors))
	}
	// Steps in order, with their errors; the tool's lines fold as output.
	if strings.Join(got, ", ") != "Initialize job 3/0, Build 11/3" {
		t.Errorf("steps: %v", got)
	}
	if lanes := l.Lanes(nil, 10); lanes.By != "step" {
		t.Errorf("lanes by %s", lanes.By)
	}
}

func TestGitHubZip(t *testing.T) {
	var b bytes.Buffer
	zw := zip.NewWriter(&b)
	for _, f := range [][2]string{
		{"0_build.txt", "2026-11-06T10:00:00Z the whole job, again\n"},
		{"build/1_Set up job.txt", "2026-11-06T10:00:00Z Current runner version: 2.320.0\n"},
		{"build/10_Complete job.txt", "2026-11-06T10:00:09Z Cleaning up orphan processes\n"},
		{"build/2_Run npm test.txt", "2026-11-06T10:00:01Z ##[group]Run npm test\n2026-11-06T10:00:02Z ##[error]Process completed with exit code 1.\n"},
	} {
		w, _ := zw.Create(f[0])
		w.Write([]byte(f[1]))
	}
	zw.Close()
	l, err := Build([]File{{Name: "logs.zip", Data: b.Bytes()}}, BuildOptions{Options: testOpts})
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, s := range l.Summary().Steps {
		got = append(got, s.Name)
	}
	// Each step file is its step, in the steps' order; the job's whole
	// log is left out, as it repeats them.
	if strings.Join(got, ", ") != "Set up job, Run npm test, Complete job" || l.Len() != 4 || l.Entry(0).Source != "build" {
		t.Errorf("steps %v, %d entries, source %q", got, l.Len(), l.Entry(0).Source)
	}

	if _, err := Build([]File{{Name: "logs.zip", Data: b.Bytes()}}, BuildOptions{Options: testOpts, Limit: 20}); err != ErrTooLarge {
		t.Errorf("limit: %v", err)
	}
}

func TestBuildKitTook(t *testing.T) {
	data, err := os.ReadFile("testdata/buildkit.log")
	if err != nil {
		t.Fatal(err)
	}
	l, err := Build([]File{{Name: "build.log", Data: data}}, BuildOptions{Options: testOpts})
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, s := range l.Summary().Steps {
		got = append(got, s.Name+" "+strconv.FormatFloat(s.Took, 'f', -1, 64)+"s/"+strconv.Itoa(s.Errors))
	}
	want := "[internal] load build definition from Dockerfile 0s/0, [internal] load metadata for docker.io/library/golang:1.25 1.2s/0, " +
		"[1/4] FROM docker.io/library/golang:1.25@sha256:abc 0s/0, [2/4] COPY . /src 0.3s/0, [3/4] RUN go build -o /app ./cmd/shop 0s/1"
	if strings.Join(got, ", ") != want {
		t.Errorf("steps\n got %s\nwant %s", strings.Join(got, ", "), want)
	}
}

// The fields platform logs are read into answer the questions asked of
// them (T6).
func TestPlatformQueries(t *testing.T) {
	l := buildFixtures(t, "access.log", "envoy.log", "syslog.log", "k8s-events.txt", "k8s-audit.log")
	for _, c := range []struct{ q, want string }{
		{"status>=500", "access:2 envoy:2"},
		{"latency>1", "access:2"},
		{"path:/api/* method:POST", "access:2 envoy:2"},
		{"upstream:shop-api-80", "access:2"},
		{"flags:*", "envoy:2 envoy:3"},
		{"level>=error source:shop", "syslog:2"},
		{"reason:BackOff", "k8s-events.txt:3"},
		{"source:bob", "k8s-audit:1"},
	} {
		m, err := l.Match(c.q, time.Time{}, time.Time{})
		if err != nil {
			t.Errorf("%s: %v", c.q, err)
			continue
		}
		if got := where(l, m.IDs); got != c.want {
			t.Errorf("%s:\ngot  %s\nwant %s", c.q, got, c.want)
		}
	}
}
