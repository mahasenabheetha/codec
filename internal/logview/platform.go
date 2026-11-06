package logview

import (
	"cmp"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Platform logs (decision 99, T6): web servers' and proxies' access
// logs, syslog and journald, and Kubernetes events and audit logs. Each
// is read into fields a query can use (status>=500, latency>0.5,
// path:/api), with a level that says how the request or event went.

var (
	// The NCSA common and combined formats of Nginx, Apache and
	// ingress-nginx: client, user, [time], "request", status, bytes,
	// then "referer" "agent" and whatever the server adds.
	accessRe = regexp.MustCompile(`^(\S+) \S+ (\S+) \[(\d\d/\w{3}/\d{4}:\d\d:\d\d:\d\d [+-]\d{4})\] "(?:(\S+) (\S+)(?: (HTTP/[\d.]+))?|[^"]*)" (\d{3}) (\d+|-)(?: "([^"]*)" "([^"]*)")?(.*)$`)
	// ingress-nginx's default after the agent: request length and time,
	// [upstream name], [alternative], upstream address, length, time,
	// status, request id.
	ingressRest = regexp.MustCompile(`^ (\d+) ([\d.]+) \[([^\]]*)\] \[([^\]]*)\] (\S+) (\S+) (\S+) (\S+) (\S+)`)
	// Envoy's and Istio's default access log, after its [time]: "request"
	// status flags received sent duration upstream-time "xff" "agent"
	// "request id" "authority" "upstream host", then Istio's cluster.
	envoyRe = regexp.MustCompile(`^"(\S+) (\S+) (\S+)" (\d{1,3}|-) (\S+) (\d+) (\d+) (\d+|-) (\d+|-) "([^"]*)" "([^"]*)" "([^"]*)" "([^"]*)" "([^"]*)"(?: (\S+))?`)

	// RFC 5424: <pri>1 time host app procid msgid [sd] message.
	syslog5424 = regexp.MustCompile(`^<(\d{1,3})>1 (\S+) (\S+) (\S+) (\S+) (\S+) ((?:\[[^\]]*\])+|-) ?(.*)$`)
	// RFC 3164 (BSD) and what /var/log/syslog holds: [<pri>]Mmm dd
	// hh:mm:ss host app[pid]: message.
	syslog3164 = regexp.MustCompile(`^(?:<(\d{1,3})>)?([A-Z][a-z]{2} [ \d]\d \d\d:\d\d:\d\d) (\S+) ([^\s:\[]+)(?:\[(\d+)\])?: (.*)$`)

	// kubectl get events: the table's header, with or without -A and -o wide.
	eventsHead = regexp.MustCompile(`^(?:NAMESPACE\s+)?LAST SEEN\s+TYPE\s+REASON\s+OBJECT\s.*MESSAGE`)
)

// syslogLevels are the severities 0 (emergency) to 7 (debug).
var syslogLevels = [8]Level{Fatal, Fatal, Fatal, Error, Warn, Info, Info, Debug}

// eventCols are the columns of a kubectl get events table, by where
// each starts in the header.
type eventCols struct {
	names  []string
	starts []int
}

// platform reads a text line of a platform log; ok is false for any
// other line.
func (r *reader) platform(s string) (Entry, bool) {
	if r.events != nil {
		if e, ok := r.events.row(s); ok {
			return e, true
		}
		r.events = nil // the table ended
	}
	switch c := s[0]; {
	case c == '"':
		if m := envoyRe.FindStringSubmatch(s); m != nil {
			return envoy(m), true
		}
	case c == '<':
		if m := syslog5424.FindStringSubmatch(s); m != nil {
			return rfc5424(m), true
		}
		if m := syslog3164.FindStringSubmatch(s); m != nil {
			return r.rfc3164(m), true
		}
	case c >= 'A' && c <= 'Z':
		if eventsHead.MatchString(s) {
			r.events = newEventCols(s)
			return Entry{Format: Text, Message: s, Tool: true}, true
		}
		if m := syslog3164.FindStringSubmatch(s); m != nil {
			return r.rfc3164(m), true
		}
	}
	if strings.Contains(s, `] "`) {
		if m := accessRe.FindStringSubmatch(s); m != nil {
			return access(m), true
		}
	}
	return Entry{}, false
}

func num(k, v string) Field {
	n, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return Field{Key: k, Text: v}
	}
	return Field{Key: k, Kind: Number, Text: v, Num: n}
}

func str(k, v string) Field { return Field{Key: k, Text: v} }

// statusLevel: a server error is an error, a client error a warning.
func statusLevel(code string) Level {
	switch {
	case len(code) != 3:
		return LevelNone
	case code[0] == '5':
		return Error
	case code[0] == '4':
		return Warn
	}
	return Info
}

// request writes a request as its message: "GET /api/cart 500".
func request(method, path, status string) string {
	if method == "" {
		return status
	}
	return method + " " + path + " " + status
}

func access(m []string) Entry {
	e := Entry{Format: Text, Level: statusLevel(m[7]), Message: request(m[4], m[5], m[7])}
	e.Time, _ = time.Parse("02/Jan/2006:15:04:05 -0700", m[3])
	path, query, _ := strings.Cut(m[5], "?")
	e.Fields = append(e.Fields, str("client", m[1]))
	if m[2] != "-" {
		e.Fields = append(e.Fields, str("user", m[2]))
	}
	if m[4] != "" {
		e.Fields = append(e.Fields, str("method", m[4]), str("path", path))
		if query != "" {
			e.Fields = append(e.Fields, str("query", query))
		}
		if m[6] != "" {
			e.Fields = append(e.Fields, str("protocol", m[6]))
		}
	}
	e.Fields = append(e.Fields, num("status", m[7]))
	if m[8] != "-" {
		e.Fields = append(e.Fields, num("bytes", m[8]))
	}
	if m[9] != "" && m[9] != "-" {
		e.Fields = append(e.Fields, str("referer", m[9]))
	}
	if m[10] != "" && m[10] != "-" {
		e.Fields = append(e.Fields, str("agent", m[10]))
	}
	if x := ingressRest.FindStringSubmatch(m[11]); x != nil {
		// ingress-nginx: the upstream is the request's source.
		e.Fields = append(e.Fields, num("request_length", x[1]), num("latency", x[2]))
		if x[3] != "" {
			e.Source = x[3]
			e.Fields = append(e.Fields, str("upstream", x[3]))
		}
		e.Fields = append(e.Fields, str("upstream_addr", x[5]), num("upstream_latency", x[7]), num("upstream_status", x[8]))
		e.TraceID = x[9]
	}
	return e
}

func envoy(m []string) Entry {
	e := Entry{Format: Text, Level: statusLevel(m[4]), Message: request(m[1], m[2], m[4])}
	if m[5] != "-" && (m[4] == "-" || m[4] == "0" || e.Level < Warn) {
		// A response flag (UF, UH, NR …) with no error status: Envoy
		// couldn't serve it.
		e.Level = Error
	}
	path, query, _ := strings.Cut(m[2], "?")
	e.Fields = append(e.Fields, str("method", m[1]), str("path", path))
	if query != "" {
		e.Fields = append(e.Fields, str("query", query))
	}
	e.Fields = append(e.Fields, str("protocol", m[3]), num("status", m[4]))
	if m[5] != "-" {
		e.Fields = append(e.Fields, str("flags", m[5]))
	}
	e.Fields = append(e.Fields, num("bytes_received", m[6]), num("bytes", m[7]))
	if ms, err := strconv.ParseFloat(m[8], 64); err == nil {
		// The access log's milliseconds, as seconds like the others.
		e.Fields = append(e.Fields, num("latency", strconv.FormatFloat(ms/1000, 'f', -1, 64)))
	}
	if ms, err := strconv.ParseFloat(m[9], 64); err == nil {
		e.Fields = append(e.Fields, num("upstream_latency", strconv.FormatFloat(ms/1000, 'f', -1, 64)))
	}
	for _, kv := range [][2]string{{"client", m[10]}, {"agent", m[11]}, {"authority", m[13]}, {"upstream_addr", m[14]}} {
		if kv[1] != "" && kv[1] != "-" {
			e.Fields = append(e.Fields, str(kv[0], kv[1]))
		}
	}
	if m[12] != "-" {
		e.TraceID = m[12]
	}
	if m[15] != "" && m[15] != "-" {
		e.Source = m[15] // Istio's upstream cluster
		e.Fields = append(e.Fields, str("upstream", m[15]))
	}
	return e
}

// priority splits a syslog <pri> into facility and severity fields and
// the severity's level.
func priority(pri string, e *Entry) {
	p, err := strconv.Atoi(pri)
	if err != nil || p > 191 {
		return
	}
	e.Level = syslogLevels[p%8]
	e.Fields = append(e.Fields, num("facility", strconv.Itoa(p/8)), num("severity", strconv.Itoa(p%8)))
}

func rfc5424(m []string) Entry {
	e := Entry{Format: Text, Message: strings.TrimPrefix(m[8], "\ufeff")}
	e.Time, _ = parseTime(m[2])
	priority(m[1], &e)
	for _, kv := range [][2]string{{"host", m[3]}, {"app", m[4]}, {"pid", m[5]}, {"msgid", m[6]}, {"sd", m[7]}} {
		if kv[1] != "-" {
			e.Fields = append(e.Fields, str(kv[0], kv[1]))
		}
	}
	if m[4] != "-" {
		e.Source = m[4]
	}
	return e
}

func (r *reader) rfc3164(m []string) Entry {
	e := Entry{Format: Text, Message: m[6], Source: m[4]}
	if t, err := time.Parse("Jan _2 15:04:05", m[2]); err == nil {
		// The year isn't written: the log's, else this year.
		e.Time = t.AddDate(r.year, 0, 0)
	}
	priority(m[1], &e)
	if e.Level == LevelNone {
		var t Entry
		readText(m[6], r.day, &t)
		e.Level, e.RawLevel = t.Level, t.RawLevel
	}
	e.Fields = append(e.Fields, str("host", m[3]), str("app", m[4]))
	if m[5] != "" {
		e.Fields = append(e.Fields, num("pid", m[5]))
	}
	return e
}

func newEventCols(head string) *eventCols {
	c := &eventCols{}
	for _, name := range []string{"NAMESPACE", "LAST SEEN", "TYPE", "REASON", "OBJECT", "SUBOBJECT", "SOURCE", "MESSAGE", "FIRST SEEN", "COUNT", "NAME"} {
		if i := column(head, name); i >= 0 {
			c.names, c.starts = append(c.names, name), append(c.starts, i)
		}
	}
	// In the header's order.
	for i := 1; i < len(c.starts); i++ {
		for j := i; j > 0 && c.starts[j] < c.starts[j-1]; j-- {
			c.starts[j], c.starts[j-1] = c.starts[j-1], c.starts[j]
			c.names[j], c.names[j-1] = c.names[j-1], c.names[j]
		}
	}
	return c
}

// column finds a header's column: the name as a whole word, so NAME
// isn't found in NAMESPACE.
func column(head, name string) int {
	for at := 0; at < len(head); {
		i := strings.Index(head[at:], name)
		if i < 0 {
			return -1
		}
		i += at
		end := i + len(name)
		if (i == 0 || head[i-1] == ' ') && (end == len(head) || head[end] == ' ') {
			return i
		}
		at = end
	}
	return -1
}

// row reads a line of the table by the header's columns; ok is false
// for a line that isn't one (its TYPE isn't Normal or Warning).
func (c *eventCols) row(s string) (Entry, bool) {
	cell := func(i int) string {
		if c.starts[i] >= len(s) {
			return ""
		}
		end := len(s)
		if i+1 < len(c.starts) && c.starts[i+1] < end {
			end = c.starts[i+1]
		}
		return strings.TrimSpace(s[c.starts[i]:end])
	}
	e := Entry{Format: Text}
	for i, name := range c.names {
		v := cell(i)
		switch name {
		case "TYPE":
			switch v {
			case "Warning":
				e.Level = Warn
			case "Normal":
				e.Level = Info
			default:
				return Entry{}, false
			}
			e.RawLevel = v
		case "MESSAGE":
			e.Message = v
		case "OBJECT":
			e.Source = v
		}
		if v != "" && name != "MESSAGE" && name != "TYPE" {
			e.Fields = append(e.Fields, str(strings.ToLower(strings.ReplaceAll(name, " ", "_")), v))
		}
	}
	return e, true
}

// jsonShapes reads the JSON logs of tools whose fields say more than
// the common names do: go test -json, journald (journalctl -o json),
// Kubernetes Events and the API server's audit log.
func jsonShapes(e *Entry) {
	get := func(k string) string {
		for _, f := range e.Fields {
			if f.Key == k {
				return f.Text
			}
		}
		return ""
	}
	switch {
	case get("Action") != "" && get("Package") != "":
		goTestEvent(e)
	case get("MESSAGE") != "" && (get("__REALTIME_TIMESTAMP") != "" || get("_SYSTEMD_UNIT") != "" || get("SYSLOG_IDENTIFIER") != ""):
		e.Message = get("MESSAGE")
		if p, err := strconv.Atoi(get("PRIORITY")); err == nil && p >= 0 && p < 8 {
			e.Level, e.RawLevel = syslogLevels[p], get("PRIORITY")
		}
		if us, err := strconv.ParseInt(get("__REALTIME_TIMESTAMP"), 10, 64); err == nil {
			e.Time = time.UnixMicro(us).UTC()
		}
		e.Source = cmp.Or(get("_SYSTEMD_UNIT"), get("SYSLOG_IDENTIFIER"))
	case strings.HasPrefix(get("apiVersion"), "audit.k8s.io/") && get("verb") != "":
		code := get("responseStatus.code")
		e.Message = strings.TrimSpace(get("verb") + " " + get("requestURI") + " " + code)
		e.Level, e.RawLevel = statusLevel(code), code
		e.Source = get("user.username")
		if t, ok := parseTime(cmp.Or(get("requestReceivedTimestamp"), get("stageTimestamp"))); ok {
			e.Time = t
		}
		e.TraceID = get("auditID")
	case get("involvedObject.name") != "" && get("reason") != "":
		switch get("type") {
		case "Warning":
			e.Level = Warn
		case "Normal":
			e.Level = Info
		}
		e.RawLevel = get("type")
		e.Source = strings.ToLower(get("involvedObject.kind")) + "/" + get("involvedObject.name")
		if e.Time.IsZero() {
			e.Time, _ = parseTime(cmp.Or(get("lastTimestamp"), get("eventTime"), get("firstTimestamp"), get("metadata.creationTimestamp")))
		}
	}
}
