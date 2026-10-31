package httpkit

import (
	"encoding/base64"
	"errors"
	"fmt"
	"net/url"
	"path"
	"strconv"
	"strings"
)

// Options that take a value, short and long.
var (
	shortArg = "XHdubAeFoxmwTcrEKy"
	longArg  = map[string]bool{
		"request": true, "header": true, "data": true, "data-raw": true, "data-binary": true, "data-ascii": true,
		"data-urlencode": true, "json": true, "user": true, "cookie": true, "user-agent": true, "referer": true,
		"form": true, "form-string": true, "output": true, "proxy": true, "max-time": true, "connect-timeout": true,
		"write-out": true, "upload-file": true, "cookie-jar": true, "range": true, "url": true, "cacert": true,
		"cert": true, "key": true, "retry": true, "oauth2-bearer": true, "resolve": true, "interface": true,
		"limit-rate": true, "unix-socket": true, "aws-sigv4": true, "proxy-user": true, "connect-to": true,
		"config": true, "speed-time": true, "speed-limit": true, "max-redirs": true, "retry-delay": true,
		"retry-max-time": true, "trace": true, "trace-ascii": true, "stderr": true, "dump-header": true,
	}
	// Flags about curl's own output and progress: nothing to carry over.
	quiet = map[string]bool{
		"s": true, "S": true, "v": true, "i": true, "o": true, "O": true, "w": true, "f": true, "#": true, "n": true,
		"silent": true, "show-error": true, "verbose": true, "include": true, "output": true, "remote-name": true,
		"write-out": true, "fail": true, "fail-with-body": true, "progress-bar": true, "no-progress-meter": true,
		"compressed": true, "location": true, "L": true, "globoff": true, "g": true, "http1.1": true, "http2": true,
		"http2-prior-knowledge": true, "no-buffer": true, "N": true, "location-trusted": true, "max-redirs": true,
		"trace": true, "trace-ascii": true, "stderr": true, "dump-header": true, "connect-timeout": true,
		"remote-header-name": true, "J": true, "create-dirs": true, "netrc": true, "4": true, "6": true,
		"ipv4": true, "ipv6": true, "tr-encoding": true, "no-keepalive": true, "path-as-is": true,
	}
)

// curlOpt is one option of the command, by its short or long name.
type curlOpt struct {
	name, value string
}

// ParseCurl reads a curl command: copied from a browser's "Copy as
// cURL", a terminal or docs, over several lines or one.
func ParseCurl(cmd string) (*Request, error) {
	words, notes, err := splitShell(cmd)
	if err != nil {
		return nil, err
	}
	// Text that isn't UTF-8 (pasted, or made with $'\x80') can't
	// round-trip through JavaScript strings.
	for i, w := range words {
		words[i] = strings.ToValidUTF8(w, "\uFFFD")
	}
	if len(words) == 0 {
		return nil, errors.New("paste a curl command")
	}
	if b := strings.ToLower(path.Base(strings.ReplaceAll(words[0], `\`, "/"))); b != "curl" && b != "curl.exe" {
		return nil, fmt.Errorf("this reads curl commands, not %q", words[0])
	}
	r := &Request{Notes: notes}
	var opts []curlOpt
	var urls []string
	for i := 1; i < len(words); i++ {
		w := words[i]
		next := func() (string, bool) {
			if i+1 < len(words) {
				i++
				return words[i], true
			}
			return "", false
		}
		switch {
		case w == "--":
			urls = append(urls, words[i+1:]...)
			i = len(words)
		case strings.HasPrefix(w, "--"):
			name, val, has := strings.Cut(w[2:], "=")
			if !has && longArg[name] {
				if val, has = next(); !has {
					return nil, fmt.Errorf("--%s needs a value", name)
				}
			}
			opts = append(opts, curlOpt{name, val})
		case strings.HasPrefix(w, "-") && len(w) > 1:
			for j := 1; j < len(w); j++ {
				c := string(w[j])
				if !strings.Contains(shortArg, c) {
					opts = append(opts, curlOpt{c, ""})
					continue
				}
				val := w[j+1:]
				if val == "" {
					var ok bool
					if val, ok = next(); !ok {
						return nil, fmt.Errorf("-%s needs a value", c)
					}
				}
				opts = append(opts, curlOpt{c, val})
				break
			}
		default:
			urls = append(urls, w)
		}
	}

	var data []string
	getMode, head, jsonMode := false, false, false
	for _, o := range opts {
		switch o.name {
		case "X", "request":
			r.Method = strings.ToUpper(o.value)
		case "H", "header":
			name, val, ok := strings.Cut(o.value, ":")
			if !ok {
				if n, empty := strings.CutSuffix(o.value, ";"); empty {
					r.Headers = append(r.Headers, Header{strings.TrimSpace(n), ""})
				} else {
					r.note(fmt.Sprintf("Header %q has no colon; left out.", o.value))
				}
				continue
			}
			r.add(strings.TrimSpace(name), val)
		case "d", "data", "data-ascii", "data-binary", "data-raw":
			if strings.HasPrefix(o.value, "@") && o.name != "data-raw" {
				r.note(fmt.Sprintf("%s reads the file %s; paste the file's content in its place.", flag(o.name), o.value[1:]))
				continue
			}
			v := o.value
			if o.name != "data-binary" && o.name != "data-raw" {
				v = strings.NewReplacer("\r", "", "\n", "").Replace(v) // -d drops line breaks, as curl does
			}
			data = append(data, v)
		case "data-urlencode":
			name, content, hasEq := strings.Cut(o.value, "=")
			if !hasEq {
				name, content = "", o.value
			}
			if strings.HasPrefix(content, "@") || (!hasEq && strings.HasPrefix(o.value, "@")) {
				r.note(fmt.Sprintf("--data-urlencode %s reads a file; paste its content instead.", o.value))
				continue
			}
			enc := url.QueryEscape(content)
			if name != "" {
				enc = name + "=" + enc
			}
			data = append(data, enc)
		case "json":
			jsonMode = true
			data = append(data, o.value)
		case "F", "form", "form-string":
			name, val, _ := strings.Cut(o.value, "=")
			file := o.name != "form-string" && (strings.HasPrefix(val, "@") || strings.HasPrefix(val, "<"))
			if file {
				val = strings.TrimLeft(val, "@<")
				r.note(fmt.Sprintf("The form field %q attaches the file %s; add the file yourself.", name, val))
			}
			r.Form = append(r.Form, Field{Name: name, Value: val, File: file})
		case "u", "user":
			if !strings.Contains(o.value, ":") {
				r.note("-u without a password makes curl ask for one; the Authorization header has none.")
			}
			r.add("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(o.value)))
		case "oauth2-bearer":
			r.add("Authorization", "Bearer "+o.value)
		case "b", "cookie":
			if !strings.Contains(o.value, "=") {
				r.note(fmt.Sprintf("-b %s reads cookies from a file; add them as a Cookie header.", o.value))
				continue
			}
			r.add("Cookie", o.value)
		case "A", "user-agent":
			r.add("User-Agent", o.value)
		case "e", "referer":
			r.add("Referer", o.value)
		case "url":
			urls = append(urls, o.value)
		case "I", "head":
			head = true
		case "G", "get":
			getMode = true
		case "m", "max-time":
			switch s, err := strconv.ParseFloat(o.value, 64); {
			case err != nil || s <= 0:
				r.note(fmt.Sprintf("--max-time %s isn't a number of seconds; left out.", o.value))
			case s > 86400:
				r.note("A --max-time over a day is left out.")
			case s*1000 >= 1:
				r.TimeoutMS = int(s * 1000)
			}
		case "k", "insecure":
			r.note("fetch can't turn off certificate checks as -k does (in Node, NODE_TLS_REJECT_UNAUTHORIZED=0 does, for the whole process).")
		case "x", "proxy":
			r.note("fetch has no proxy option; the browser's or the system's proxy is used.")
		case "T", "upload-file":
			r.note(fmt.Sprintf("-T uploads the file %s; send its content as the body yourself.", o.value))
		case "cacert", "cert", "key", "E":
			r.note(fmt.Sprintf("%s sets TLS certificates, which fetch takes from the browser or system.", flag(o.name)))
		case "retry":
			r.note("fetch doesn't retry; --retry is left out.")
		default:
			if !quiet[o.name] {
				r.note(fmt.Sprintf("%s isn't converted.", flag(o.name)))
			}
		}
	}

	if len(urls) == 0 {
		return nil, errors.New("the command has no URL")
	}
	r.URL = urls[0]
	if len(urls) > 1 {
		r.note(fmt.Sprintf("curl requests %d URLs; only the first is used.", len(urls)))
	}
	if !strings.Contains(r.URL, "://") {
		r.URL = "http://" + r.URL
		r.note("The URL has no scheme; curl uses http://, so it is added.")
	}
	body := strings.Join(data, "&")
	if jsonMode {
		body = strings.Join(data, "")
		r.setDefault("Content-Type", "application/json")
		r.setDefault("Accept", "application/json")
	}
	switch {
	case getMode && body != "":
		sep := "?"
		if strings.Contains(r.URL, "?") {
			sep = "&"
		}
		r.URL += sep + body
		body = ""
	case body != "" && len(r.Form) > 0:
		r.note("curl refuses -d together with -F; the form is used.")
		body = ""
	}
	r.Body = body
	if r.Method == "" {
		switch {
		case head:
			r.Method = "HEAD"
		case getMode:
			r.Method = "GET"
		case r.Body != "" || len(r.Form) > 0:
			r.Method = "POST"
		default:
			r.Method = "GET"
		}
	}
	if r.Body != "" {
		r.setDefault("Content-Type", "application/x-www-form-urlencoded") // what curl sends with -d
	}
	return r, nil
}

// add adds a header as curl sends it: the space before the value is
// dropped, and an empty value means no header.
func (r *Request) add(name, value string) {
	if value = strings.TrimLeft(value, " \t"); value != "" {
		r.Headers = append(r.Headers, Header{name, value})
	}
}

func flag(name string) string {
	if len(name) == 1 {
		return "-" + name
	}
	return "--" + name
}

// Curl writes r as a curl command over several lines.
func (r *Request) Curl() string {
	parts := []string{"curl " + shellQuote(r.URL)}
	switch {
	case r.Method == "HEAD" && r.Body == "" && len(r.Form) == 0:
		parts = append(parts, "-I")
	case r.Method == "POST" && (r.Body != "" || len(r.Form) > 0):
	case r.Method != "GET" || r.Body != "" || len(r.Form) > 0:
		parts = append(parts, "-X "+shellQuote(r.Method))
	}
	for _, h := range r.Headers {
		if h.Value == "" {
			parts = append(parts, "-H "+shellQuote(h.Name+";"))
		} else {
			parts = append(parts, "-H "+shellQuote(h.Name+": "+h.Value))
		}
	}
	if r.Body != "" {
		parts = append(parts, "--data-raw "+shellQuote(r.Body))
	}
	for _, f := range r.Form {
		if f.File {
			parts = append(parts, "-F "+shellQuote(f.Name+"=@"+f.Value))
		} else {
			parts = append(parts, "--form-string "+shellQuote(f.Name+"="+f.Value))
		}
	}
	if r.TimeoutMS > 0 {
		parts = append(parts, "--max-time "+strconv.FormatFloat(float64(r.TimeoutMS)/1000, 'f', -1, 64))
	}
	return strings.Join(parts, " \\\n  ")
}
