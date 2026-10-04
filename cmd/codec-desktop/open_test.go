//go:build windows

package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRequestFor(t *testing.T) {
	dir := t.TempDir()
	file := filepath.Join(dir, "my values.yaml")
	if err := os.WriteFile(file, []byte("a: 1\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name  string
		args  []string
		wd    string
		want  openRequest
		ok    bool
		route string
	}{
		{"none", nil, "", openRequest{}, false, ""},
		{"flags and missing paths skipped", []string{"--x", filepath.Join(dir, "nope"), dir}, "", openRequest{Root: dir}, true, ""},
		{"file opens its folder", []string{file}, "", openRequest{Root: dir, File: "my values.yaml"}, true, "#/file/my%20values.yaml"},
		{"relative to the working folder", []string{"my values.yaml"}, dir, openRequest{Root: dir, File: "my values.yaml"}, true, "#/file/my%20values.yaml"},
	}
	for _, tt := range tests {
		got, ok := requestFor(tt.args, tt.wd)
		if ok != tt.ok || got != tt.want {
			t.Errorf("%s: requestFor = %+v, %v; want %+v, %v", tt.name, got, ok, tt.want, tt.ok)
		}
		if r := got.route(); r != tt.route {
			t.Errorf("%s: route = %q, want %q", tt.name, r, tt.route)
		}
	}
}
