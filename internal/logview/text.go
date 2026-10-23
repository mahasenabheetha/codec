package logview

import (
	"regexp"
	"strconv"
	"strings"
	"time"
)

var (
	// klog: "I1022 10:00:00.123456    1 controller.go:42] message".
	klogRe = regexp.MustCompile(`^([IWEF])(\d\d)(\d\d) (\d\d:\d\d:\d\d\.\d{6})\s+\d+ ([^\s\]]+)\] ?`)
	// Go's log package: "2026/10/22 10:00:00.123456 text", perhaps
	// followed by "file.go:42: ".
	goLogRe  = regexp.MustCompile(`^(\d{4}/\d\d/\d\d \d\d:\d\d:\d\d(?:\.\d+)?) `)
	goFileRe = regexp.MustCompile(`^([\w./-]+\.go:\d+): `)
	// Text with a leading time: logback, log4j, Python, most others.
	textTimeRe = regexp.MustCompile(`^\[?(\d{4}-\d\d-\d\d[T ]\d\d:\d\d:\d\d(?:[.,]\d+)?(?: ?(?:Z|[+-]\d\d:?\d\d))?)(?: UTC)?\]?:?\s+`)
	// A line that is only a date and time (Oracle's alert log).
	timeOnlyRe = regexp.MustCompile(`^\[?\d{4}-\d\d-\d\d[T ]\d\d:\d\d:\d\d(?:[.,]\d+)?(?: ?(?:Z|[+-]\d\d:?\d\d))?\]?$`)
	// A time of day without a date: "10:00:00.123 [main] INFO …".
	clockRe = regexp.MustCompile(`^\[?(\d\d:\d\d:\d\d(?:[.,]\d{1,9})?)\]?\s+`)
	// syslog: "Oct 22 10:00:00 ", without a year.
	syslogRe = regexp.MustCompile(`^([A-Z][a-z]{2} [ \d]\d \d\d:\d\d:\d\d) `)

	goroutineRe = regexp.MustCompile(`^goroutine \d+ \[[^\]]*\]:$`)
	// "java.lang.IllegalStateException: boom", `Exception in thread "main" …`,
	// "System.InvalidOperationException", Python's last traceback line
	// "ValueError: bad value".
	exceptionRe = regexp.MustCompile(`^(?:Exception in thread "[^"]*" )?(?:[a-zA-Z_$][\w$]*\.)*[A-Z][\w$]*(?:Exception|Error|Throwable|Fault)(?::|$)`)
)

// klog reads a klog line, including klog's structured form
// (`"message" key="value"`), into e.
func readKlog(s string, year int, e *Entry) bool {
	if len(s) < 22 || !strings.ContainsRune("IWEF", rune(s[0])) {
		return false
	}
	m := klogRe.FindStringSubmatchIndex(s)
	if m == nil {
		return false
	}
	month, _ := strconv.Atoi(s[m[4]:m[5]])
	day, _ := strconv.Atoi(s[m[6]:m[7]])
	clock, err := time.Parse("15:04:05.000000", s[m[8]:m[9]])
	if err != nil || month < 1 || month > 12 || day < 1 || day > 31 {
		return false
	}
	e.Format = Klog
	e.RawLevel = s[m[2]:m[3]]
	e.Level = ParseLevel(e.RawLevel)
	e.Time = time.Date(year, time.Month(month), day, clock.Hour(), clock.Minute(), clock.Second(), clock.Nanosecond(), time.UTC)
	e.Caller = s[m[10]:m[11]]
	rest := s[m[1]:]
	e.Message = rest
	if strings.HasPrefix(rest, `"`) {
		if msg, n, ok := unquoteLogfmt(rest); ok {
			if fs, _, words, ok := parseLogfmt(rest[n:], nil); ok && words == 0 {
				e.Message, e.Fields = msg, fs
				for _, f := range fs {
					if f.Key == "err" && e.Error == "" {
						e.Error = f.Text
					}
				}
			}
		}
	}
	return true
}

// readText reads a plain text line: a leading time if there is one,
// then a level word near the start. It reports whether a time was found.
func readText(s string, day time.Time, e *Entry) bool {
	e.Format = Text
	rest, found := s, false
	if m := textTimeRe.FindStringSubmatchIndex(s); m != nil {
		if t, ok := parseTime(strings.Replace(s[m[2]:m[3]], " +", "+", 1)); ok {
			e.Time, rest, found = t, s[m[1]:], true
		} else if t, ok := parseTime(strings.Replace(s[m[2]:m[3]], " -", "-", 1)); ok {
			e.Time, rest, found = t, s[m[1]:], true
		}
	} else if m := clockRe.FindStringSubmatchIndex(s); m != nil {
		// A time of day only (logback's default HH:mm:ss.SSS): on the
		// log's day, or year 1 while the reader is still learning.
		if c, err := time.Parse("15:04:05.999999999", strings.Replace(s[m[2]:m[3]], ",", ".", 1)); err == nil {
			y, mo, d := day.Date()
			if day.IsZero() {
				y, mo, d = 1, 1, 1
			}
			e.Time = time.Date(y, mo, d, c.Hour(), c.Minute(), c.Second(), c.Nanosecond(), time.UTC)
			rest, found = s[m[1]:], true
		}
	} else if m := goLogRe.FindStringSubmatchIndex(s); m != nil {
		if t, ok := parseTime(s[m[2]:m[3]]); ok {
			e.Time, rest, found = t, s[m[1]:], true
			if f := goFileRe.FindStringSubmatchIndex(rest); f != nil {
				e.Caller, rest = rest[f[2]:f[3]], rest[f[1]:]
			}
		}
	} else if m := syslogRe.FindStringSubmatchIndex(s); m != nil {
		if t, err := time.Parse("Jan _2 15:04:05", s[m[2]:m[3]]); err == nil {
			e.Time, rest, found = withYear(t, max(day.Year(), 1)), s[m[1]:], true
		}
	}
	e.Level, e.RawLevel, rest = textLevel(rest)
	e.Message = rest
	return found
}

// textLevel finds a level word among the first few words: in capitals
// anywhere there ("[main] INFO  c.e.App - started"), or as the first
// word followed by a colon or in brackets in any case ("error: x",
// "[warn] y", Python's "ERROR:root:z"). A leading level word is taken
// off the message, as the list shows the level on its own.
func textLevel(s string) (Level, string, string) {
	pos := 0
	for w := 0; w < 4; w++ {
		for pos < len(s) && s[pos] == ' ' {
			pos++
		}
		if pos >= len(s) {
			break
		}
		end := len(s)
		if i := strings.IndexByte(s[pos:], ' '); i >= 0 {
			end = pos + i
		}
		tok := s[pos:end]
		word, marked := tok, false
		if i := strings.IndexByte(word, ':'); i > 0 {
			word, marked = word[:i], true
		}
		if t := strings.Trim(word, "[]()<>|"); t != word {
			word, marked = t, true
		}
		if isLevelWord(word, w == 0, marked) {
			if w > 0 {
				return ParseLevel(word), word, s
			}
			// "ERROR:root:msg" → "msg"; "[INFO] x" and "ERROR: x" → "x".
			msg := end
			if i := strings.LastIndexByte(tok, ':'); i >= 0 && i+1 < len(tok) {
				msg = pos + i + 1
			}
			return ParseLevel(word), word, strings.TrimLeft(s[msg:], " -:|")
		}
		pos = end
	}
	return LevelNone, "", s
}

// isLevelWord: capitals count anywhere; the first word may also be
// "Error" or marked by brackets or a colon. Numbers and single letters
// ("40 items", "I think") never count in text.
func isLevelWord(word string, first, marked bool) bool {
	if len(word) < 3 || word[0] < 'A' || ParseLevel(word) == LevelNone {
		return false
	}
	return word == strings.ToUpper(word) || first && (marked || word == title(word))
}

func title(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + strings.ToLower(s[1:])
}

// Stack traces.

func goPanicStart(s string) bool {
	return strings.HasPrefix(s, "panic: ") || strings.HasPrefix(s, "fatal error: ")
}

// goStackLine is a line inside a Go panic or goroutine dump.
func goStackLine(s string) bool {
	switch {
	case s == "":
		return false
	case s[0] == '\t', goroutineRe.MatchString(s):
		return true
	}
	for _, p := range [...]string{"created by ", "[signal ", "exit status ", "runtime stack:", "panic: ", "fatal error: ", "...additional frames elided..."} {
		if strings.HasPrefix(s, p) {
			return true
		}
	}
	// A frame's function: "main.(*Server).handle(0xc000123456, {0x0, 0x0})".
	open := strings.IndexByte(s, '(')
	return open > 0 && s[len(s)-1] == ')' && !strings.ContainsAny(s[:open], " \t")
}

// javaFrame is a frame line of a Java, .NET, Node or Kotlin stack trace.
func javaFrame(s string) bool {
	t := strings.TrimLeft(s, " \t")
	if t == s {
		return strings.HasPrefix(s, "Caused by: ")
	}
	if strings.HasPrefix(t, "at ") || strings.HasPrefix(t, "Caused by: ") || strings.HasPrefix(t, "Suppressed: ") {
		return true
	}
	return strings.HasPrefix(t, "... ") && (strings.HasSuffix(t, " more") || strings.HasSuffix(t, " omitted"))
}

const (
	pyTraceback = "Traceback (most recent call last):"
	pyDuring    = "During handling of the above exception, another exception occurred:"
	pyCause     = "The above exception was the direct cause of the following exception:"
)
