package logview

import (
	"iter"
	"strings"
	"time"
)

// Options tune Read; the zero value is fine.
type Options struct {
	// Now gives the year for times written without one (klog, syslog)
	// when the log has no full time to take it from; time.Now when nil.
	Now func() time.Time
}

// ReadAll reads a whole log into a slice; see Read.
func ReadAll(text string, opt Options) []Entry {
	var out []Entry
	for e := range Read(text, opt) {
		out = append(out, e)
	}
	return out
}

// Read reads a decoded log (see Decode) and yields its entries in the
// order they start. Every non-blank line ends up in exactly one entry:
// stack traces and continuation lines join the entry before them from
// the same source, the rest become entries of their own.
//
// Read returns an iterator (a range-over-func sequence), so a caller
// can index a big log without holding every Entry at once.
func Read(text string, opt Options) iter.Seq[Entry] {
	return func(yield func(Entry) bool) {
		if items, ok := documentItems(text); ok {
			for _, it := range items {
				if !yield(it) {
					return
				}
			}
			return
		}
		r := newReader(text, opt)
		r.run(yield)
	}
}

// An entry stays open for continuation lines until its source starts
// another entry, or this many lines pass.
const openLines = 1000

type blockKind uint8

const (
	noBlock   blockKind = iota
	goBlock             // a Go panic or goroutine dump
	javaBlock           // frames after an exception line
	pyBlock             // a Python traceback, before its last line
	pyEnded             // after a traceback's last line: only a chained one may follow
	headBlock           // after a line that is only a time: the plain lines that follow are its message
)

type pending struct {
	e        Entry
	lastLine int
	block    blockKind
	blanks   []string // raw blank lines seen inside a block, kept only if it goes on
	stream   string
	raws     []string // raw lines, when they are not next to each other
	start    int      // offsets of the raw text, while the lines are contiguous
	end      int
	split    bool // raws holds the raw text
	msg      []string
	stack    []string
	done     bool
	alone    bool // a stack with no entry before it: its last line names it
}

// line is one input line, ready to read.
type line struct {
	n          int // 1-based
	start, end int // raw offsets in the text, without the line ending
	raw        string
	s          string // the payload: prefixes, colour codes and redraws removed
	pre        prefix
	parts      int // physical lines, more than one when a runtime split it
}

type reader struct {
	text  string
	day   time.Time // the date for times written without one
	year  int
	shape labelShape
	// cont: text lines without a time continue the entry before them,
	// because the log's text lines usually start with one.
	cont  bool
	pos   int // next unread offset
	n     int // lines read
	peek  *line
	queue []*pending
	head  int
	last  map[string]*pending
}

func newReader(text string, opt Options) *reader {
	r := &reader{text: text, last: map[string]*pending{}}
	r.learn(opt)
	return r
}

// learn looks at a sample of the log: which label prefix its lines
// carry, its year, and whether its text lines start with a time.
func (r *reader) learn(opt Options) {
	var sample []string
	rest := r.text
	for len(sample) < 2000 && rest != "" {
		i := strings.IndexByte(rest, '\n')
		if i < 0 {
			i = len(rest)
		}
		sample = append(sample, redraw(stripANSI(rest[:i])))
		rest = rest[min(i+1, len(rest)):]
	}
	r.shape = learnLabel(sample)
	// The day of the log's first full time, for times written without a
	// date (klog, syslog, logback's HH:mm:ss.SSS). While learning, those
	// read with year 1 and don't count.
	setDay := func(t time.Time) {
		if r.day.IsZero() && t.Year() > 1 {
			t = t.UTC()
			r.day = time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
		}
	}
	withTime, without := 0, 0
	for _, s := range sample {
		var p prefix
		s = r.prep(s, &p)
		setDay(p.time)
		if strings.TrimSpace(s) == "" || isStackish(s) {
			continue
		}
		var e Entry
		if r.structured(s, &e) {
			setDay(e.Time)
			continue
		}
		if readText(s, time.Time{}, &e) {
			withTime++
			setDay(e.Time)
		} else {
			without++
		}
	}
	r.cont = withTime >= 2 && withTime >= without
	if r.day.IsZero() {
		now := time.Now
		if opt.Now != nil {
			now = opt.Now
		}
		t := now().UTC()
		r.day = time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
	}
	r.year = r.day.Year()
}

func isStackish(s string) bool {
	return javaFrame(s) || goStackLine(s) || s == pyTraceback || s[0] == ' ' || s[0] == '\t'
}

// next reads the next line, joining the pieces a container runtime split.
func (r *reader) next() *line {
	if r.peek != nil {
		l := r.peek
		r.peek = nil
		return l
	}
	var l *line
	for r.pos < len(r.text) {
		start := r.pos
		i := strings.IndexByte(r.text[start:], '\n')
		end := len(r.text)
		if i >= 0 {
			end = start + i
		}
		r.pos = end + 1
		r.n++
		raw := strings.TrimRight(r.text[start:end], "\r")
		var p prefix
		s := r.prep(redraw(stripANSI(raw)), &p)
		if l != nil {
			// The rest of a line the runtime split.
			l.s += s
			l.end, l.raw = end, r.text[l.start:end]
			l.parts++
			l.pre.partial = p.partial
		} else {
			l = &line{n: r.n, start: start, end: start + len(raw), raw: raw, s: s, pre: p, parts: 1}
		}
		if !l.pre.partial {
			return l
		}
	}
	return l
}

// prep takes the prefixes off a cleaned line: a time, a label, a time
// after the label (`argo logs --timestamps`), and Docker's JSON wrapper.
func (r *reader) prep(s string, p *prefix) string {
	s = timePrefix(s, p)
	s, labelled := label(s, r.shape, p)
	s = timePrefix(s, p)
	if !labelled && s != "" && s[0] != '{' {
		s, _ = jsonAfterLabel(s, p)
	}
	if strings.HasPrefix(s, `{"log":`) {
		s = dockerLine(s, p)
	}
	return s
}

// dockerLine unwraps Docker's json-file format: the line is in "log".
func dockerLine(s string, p *prefix) string {
	fields, ok := parseObject(s, nil)
	if !ok {
		return s
	}
	inner, wrapped, rest := unwrapPayload(fields)
	if !wrapped {
		return s
	}
	for _, f := range rest {
		switch f.Key {
		case "time":
			if t, ok := parseTime(f.Text); ok {
				p.time = t
			}
		case "stream":
			p.stream = f.Text
		default:
			p.fields = append(p.fields, f)
		}
	}
	return inner
}

func (r *reader) peekLine() *line {
	if r.peek == nil {
		r.peek = r.next()
	}
	return r.peek
}

func (r *reader) run(yield func(Entry) bool) {
	for {
		l := r.next()
		if l == nil {
			break
		}
		r.handle(l)
		if !r.flush(l.n, yield) {
			return
		}
	}
	for _, p := range r.queue[r.head:] {
		p.done = true
	}
	r.flush(r.n+openLines+1, yield)
}

// handle places one line: into an open entry, or as a new one.
func (r *reader) handle(l *line) {
	src := l.pre.source
	p := r.last[src]
	if p != nil && l.n-p.lastLine > openLines {
		p.done, p = true, nil
		delete(r.last, src)
	}
	if strings.TrimSpace(l.s) == "" {
		if p != nil && p.block != noBlock {
			p.blanks = append(p.blanks, l.raw)
		}
		return
	}
	if p != nil && p.block != noBlock {
		if r.inBlock(p, l.s) {
			if p.block == headBlock {
				p.msg = append(p.msg, l.s)
				r.join(p, l, false)
			} else {
				r.join(p, l, true)
			}
			return
		}
		p.block, p.blanks = noBlock, nil
	}
	var e Entry
	if r.structured(l.s, &e) {
		r.start(e, l, noBlock)
		return
	}
	// A text logger prints a stack after its line; a JSON one puts it in
	// a field, so lines after a JSON entry (or from the other stream) are
	// not its stack.
	takes := p != nil && p.e.Format == Text && p.stream == l.pre.stream
	switch {
	case timeOnlyRe.MatchString(l.s):
		// Oracle's alert log and others write the time on a line of its
		// own; the lines after it are what happened then.
		e := Entry{Format: Text}
		e.Time, _ = parseTime(strings.Replace(strings.Trim(l.s, "[]"), " +", "+", 1))
		r.start(e, l, headBlock)
		return
	case takes && javaFrame(l.s):
		r.join(p, l, true)
		if p.e.Error == "" && exceptionRe.MatchString(p.e.Message) {
			// The exception line came first, as an entry of its own.
			p.e.Error = p.e.Message
		}
		p.block = javaBlock
		return
	case takes && exceptionRe.MatchString(l.s) && r.frameFollows(src):
		r.join(p, l, true)
		if p.e.Error == "" {
			p.e.Error = l.s
		}
		p.block = javaBlock
		return
	case l.s == pyTraceback:
		if takes {
			r.join(p, l, true)
			p.block = pyBlock
			return
		}
		r.start(Entry{Format: Text, Level: Error}, l, pyBlock).alone = true
		return
	case goPanicStart(l.s):
		e := Entry{Format: Text, Level: Fatal, Message: l.s}
		e.Error, _ = strings.CutPrefix(l.s, "panic: ")
		r.start(e, l, goBlock)
		return
	case goroutineRe.MatchString(l.s):
		r.start(Entry{Format: Text, Message: l.s}, l, goBlock)
		return
	}
	hasTime := readText(l.s, r.day, &e)
	if r.cont && !hasTime && e.Level == LevelNone && p != nil && p.e.Format == Text {
		p.msg = append(p.msg, l.s)
		r.join(p, l, false)
		return
	}
	r.start(e, l, noBlock)
}

// frameFollows reports whether the next line from src is a stack frame,
// which makes the current line the exception that heads it.
func (r *reader) frameFollows(src string) bool {
	nl := r.peekLine()
	return nl != nil && nl.pre.source == src && javaFrame(nl.s)
}

// inBlock reports whether s goes on with p's stack trace.
func (r *reader) inBlock(p *pending, s string) bool {
	switch p.block {
	case goBlock:
		return goStackLine(s)
	case javaBlock:
		return javaFrame(s)
	case pyBlock:
		if s[0] != ' ' && s[0] != '\t' {
			// The traceback's last line: the exception.
			p.block = pyEnded
			if p.e.Error == "" || p.alone {
				p.e.Error = s
			}
		}
		return true
	case pyEnded:
		if s == pyDuring || s == pyCause || s == pyTraceback {
			p.block = pyBlock
			return true
		}
	case headBlock:
		// Plain lines only: one that is structured, has a time or a level
		// of its own, or starts a stack trace begins its own entry.
		if timeOnlyRe.MatchString(s) || goPanicStart(s) || s == pyTraceback {
			return false
		}
		var e Entry
		if r.structured(s, &e) || readText(s, r.day, &e) || e.Level != LevelNone {
			return false
		}
		return true
	}
	return false
}

// structured reads JSON, klog or logfmt payloads into e.
func (r *reader) structured(s string, e *Entry) bool {
	switch {
	case s[0] == '{':
		fields, ok := parseObject(s, make([]Field, 0, 16))
		if !ok {
			return false
		}
		_, _, fields = unwrapPayload(fields)
		e.Format, e.Fields = JSON, fields
		fromFields(e)
		e.Fields = messageFields(e)
		return true
	case readKlog(s, r.year, e):
		return true
	case strings.IndexByte(s, '=') > 0:
		fields, pairs, words, ok := parseLogfmt(s, make([]Field, 0, 8))
		if !ok || pairs < 2 || words > pairs/2 {
			return false
		}
		known := false
		for _, f := range fields {
			if rr, ok := keyRoles[f.Key]; ok && (rr.role == roleMessage || rr.role == roleLevel || rr.role == roleTime) {
				known = true
				break
			}
		}
		if !known && pairs < 3 {
			return false
		}
		*e = Entry{Format: Logfmt, Fields: fields}
		fromFields(e)
		if e.Message == "" && words > 0 {
			// "Processing user=bob id=7": the words are only in the line.
			e.Message = s
		}
		return true
	}
	return false
}

// messageFields reads a message that is itself a JSON object as fields
// under the message's key ("msg.user.id").
func messageFields(e *Entry) []Field {
	if !strings.HasPrefix(e.Message, "{") {
		return e.Fields
	}
	for _, f := range e.Fields {
		if rr := keyRoles[f.Key]; rr.role != roleMessage || f.Text != e.Message {
			continue
		}
		inner, ok := parseObject(e.Message, nil)
		if !ok {
			return e.Fields
		}
		for _, g := range inner {
			g.Key = f.Key + "." + g.Key
			e.Fields = append(e.Fields, g)
		}
		break
	}
	return e.Fields
}

// start opens a new entry for l.
func (r *reader) start(e Entry, l *line, block blockKind) *pending {
	src := l.pre.source
	if prev := r.last[src]; prev != nil {
		prev.done = true
	}
	e.Line, e.Source = l.n, src
	if e.Time.IsZero() {
		e.Time = l.pre.time
	}
	if l.pre.stream != "" && !hasKey(e.Fields, "stream") {
		e.Fields = append(e.Fields, Field{Key: "stream", Text: l.pre.stream})
	}
	e.Fields = append(e.Fields, l.pre.fields...)
	p := &pending{e: e, lastLine: l.n + l.parts - 1, block: block, start: l.start, end: l.end, stream: l.pre.stream}
	p.e.Lines = l.parts
	if block == pyBlock {
		// A Go panic's first line is its message; a traceback's is part of the stack.
		p.stack = []string{l.s}
	}
	r.queue = append(r.queue, p)
	r.last[src] = p
	return p
}

// join adds l to p, to its stack or its message.
func (r *reader) join(p *pending, l *line, toStack bool) {
	blanks := p.blanks
	p.blanks = nil
	if toStack {
		for range blanks {
			p.stack = append(p.stack, "")
		}
		p.stack = append(p.stack, l.s)
	}
	if !p.split && l.n == p.lastLine+1+len(blanks) {
		p.end = l.end
	} else {
		if !p.split {
			p.raws = []string{r.text[p.start:p.end]}
			p.split = true
		}
		p.raws = append(append(p.raws, blanks...), l.raw)
	}
	p.lastLine = l.n + l.parts - 1
	p.e.Lines += len(blanks) + l.parts
}

// flush yields the entries at the front of the queue that are complete.
func (r *reader) flush(n int, yield func(Entry) bool) bool {
	for r.head < len(r.queue) {
		p := r.queue[r.head]
		if !p.done && n-p.lastLine <= openLines {
			break
		}
		if r.last[p.e.Source] == p {
			delete(r.last, p.e.Source)
		}
		r.queue[r.head] = nil
		r.head++
		if !yield(r.finish(p)) {
			return false
		}
	}
	if r.head > 1024 && r.head*2 > len(r.queue) {
		r.queue = append(r.queue[:0], r.queue[r.head:]...)
		r.head = 0
	}
	return true
}

// finish puts together an entry's text.
func (r *reader) finish(p *pending) Entry {
	e := p.e
	if p.split {
		e.Raw = strings.Join(p.raws, "\n")
	} else {
		e.Raw = r.text[p.start:p.end]
	}
	if strings.ContainsAny(e.Raw, "\x1b\r") {
		lines := strings.Split(e.Raw, "\n")
		for i, s := range lines {
			lines[i] = redraw(stripANSI(s))
		}
		e.Raw = strings.Join(lines, "\n")
	}
	if len(p.msg) > 0 {
		if e.Message != "" {
			p.msg = append([]string{e.Message}, p.msg...)
		}
		e.Message = strings.Join(p.msg, "\n")
	}
	for len(p.stack) > 0 && p.stack[0] == "" {
		// The blank line between a Go panic's message and its goroutines.
		p.stack = p.stack[1:]
	}
	if len(p.stack) > 0 {
		st := strings.Join(p.stack, "\n")
		if e.Stack != "" {
			st = e.Stack + "\n" + st
		}
		e.Stack = st
	}
	if p.alone && e.Message == "" {
		// A traceback on its own is named by the exception it ends with.
		e.Message = e.Error
		if e.Message == "" {
			e.Message = pyTraceback
		}
	}
	return e
}

// documentItems reads a log that is one JSON document rather than lines:
// an array of entries, or an Elasticsearch response (hits.hits), as
// Kibana and the search API export them.
func documentItems(text string) ([]Entry, bool) {
	t := strings.TrimLeft(text, " \t\r\n")
	if t == "" || t[0] != '[' && t[0] != '{' {
		return nil, false
	}
	off := len(text) - len(t)
	t = strings.TrimRight(t, " \t\r\n")
	var spans [][2]int
	if t[0] == '{' {
		// One object per line (NDJSON) is the common case: this fails at
		// the end of the first line.
		if _, ok := parseObject(t, nil); !ok {
			return nil, false
		}
		if a, b, ok := memberSpan(t, "hits"); ok {
			if c, d, ok := memberSpan(t[a:b], "hits"); ok && t[a+c] == '[' {
				off += a + c
				t = t[a+c : a+d]
			}
		}
		if t[0] == '{' {
			if !strings.Contains(t, "\n") {
				return nil, false // a single line reads as any other line
			}
			// One pretty-printed entry.
			spans = [][2]int{{0, len(t)}}
		}
	}
	if spans == nil {
		var ok bool
		if spans, ok = arrayItems(t); !ok {
			return nil, false
		}
	}
	out := make([]Entry, 0, len(spans))
	line, at := 1, 0
	for _, sp := range spans {
		item := t[sp[0]:sp[1]]
		line += strings.Count(text[at:off+sp[0]], "\n")
		at = off + sp[0]
		e := Entry{Line: line, Lines: strings.Count(item, "\n") + 1, Raw: item}
		var r reader
		if item == "" || item[0] != '{' || !r.structured(item, &e) {
			return nil, false
		}
		out = append(out, e)
	}
	return out, true
}

// memberSpan finds the value of key in the JSON object s.
func memberSpan(s, key string) (int, int, bool) {
	p := jscan{s: s}
	p.ws()
	if p.i >= len(s) || s[p.i] != '{' {
		return 0, 0, false
	}
	p.i++
	for {
		p.ws()
		k, ok := p.str()
		if !ok {
			return 0, 0, false
		}
		p.ws()
		if p.i >= len(s) || s[p.i] != ':' {
			return 0, 0, false
		}
		p.i++
		p.ws()
		start := p.i
		if !p.skip() {
			return 0, 0, false
		}
		if k == key {
			return start, p.i, true
		}
		p.ws()
		if p.i >= len(s) || s[p.i] != ',' {
			return 0, 0, false
		}
		p.i++
	}
}
