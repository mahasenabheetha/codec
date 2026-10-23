package logview

import (
	"strconv"
	"strings"
	"time"
)

// role is what a well-known field means for the common entry.
type role uint8

const (
	roleNone role = iota
	roleTime
	roleTimeMillis
	roleLevel
	roleMessage
	roleLogger
	roleCaller
	roleFile
	roleLine
	roleFunc
	roleError
	roleStack
	roleTrace
	roleCount
)

// Field names that carry each role, across ECS, slog, zap, logrus,
// zerolog, Bunyan, pino, Serilog, log4j2, Python and structlog. When an
// entry has several, the earlier name in its list wins.
var roleKeys = [roleCount][]string{
	roleTime:       {"@timestamp", "timestamp", "_timestamp", "time", "ts", "@t", "t", "Timestamp", "date", "datetime", "dateTime", "date_time", "eventTime", "asctime", "logtime", "log_time", "time_local", "created_at", "createdAt"},
	roleTimeMillis: {"timeMillis", "instant.epochSecond"},
	roleLevel:      {"log.level", "level", "severity", "lvl", "@l", "Level", "loglevel", "log_level", "levelname", "level_name"},
	roleMessage:    {"message", "msg", "@m", "RenderedMessage", "@mt", "MessageTemplate", "event", "log", "textPayload", "Message"},
	roleLogger:     {"log.logger", "logger", "logger_name", "loggerName", "SourceContext"},
	roleCaller:     {"caller"},
	roleFile:       {"log.origin.file.name", "source.file", "file", "filename"},
	roleLine:       {"log.origin.file.line", "source.line", "line", "lineno"},
	roleFunc:       {"log.origin.function", "source.function", "func", "funcName"},
	roleError:      {"error.message", "error", "err", "exception"},
	roleStack:      {"error.stack_trace", "stacktrace", "stack_trace", "stack", "error.stack", "@x", "Exception", "exc_info", "errorVerbose", "exception.stacktrace"},
	roleTrace:      {"trace.id", "trace_id", "traceId", "traceID", "TraceId", "dd.trace_id", "otelTraceID", "@tr"},
}

type roleRank struct {
	role role
	rank uint8
}

var keyRoles = func() map[string]roleRank {
	m := map[string]roleRank{}
	for r, keys := range roleKeys {
		for i, k := range keys {
			m[k] = roleRank{role(r), uint8(i)}
		}
	}
	return m
}()

// fromFields fills e's core fields from e.Fields, one map lookup per
// field (this runs for every line of a big log).
func fromFields(e *Entry) {
	var best [roleCount]int // index+1 into e.Fields
	var rank [roleCount]uint8
	for i, f := range e.Fields {
		rr, ok := keyRoles[f.Key]
		if !ok {
			continue
		}
		if b := best[rr.role]; b != 0 && rank[rr.role] <= rr.rank {
			continue
		}
		// Only text and numbers can be a level, message or time.
		if f.Kind != String && f.Kind != Number && rr.role != roleStack {
			continue
		}
		best[rr.role], rank[rr.role] = i+1, rr.rank
	}
	get := func(r role) (Field, bool) {
		if best[r] == 0 {
			return Field{}, false
		}
		return e.Fields[best[r]-1], true
	}
	if f, ok := get(roleTime); ok {
		if t, ok := parseTime(f.Text); ok && f.Kind == String {
			e.Time = t
		} else if isNumber(f.Text) {
			e.Time, _ = epochText(f.Text)
		}
	}
	if f, ok := get(roleTimeMillis); ok && e.Time.IsZero() && f.Kind == Number {
		e.Time, _ = epochText(f.Text)
		// log4j2's JSON layout: "instant": {"epochSecond": …, "nanoOfSecond": …}
		if n, ok := field(e.Fields, "instant.nanoOfSecond"); ok && f.Key == "instant.epochSecond" && !e.Time.IsZero() {
			if ns, err := strconv.ParseInt(n.Text, 10, 64); err == nil {
				e.Time = e.Time.Add(time.Duration(ns))
			}
		}
	}
	if e.Time.IsZero() {
		// Any other field named like a time ("_timestamp", "event_time")
		// whose value reads as one.
		for _, f := range e.Fields {
			if f.Kind == String && timeKey(f.Key) {
				if t, ok := parseTime(f.Text); ok {
					e.Time = t
					break
				}
			}
		}
	}
	if f, ok := get(roleLevel); ok {
		e.RawLevel = f.Text
		e.Level = ParseLevel(f.Text)
	} else if _, ok := get(roleMessage); ok && hasKey(e.Fields, "@t") {
		// Serilog's compact format leaves out the level when it is Information.
		e.Level = Info
	}
	if f, ok := get(roleMessage); ok {
		e.Message = f.Text
		if f.Key == "@mt" || f.Key == "MessageTemplate" {
			e.Message = renderTemplate(f.Text, e.Fields)
		}
	}
	if f, ok := get(roleLogger); ok {
		e.Logger = f.Text
	}
	if f, ok := get(roleCaller); ok {
		e.Caller = f.Text
	} else if f, ok := get(roleFile); ok {
		e.Caller = f.Text
		if l, ok := get(roleLine); ok && !strings.Contains(f.Text, ":") {
			e.Caller += ":" + l.Text
		}
	} else if f, ok := get(roleFunc); ok {
		e.Caller = f.Text
	}
	if f, ok := get(roleError); ok {
		e.Error = f.Text
	}
	if f, ok := get(roleStack); ok {
		e.Stack = f.Text
		if f.Kind == Array {
			e.Stack = stackFromArray(f.Text)
		}
	}
	if f, ok := get(roleTrace); ok {
		e.TraceID = f.Text
	}
	if hasKey(e.Fields, "ecs.version") || best[roleTime] != 0 && e.Fields[best[roleTime]-1].Key == "@timestamp" && best[roleLevel] != 0 && e.Fields[best[roleLevel]-1].Key == "log.level" {
		e.Format = ECS
	}
}

// renderTemplate fills the holes of a Serilog message template
// ("Order {OrderId} took {Elapsed:0.0} ms") with the entry's fields, as
// Serilog's own sinks show it; "{{" and "}}" are literal braces. A hole
// with no field of its name (a destructured "{@Order}") stays as written.
func renderTemplate(tpl string, fields []Field) string {
	if !strings.ContainsAny(tpl, "{}") {
		return tpl
	}
	var b strings.Builder
	for i := 0; i < len(tpl); i++ {
		c := tpl[i]
		if (c == '{' || c == '}') && i+1 < len(tpl) && tpl[i+1] == c {
			b.WriteByte(c)
			i++
			continue
		}
		if c == '{' {
			if end := strings.IndexByte(tpl[i:], '}'); end > 1 {
				name := strings.TrimLeft(tpl[i+1:i+end], "@$")
				if j := strings.IndexAny(name, ":,"); j >= 0 {
					name = name[:j]
				}
				if f, ok := field(fields, name); ok {
					b.WriteString(f.Text)
					i += end
					continue
				}
			}
		}
		b.WriteByte(c)
	}
	return b.String()
}

func field(fields []Field, key string) (Field, bool) {
	for _, f := range fields {
		if f.Key == key {
			return f, true
		}
	}
	return Field{}, false
}

// timeKey reports a field name whose last part is a time: "_timestamp",
// "event_time", "log.datetime".
func timeKey(k string) bool {
	k = strings.ToLower(k)
	last := k[strings.LastIndexAny(k, "._-@")+1:]
	return last == "time" || last == "datetime" || strings.HasSuffix(last, "timestamp")
}

func hasKey(fields []Field, key string) bool {
	for _, f := range fields {
		if f.Key == key {
			return true
		}
	}
	return false
}

// stackFromArray lays out a stack written as a JSON array (zerolog's
// pkgerrors marshaller): one frame per line.
func stackFromArray(s string) string {
	spans, ok := arrayItems(s)
	if !ok {
		return s
	}
	lines := make([]string, 0, len(spans))
	for _, sp := range spans {
		it := s[sp[0]:sp[1]]
		if strings.HasPrefix(it, "{") {
			if fs, ok := parseObject(it, nil); ok {
				parts := make([]string, 0, len(fs))
				for _, f := range fs {
					parts = append(parts, f.Text)
				}
				lines = append(lines, strings.Join(parts, " "))
				continue
			}
		}
		if strings.HasPrefix(it, `"`) {
			p := jscan{s: it}
			if v, ok := p.str(); ok {
				it = v
			}
		}
		lines = append(lines, it)
	}
	return strings.Join(lines, "\n")
}

// unwrapPayload handles JSON that wraps the real line: Docker's
// json-file driver ({"log": "...", "stream": ..., "time": ...}), whose
// log is read again as its own payload, and Elasticsearch hits, whose
// _source is the document. It reports the inner line for Docker.
func unwrapPayload(fields []Field) (inner string, wrapped bool, out []Field) {
	docker := len(fields) > 0
	for _, f := range fields {
		switch {
		case f.Key == "log" && f.Kind == String, f.Key == "stream", f.Key == "time", strings.HasPrefix(f.Key, "attrs."):
		default:
			docker = false
		}
	}
	if docker && hasKey(fields, "log") {
		out = fields[:0:0]
		for _, f := range fields {
			if f.Key == "log" {
				inner = strings.TrimRight(f.Text, "\r\n")
				continue
			}
			out = append(out, f)
		}
		return inner, true, out
	}
	if !hasPrefixKey(fields, "_source.") {
		return "", false, fields
	}
	out = make([]Field, 0, len(fields))
	for _, f := range fields {
		switch {
		case strings.HasPrefix(f.Key, "_source."):
			f.Key = f.Key[len("_source."):]
			out = append(out, f)
		case f.Key == "_index", f.Key == "_id":
			out = append(out, f)
		}
		// _score, sort, fields.* (Kibana's copies as arrays) and
		// highlight.* repeat the document; they are left out.
	}
	return "", false, out
}

func hasPrefixKey(fields []Field, prefix string) bool {
	for _, f := range fields {
		if strings.HasPrefix(f.Key, prefix) {
			return true
		}
	}
	return false
}
