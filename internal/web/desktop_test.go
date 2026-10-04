package web

import (
	"net/http"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestOpenPath(t *testing.T) {
	s := New(testOptions(false))
	defer s.Close()
	events, unsubscribe := s.hub.subscribe()
	defer unsubscribe()

	root := t.TempDir()
	next := func() string { // the next open-file path, skipping other events
		for {
			select {
			case e := <-events:
				if e.name == "open-file" {
					return e.data.(map[string]string)["path"]
				}
			case <-time.After(5 * time.Second):
				t.Fatal("no open-file event")
			}
		}
	}

	if err := s.OpenPath(root, "app.yaml"); err != nil {
		t.Fatal(err)
	}
	if got := next(); got != "app.yaml" {
		t.Errorf("open-file path = %q, want app.yaml", got)
	}

	// A file deeper inside the open folder keeps that folder.
	if err := s.OpenPath(filepath.Join(root, "k8s", "base"), "deploy.yaml"); err != nil {
		t.Fatal(err)
	}
	if got := next(); got != "k8s/base/deploy.yaml" {
		t.Errorf("open-file path = %q, want k8s/base/deploy.yaml", got)
	}
	if v := s.view(); v.Root != root {
		t.Errorf("workspace = %q, want it kept at %q", v.Root, root)
	}
}

type fakeDesktop struct {
	dark   []bool
	picked string // what the folder dialog returns
	asked  [2]string
}

func (f *fakeDesktop) SetTheme(dark bool) { f.dark = append(f.dark, dark) }

func (f *fakeDesktop) PickFolder(title, start string) (string, error) {
	f.asked = [2]string{title, start}
	return f.picked, nil
}

func TestDesktopPickFolder(t *testing.T) {
	fake := &fakeDesktop{picked: `C:\repos\app`}
	opts := testOptions(false)
	opts.Desktop = fake
	s := New(opts)
	rec := call(s, "POST", "/api/v2/desktop/pick-folder", "127.0.0.1:8769", s.Token(), "", `{"title":"Open folder","start":"C:\\repos"}`)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"path":"C:\\repos\\app"`) {
		t.Fatalf("status %d, body %s", rec.Code, rec.Body)
	}
	if fake.asked != [2]string{"Open folder", `C:\repos`} {
		t.Errorf("dialog asked with %q", fake.asked)
	}
}

func TestDesktopTheme(t *testing.T) {
	const host = "127.0.0.1:8769"

	// codec serve: no desktop routes, no desktop meta.
	plain := New(testOptions(false))
	if rec := call(plain, "POST", "/api/v2/desktop/theme", host, plain.Token(), "", `{"theme":"dark"}`); rec.Code != http.StatusNotFound {
		t.Errorf("without Desktop: status = %d, want 404", rec.Code)
	}
	if page := call(plain, "GET", "/", host, "", "", "").Body.String(); strings.Contains(page, "codec-desktop") {
		t.Error("without Desktop: page carries the desktop meta")
	}

	fake := &fakeDesktop{}
	opts := testOptions(false)
	opts.Desktop = fake
	s := New(opts)
	tok := s.Token()
	for _, tt := range []struct {
		body string
		code int
	}{
		{`{"theme":"dark"}`, http.StatusNoContent},
		{`{"theme":"light"}`, http.StatusNoContent},
		{`{"theme":"blue"}`, http.StatusBadRequest},
		{`not json`, http.StatusBadRequest},
	} {
		if rec := call(s, "POST", "/api/v2/desktop/theme", host, tok, "", tt.body); rec.Code != tt.code {
			t.Errorf("%s: status = %d, want %d", tt.body, rec.Code, tt.code)
		}
	}
	if len(fake.dark) != 2 || !fake.dark[0] || fake.dark[1] {
		t.Errorf("SetTheme calls = %v, want [true false]", fake.dark)
	}
	if rec := call(s, "POST", "/api/v2/desktop/theme", host, "", "", `{"theme":"dark"}`); rec.Code != http.StatusUnauthorized && rec.Code != http.StatusForbidden {
		t.Errorf("no token: status = %d, want 401 or 403", rec.Code)
	}
	if page := call(s, "GET", "/", host, "", "", "").Body.String(); !strings.Contains(page, desktopMeta) {
		t.Error("with Desktop: page lacks the desktop meta")
	}
}
