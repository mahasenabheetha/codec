package logview

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"
	"unsafe"
)

// File is one log given to Build: a file's bytes, or pasted text.
type File struct {
	Name string
	Data []byte
}

// Log is a read log held in memory for queries: entries in a compact
// form (about 70 bytes each plus 24 per field; texts share the decoded
// input's memory), with field keys and sources stored once.
type Log struct {
	Files   []FileInfo
	texts   []string // decoded inputs, kept alive for the rows' strings
	rows    []row
	cells   []cell
	extras  []extra
	keys    []string // field keys; a cell's key is an index here
	keyIDs  map[string]uint32
	strs    []string // sources; index 0 is ""
	strIDs  map[string]uint32
	groups  []groupKey // error groups by fingerprint; a row's group is index+1
	summary Summary

	msgOnce sync.Once // every entry's fingerprint, for comparing runs
	msg     *msgIndex
}

// FileInfo describes one input of a log.
type FileInfo struct {
	Name     string   `json:"name"`
	Bytes    int      `json:"bytes"`
	Lines    int      `json:"lines"`
	Entries  int      `json:"entries"`
	Encoding []string `json:"encoding,omitempty"` // what Decode undid ("gzip", "utf-16le")
}

type row struct {
	time    int64 // Unix nanoseconds; 0 when the entry has none
	line    uint32
	lines   uint32
	cellAt  uint32 // its first field in Log.cells
	ncells  uint16
	file    uint16
	source  uint32 // index into Log.strs
	group   uint32 // error group + 1, for warnings and worse
	extra   uint32 // index + 1 into Log.extras
	level   Level
	format  Format
	message string
	raw     string
}

// extra holds the core fields most entries lack.
type extra struct {
	rawLevel, logger, caller, err, stack, trace string
}

type cell struct {
	key  uint32
	kind Kind
	text string
}

// BuildOptions tune Build.
type BuildOptions struct {
	Options
	// Limit caps the decoded size of all files together; 0 means none.
	Limit int64
}

// Build reads and indexes one or more logs. Several files are merged by
// time; an entry without a time stays after the entry before it in its
// file. Entries without a source of their own take their file's name.
func Build(files []File, opt BuildOptions) (*Log, error) {
	l := &Log{keyIDs: map[string]uint32{}, strIDs: map[string]uint32{"": 0}, strs: []string{""}}
	g := newGrouper()
	var total int64
	for fi, f := range files {
		limit := int64(0)
		if opt.Limit > 0 {
			limit = opt.Limit - total
			if limit <= 0 {
				return nil, ErrTooLarge
			}
		}
		text, enc, err := Decode(f.Data, limit)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", f.Name, err)
		}
		if strings.IndexByte(text[:min(len(text), 8000)], 0) >= 0 {
			return nil, fmt.Errorf("%s: %w", f.Name, ErrNotText)
		}
		total += int64(len(text))
		if opt.Limit > 0 && total > opt.Limit {
			return nil, ErrTooLarge
		}
		l.texts = append(l.texts, text)
		info := FileInfo{Name: f.Name, Bytes: len(f.Data), Lines: countLines(text), Encoding: enc}
		for e := range Read(text, opt.Options) {
			if e.Source == "" && len(files) > 1 {
				e.Source = f.Name
			}
			l.add(&e, fi, g)
			info.Entries++
		}
		l.Files = append(l.Files, info)
	}
	if len(files) > 1 {
		l.merge()
	}
	l.groups = g.keys
	l.summarize()
	return l, nil
}

func countLines(s string) int {
	n := 0
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' {
			n++
		}
	}
	if len(s) > 0 && s[len(s)-1] != '\n' {
		n++
	}
	return n
}

func (l *Log) add(e *Entry, file int, g *grouper) {
	r := row{
		line: uint32(e.Line), lines: uint32(e.Lines), file: uint16(file),
		cellAt: uint32(len(l.cells)), ncells: uint16(min(len(e.Fields), 1<<16-1)),
		source: l.str(e.Source), level: e.Level, format: e.Format,
		message: e.Message, raw: e.Raw,
	}
	if !e.Time.IsZero() {
		r.time = e.Time.UnixNano()
	}
	for _, f := range e.Fields[:r.ncells] {
		l.cells = append(l.cells, cell{key: l.key(f.Key), kind: f.Kind, text: f.Text})
	}
	if x := (extra{e.RawLevel, e.Logger, e.Caller, e.Error, e.Stack, e.TraceID}); x != (extra{}) {
		l.extras = append(l.extras, x)
		r.extra = uint32(len(l.extras))
	}
	if r.level >= Warn {
		r.group = g.id(e.Message, e.Error)
	}
	l.rows = append(l.rows, r)
}

func (l *Log) key(k string) uint32 {
	if id, ok := l.keyIDs[k]; ok {
		return id
	}
	id := uint32(len(l.keys))
	k = string([]byte(k)) // a flattened key may be built per entry; keep one copy
	l.keys = append(l.keys, k)
	l.keyIDs[k] = id
	return id
}

func (l *Log) str(s string) uint32 {
	if id, ok := l.strIDs[s]; ok {
		return id
	}
	id := uint32(len(l.strs))
	l.strs = append(l.strs, s)
	l.strIDs[s] = id
	return id
}

// merge orders the rows of several files by time. Rows without a time
// sort with the time of the row before them in their file, so a stack
// printed after an entry stays next to it.
func (l *Log) merge() {
	eff := make([]int64, len(l.rows))
	var last int64
	for i, r := range l.rows {
		if i > 0 && r.file != l.rows[i-1].file {
			last = 0
		}
		if r.time != 0 {
			last = r.time
		}
		eff[i] = last
	}
	idx := make([]int, len(l.rows))
	for i := range idx {
		idx[i] = i
	}
	// Stable, so equal times keep file order, then input order.
	slices.SortStableFunc(idx, func(a, b int) int {
		switch {
		case eff[a] < eff[b]:
			return -1
		case eff[a] > eff[b]:
			return 1
		}
		return 0
	})
	rows := make([]row, len(l.rows))
	for i, j := range idx {
		rows[i] = l.rows[j]
	}
	l.rows = rows
}

// Size estimates the memory the log holds, in bytes: its decoded
// inputs and its index (about 2.4 times an ECS input in all).
func (l *Log) Size() int64 {
	n := int64(len(l.rows))*int64(unsafe.Sizeof(row{})) +
		int64(len(l.cells))*int64(unsafe.Sizeof(cell{})) +
		int64(len(l.extras))*int64(unsafe.Sizeof(extra{}))
	for _, t := range l.texts {
		n += int64(len(t))
	}
	return n
}

// Len is the number of entries.
func (l *Log) Len() int { return len(l.rows) }

// Entry returns entry i (in the log's order) in full. Its Line is in
// its own file, File(i) names which.
func (l *Log) Entry(i int) Entry {
	r := &l.rows[i]
	e := Entry{
		Line: int(r.line), Lines: int(r.lines), Format: r.format, Source: l.strs[r.source],
		Level: r.level, Message: r.message, Raw: r.raw,
	}
	if r.time != 0 {
		e.Time = time.Unix(0, r.time).UTC()
	}
	if r.extra != 0 {
		x := l.extras[r.extra-1]
		e.RawLevel, e.Logger, e.Caller, e.Error, e.Stack, e.TraceID = x.rawLevel, x.logger, x.caller, x.err, x.stack, x.trace
	}
	if r.ncells > 0 {
		e.Fields = make([]Field, r.ncells)
		for j, c := range l.cells[r.cellAt : r.cellAt+uint32(r.ncells)] {
			e.Fields[j] = Field{Key: l.keys[c.key], Kind: c.kind, Text: c.text}
			if c.kind == Number {
				e.Fields[j].Num, _ = strconv.ParseFloat(c.text, 64)
			}
		}
	}
	return e
}

// Time is entry i's time; zero when it has none.
func (l *Log) Time(i int) time.Time {
	if t := l.rows[i].time; t != 0 {
		return time.Unix(0, t).UTC()
	}
	return time.Time{}
}

// File is the index into Files of the file entry i came from.
func (l *Log) File(i int) int { return int(l.rows[i].file) }

// fields returns row i's cells.
func (l *Log) fields(i int) []cell {
	r := &l.rows[i]
	return l.cells[r.cellAt : r.cellAt+uint32(r.ncells)]
}
