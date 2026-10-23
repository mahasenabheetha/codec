// Package logview reads structured logs of any common format into one
// entry shape: ECS and other JSON (Go's slog, zap, logrus, zerolog;
// Bunyan, pino, Serilog), logfmt, Kubernetes klog and plain text with
// stack traces, behind whatever prefixes `argo logs`, `kubectl logs`,
// container runtimes, Compose or CI add. It is a pure engine package.
//
// Read never fails on content: a line it cannot place becomes a text
// entry, so every input line ends up in exactly one entry.
package logview

import (
	"strings"
	"time"
)

// Entry is one log record in the common shape.
type Entry struct {
	Line     int       // input line it starts on, 1-based
	Lines    int       // input lines it covers (a stack trace joins its entry)
	Format   Format    // how the payload was read
	Source   string    // from a prefix: pod, container or service; "" when none
	Time     time.Time // the payload's own time, else the prefix's; zero when none
	Level    Level
	RawLevel string // the level as written ("WARNING", "40", "W")
	Message  string
	Logger   string
	Caller   string // "file.go:42" or a function
	Error    string // the error's message, when the entry carries one
	Stack    string // a stack trace, from a field or from the lines that follow
	TraceID  string
	// Fields holds every field of a JSON, logfmt or klog payload in the
	// order written, nested objects flattened to dotted keys
	// ("http.request.method"). The core fields above are taken from
	// them, so a query can use either "level" or "log.level".
	Fields []Field
	// Raw is the input the entry was read from, colour codes removed.
	Raw string
}

// Field is one key and its value.
type Field struct {
	Key  string
	Kind Kind
	Text string  // the value as text; for arrays and objects, compact JSON
	Num  float64 // the value, for Kind Number
}

// Kind is a field value's JSON type.
type Kind uint8

const (
	String Kind = iota
	Number
	Bool
	Null
	Array
	Object // only for empty objects and ones nested too deep to flatten
)

// Format tells how an entry's payload was read.
type Format uint8

const (
	Text Format = iota
	JSON
	ECS // JSON with ECS field names
	Logfmt
	Klog
)

var formatNames = [...]string{"text", "json", "ecs", "logfmt", "klog"}

func (f Format) String() string { return formatNames[f] }

// Level is a normalised severity; the zero value means the entry has none.
type Level uint8

const (
	LevelNone Level = iota
	Trace
	Debug
	Info
	Warn
	Error
	Fatal // also panic, critical, alert, emergency
)

var levelNames = [...]string{"", "trace", "debug", "info", "warn", "error", "fatal"}

func (l Level) String() string { return levelNames[l] }

// ParseLevel reads a level as loggers write it: names in any case and
// common spellings ("WARNING", "err", "crit", "Information", "dpanic"),
// single letters (klog's I/W/E/F) and the numbers of Bunyan and pino
// (10 trace … 60 fatal). It returns LevelNone for anything else.
func ParseLevel(s string) Level {
	if l, ok := levelWords[strings.ToLower(strings.TrimSpace(s))]; ok {
		return l
	}
	return LevelNone
}

var levelWords = map[string]Level{
	"trace": Trace, "trc": Trace, "verbose": Trace, "vrb": Trace, "finest": Trace, "finer": Trace, "t": Trace, "10": Trace,
	"debug": Debug, "dbg": Debug, "fine": Debug, "config": Debug, "d": Debug, "20": Debug,
	"info": Info, "inf": Info, "information": Info, "informational": Info, "notice": Info, "i": Info, "30": Info,
	"warn": Warn, "warning": Warn, "wrn": Warn, "w": Warn, "40": Warn,
	"error": Error, "err": Error, "erro": Error, "eror": Error, "severe": Error, "e": Error, "50": Error,
	"fatal": Fatal, "ftl": Fatal, "fata": Fatal, "panic": Fatal, "dpanic": Fatal, "critical": Fatal, "crit": Fatal,
	"alert": Fatal, "emerg": Fatal, "emergency": Fatal, "f": Fatal, "60": Fatal,
}
