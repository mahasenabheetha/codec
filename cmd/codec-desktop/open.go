package main

import (
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

// openRequest is a folder, or a file and the folder to open it in, from
// the command line (codec-desktop <path>, "Open with codec").
type openRequest struct {
	Root string
	File string // '/'-separated, relative to Root; "" for the folder alone
}

// requestFor reads the first argument that names an existing folder or
// file; flags and missing paths are skipped.
func requestFor(args []string, workDir string) (openRequest, bool) {
	for _, a := range args {
		if a == "" || strings.HasPrefix(a, "-") {
			continue
		}
		p := a
		if !filepath.IsAbs(p) && workDir != "" {
			p = filepath.Join(workDir, p)
		}
		info, err := os.Stat(p)
		if err != nil {
			continue
		}
		if info.IsDir() {
			return openRequest{Root: p}, true
		}
		return openRequest{Root: filepath.Dir(p), File: filepath.Base(p)}, true
	}
	return openRequest{}, false
}

// route is the page address that shows the request's file, for the
// first launch (the folder is opened before the page loads).
func (o openRequest) route() string {
	if o.File == "" {
		return ""
	}
	// Same shape as fileRoute in frontend/src/lib/stores/router.svelte.ts.
	parts := strings.Split(o.File, "/")
	for i, p := range parts {
		parts[i] = url.PathEscape(p)
	}
	return "#/file/" + strings.Join(parts, "/")
}
