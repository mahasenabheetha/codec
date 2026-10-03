package web

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"net"
	"net/http"
	"net/url"
	"strings"
)

// The server reads files, so a web page the user happens to visit must
// not be able to use it. Three checks, per design/architecture.md:
//
//  1. Host header must name this machine. Stops DNS rebinding, where
//     evil.example resolves to 127.0.0.1 and its page reads the API.
//  2. /api/v2 needs the per-run token that only our own page carries.
//     Other origins can't read our page, so they can't get the token.
//  3. Requests that change state must not come from a foreign Origin
//     (cross-site form posts, which don't need to read the response).

// newToken returns 32 random bytes as hex.
func newToken() string {
	b := make([]byte, 32)
	rand.Read(b) // never fails (crypto/rand panics instead since Go 1.24)
	return hex.EncodeToString(b)
}

// Token is the per-run secret the UI must send as X-Codec-Token.
func (s *Server) Token() string { return s.token }

// guard wraps every route with the checks above plus a few headers
// that stop the page being framed or content being sniffed.
func (s *Server) guard(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Referrer-Policy", "no-referrer")

		if !s.allowedHost(r.Host) {
			http.Error(w, "forbidden: unexpected Host header "+r.Host, http.StatusForbidden)
			return
		}
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			if o := r.Header.Get("Origin"); o != "" && !s.allowedOrigin(o) {
				http.Error(w, "forbidden: cross-origin request", http.StatusForbidden)
				return
			}
		}
		if strings.HasPrefix(r.URL.Path, "/api/v2/") && !s.validToken(r) {
			// A token that is present but wrong is almost always a page
			// opened before codec restarted: the UI offers a reload.
			resp := errorResponse{Error: "missing or invalid token; reload the page"}
			if requestToken(r) != "" {
				resp = errorResponse{Error: "codec was restarted; reload the page", Code: "stale-token"}
			}
			writeJSON(w, http.StatusUnauthorized, resp)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// allowedHost accepts loopback names on any port (Docker may map the
// port to another one) plus any extra hosts from Options.
func (s *Server) allowedHost(hostport string) bool {
	host, _, err := net.SplitHostPort(hostport)
	if err != nil {
		host = hostport // no port
	}
	host = strings.Trim(host, "[]")
	switch strings.ToLower(host) {
	case "localhost", "127.0.0.1", "::1":
		return true
	}
	for _, h := range s.opts.Hosts {
		if strings.EqualFold(h, host) {
			return true
		}
	}
	return false
}

// allowedOrigin applies the host rule to an Origin URL. Any loopback
// port is fine: the Vite dev server runs on its own port.
func (s *Server) allowedOrigin(origin string) bool {
	u, err := url.Parse(origin)
	return err == nil && (u.Scheme == "http" || u.Scheme == "https") && s.allowedHost(u.Host)
}

// validToken checks X-Codec-Token. EventSource can't set headers, so
// the event stream alone may pass it as ?token= instead.
func (s *Server) validToken(r *http.Request) bool {
	// Constant-time, so response timing can't leak the token byte by byte.
	return subtle.ConstantTimeCompare([]byte(requestToken(r)), []byte(s.token)) == 1
}

// requestToken is the token a request carries: the header, or the query
// string for the event stream (EventSource can't set headers).
func requestToken(r *http.Request) string {
	tok := r.Header.Get("X-Codec-Token")
	if tok == "" && r.URL.Path == "/api/v2/events" {
		tok = r.URL.Query().Get("token")
	}
	return tok
}
