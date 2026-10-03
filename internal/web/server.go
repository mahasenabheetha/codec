// Package web serves the browser UI for codec. Like the cli package,
// it is a presentation layer only: it translates HTTP requests into
// calls on the engine and adapter packages and the results back into
// JSON.
//
// It is a local server that can read files, so every request passes
// the guard in security.go first (design/architecture.md, Security).
package web

import (
	"bytes"
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/mahasenabheetha/codec/v2/internal/check"
	"github.com/mahasenabheetha/codec/v2/internal/config"
	"github.com/mahasenabheetha/codec/v2/internal/provider"
	"github.com/mahasenabheetha/codec/v2/internal/version"
	"github.com/mahasenabheetha/codec/v2/internal/workspace"
)

// distFiles holds the Vite build of frontend/ (written to dist/app by
// `npm run build`), compiled INTO the binary so the shipped executable
// needs no files next to it. The "all:" prefix also embeds the
// committed dist/.keep, so the package compiles even before the
// frontend has ever been built — go:embed rejects an empty directory.
//
//go:embed all:dist
var distFiles embed.FS

// notBuiltPage is served in place of the UI when the binary was
// compiled without a frontend build.
const notBuiltPage = `<!doctype html>
<html><head><title>codec</title></head>
<body style="background:#0b0d12;color:#e6e8ee;font-family:system-ui;padding:2rem">
<h1>Frontend not built</h1>
<p>This binary was compiled without the web UI. Run
<code>npm --prefix frontend ci &amp;&amp; npm --prefix frontend run build</code>,
then rebuild codec.</p>
</body></html>`

// Options configures a Server.
type Options struct {
	// Config stores recent folders; nil keeps them in memory only.
	Config *config.Store
	// Poll watches files by polling instead of native OS events.
	Poll bool
	// Hosts are extra Host header names to accept besides loopback,
	// e.g. the address given to --host.
	Hosts []string
	// SampleDir is where the sample workspace is written; "" = codec's
	// cache folder (sample.Dir).
	SampleDir string
}

// Server is the codec web server and the state it holds: the per-run
// token, the open workspace and the event subscribers.
type Server struct {
	opts  Options
	token string
	hub   *hub
	helm  helmCache
	argo  argoCache
	check *check.Checker // lint and schema settings live here

	mu    sync.Mutex // guards the workspace fields
	ws    *workspace.Workspace
	stop  context.CancelFunc // stops ws's watcher
	watch string             // watcher mode for ws

	handler http.Handler
}

// New builds a server. No workspace is open until OpenWorkspace or the
// UI opens one.
func New(opts Options) *Server {
	if opts.Config == nil {
		opts.Config = config.Memory()
	}
	s := &Server{opts: opts, token: newToken(), hub: newHub()}
	s.check = check.New(provider.Default, opts.Config.Get().Lint, version.Get().Version)
	s.handler = s.guard(s.routes())
	return s
}

// Handler is the full route table behind the security guard. Tests
// call it directly, without opening a network port.
func (s *Server) Handler() http.Handler { return s.handler }

func (s *Server) routes() http.Handler {
	mux := http.NewServeMux()

	// The embedded FS is rooted above "dist/app"; re-root it so the
	// browser requests /index.html, not /dist/app/index.html.
	appRoot, err := fs.Sub(distFiles, "dist/app")
	if err != nil {
		// Unreachable unless the literal path above is malformed,
		// which would be a programming mistake, not a runtime one.
		panic(err)
	}
	mux.Handle("/", appHandler(appRoot, s.token))

	// v1
	mux.HandleFunc("POST /api/transform", handleTransform)
	mux.HandleFunc("GET /api/version", handleVersion)
	// Non-POST requests to the API path would otherwise fall through
	// to the "/" file server above and produce a confusing 404. This
	// method-less pattern is more specific than "/" (so it wins for
	// GET etc.) but less specific than "POST /api/transform" (so real
	// API calls never land here).
	mux.HandleFunc("/api/transform", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Allow", "POST")
		http.Error(w, "method not allowed; use POST", http.StatusMethodNotAllowed)
	})

	// v2 (token required, see guard)
	mux.HandleFunc("GET /api/v2/workspace", s.handleWorkspace)
	mux.HandleFunc("POST /api/v2/workspace/open", s.handleOpen)
	mux.HandleFunc("POST /api/v2/workspace/sample", s.handleOpenSample)
	mux.HandleFunc("GET /api/v2/fs/dirs", s.handleDirs)
	mux.HandleFunc("GET /api/v2/files/tree", s.handleTree)
	mux.HandleFunc("GET /api/v2/files/content", s.handleContent)
	mux.HandleFunc("GET /api/v2/events", s.handleEvents)
	mux.HandleFunc("POST /api/v2/files/diff", s.handleDiff)
	mux.HandleFunc("POST /api/v2/yaml/analyze", s.handleAnalyze)
	mux.HandleFunc("POST /api/v2/yaml/hover", s.handleHover)
	mux.HandleFunc("POST /api/v2/yaml/definition", s.handleDefinition)
	mux.HandleFunc("POST /api/v2/yaml/complete", s.handleComplete)
	mux.HandleFunc("POST /api/v2/yaml/path", s.handlePath)
	mux.HandleFunc("GET /api/v2/helm/charts", s.handleHelmCharts)
	mux.HandleFunc("POST /api/v2/helm/render", s.handleHelmRender)
	mux.HandleFunc("POST /api/v2/helm/profiles", s.handleHelmProfiles)
	mux.HandleFunc("GET /api/v2/lint/settings", s.handleLintSettings)
	mux.HandleFunc("POST /api/v2/lint/settings", s.handleSaveLintSettings)
	mux.HandleFunc("POST /api/v2/lint/workspace", s.handleLintWorkspace)
	mux.HandleFunc("GET /api/v2/settings", s.handleSettings)
	mux.HandleFunc("POST /api/v2/settings/forget", s.handleSettingsForget)
	mux.HandleFunc("POST /api/v2/compare", s.handleCompare)
	mux.HandleFunc("POST /api/v2/search", s.handleSearch)
	mux.HandleFunc("GET /api/v2/compare/ignore", s.handleDiffIgnore)
	mux.HandleFunc("POST /api/v2/compare/ignore", s.handleDiffIgnore)
	mux.HandleFunc("POST /api/v2/query", s.handleQuery)
	mux.HandleFunc("POST /api/v2/k8s/analyze", s.handleK8sAnalyze)
	mux.HandleFunc("POST /api/v2/k8s/neat", s.handleK8sNeat)
	mux.HandleFunc("POST /api/v2/argo/analyze", s.handleArgoAnalyze)
	mux.HandleFunc("POST /api/v2/ci/analyze", s.handleCIAnalyze)
	mux.HandleFunc("POST /api/v2/compose/analyze", s.handleComposeAnalyze)
	mux.HandleFunc("POST /api/v2/ansible/analyze", s.handleAnsibleAnalyze)
	mux.HandleFunc("POST /api/v2/ansible/find-task", s.handleFindTask)
	mux.HandleFunc("POST /api/v2/ansible/log", s.handleAnsibleLog)
	mux.HandleFunc("POST /api/v2/encode/{kind}", s.handleEncode)
	mux.HandleFunc("POST /api/v2/time/{kind}", s.handleTime)
	mux.HandleFunc("POST /api/v2/regex", s.handleRegex)
	mux.HandleFunc("GET /api/v2/scaffold/starters", s.handleStarters)
	mux.HandleFunc("POST /api/v2/scaffold/render", s.handleScaffoldRender)
	mux.HandleFunc("POST /api/v2/scaffold/check", s.handleScaffoldCheck)
	mux.HandleFunc("POST /api/v2/scaffold/zip", s.handleScaffoldZip)
	mux.HandleFunc("POST /api/v2/scaffold/clone", s.handleScaffoldClone)
	mux.HandleFunc("/api/v2/", func(w http.ResponseWriter, r *http.Request) {
		writeError(w, http.StatusNotFound, "no such endpoint: "+r.Method+" "+r.URL.Path)
	})

	// The v2 UI was previewed under /app/ before it replaced v1;
	// send old bookmarks and installed shortcuts to the real page.
	mux.Handle("/app/", http.RedirectHandler("/", http.StatusMovedPermanently))
	return mux
}

// appHandler serves the built frontend from fsys, or a short "not
// built" page when fsys holds no index.html. Either way the page
// carries the per-run token in a <meta> tag, which the frontend sends
// back on every /api/v2 call. It takes the FS as a parameter so tests
// can exercise both cases without a real build.
func appHandler(fsys fs.FS, token string) http.Handler {
	meta := `<meta name="codec-token" content="` + token + `">`
	index, err := fs.ReadFile(fsys, "index.html")
	if err != nil {
		page := strings.Replace(notBuiltPage, "<head>", "<head>"+meta, 1)
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.Header().Set("Cache-Control", "no-store")
			w.WriteHeader(http.StatusServiceUnavailable)
			fmt.Fprint(w, page)
		})
	}
	page := bytes.Replace(index, []byte("<head>"), []byte("<head>\n    "+meta), 1)
	files := http.FileServer(http.FS(fsys))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" && r.URL.Path != "/index.html" {
			files.ServeHTTP(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		// The token changes every run; a cached page would hold a dead one.
		w.Header().Set("Cache-Control", "no-store")
		w.Write(page)
	})
}

// Serve answers requests on ln until ctx is cancelled, then shuts down.
// Request contexts derive from ctx, so long-lived event streams end
// with it instead of holding shutdown open.
func (s *Server) Serve(ctx context.Context, ln net.Listener) error {
	srv := &http.Server{
		Handler:           s.handler,
		ReadHeaderTimeout: 10 * time.Second,
		BaseContext:       func(net.Listener) context.Context { return ctx },
	}
	errc := make(chan error, 1)
	go func() { errc <- srv.Serve(ln) }()

	select {
	case err := <-errc:
		s.Close()
		return err
	case <-ctx.Done():
		shutdown, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		err := srv.Shutdown(shutdown)
		s.Close()
		if errors.Is(err, context.DeadlineExceeded) {
			return nil // a client still held a connection; we're exiting anyway
		}
		return err
	}
}

// Close stops the watcher and closes the open workspace.
func (s *Server) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.stop != nil {
		s.stop()
	}
	if s.ws != nil {
		s.ws.Close()
	}
	s.ws, s.stop, s.watch = nil, nil, ""
}

// errorResponse is returned for any failure. Line and Column point at
// the problem in the input when there is one (JSON syntax errors).
type errorResponse struct {
	Error  string `json:"error"`
	Line   int    `json:"line,omitempty"`
	Column int    `json:"column,omitempty"`
	Code   string `json:"code,omitempty"` // machine-readable, e.g. "stale-token"
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, errorResponse{Error: msg})
}

// writeJSON sends v as a JSON response with the given status code.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		// Too late to tell the client anything — the status line is
		// already sent. Log it and move on.
		log.Printf("write response: %v", err)
	}
}
