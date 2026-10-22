package web

import (
	"bytes"
	"crypto/rand"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"log"
	"mime"
	"net/http"
	"path"
	"runtime/debug"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/mahasenabheetha/codec/v2/internal/logview"
	"github.com/mahasenabheetha/codec/v2/internal/workspace"
)

// The Log viewer keeps a read log in memory under an id, so the browser
// asks only for what it shows (decision 86). Nothing is written to
// disk; a log is dropped after an idle hour, when the logs held pass
// maxLogsHeld (oldest use first), on DELETE or when codec stops. An
// unknown id answers 410 Gone, and the UI sends the log again.
const (
	maxLogText  = 256 << 20 // decoded text per log; held memory is about 2.4 times that
	maxLogsHeld = 1 << 30   // memory held by all logs together
	logIdle     = time.Hour
	maxPage     = 1000
)

// logBuilds lets two logs be read at once; a third waits.
var logBuilds = make(chan struct{}, 2)

type heldLog struct {
	id   string
	log  *logview.Log
	size int64
	used time.Time
	idle *time.Timer

	mu  sync.Mutex // guards the last query, which paging reuses
	key string
	m   *logview.Matches
}

type logStore struct {
	mu    sync.Mutex
	logs  map[string]*heldLog
	total int64
	idle  time.Duration
}

func newLogStore() *logStore { return &logStore{logs: map[string]*heldLog{}, idle: logIdle} }

// add holds l, dropping the least recently used logs to make room.
func (st *logStore) add(l *logview.Log) (*heldLog, error) {
	size := l.Size()
	if size > maxLogsHeld {
		return nil, errors.New("this log needs more memory than codec keeps for logs; open part of it")
	}
	b := make([]byte, 12)
	rand.Read(b)
	h := &heldLog{id: hex.EncodeToString(b), log: l, size: size, used: time.Now()}
	st.mu.Lock()
	defer st.mu.Unlock()
	for st.total+size > maxLogsHeld {
		var oldest *heldLog
		for _, o := range st.logs {
			if oldest == nil || o.used.Before(oldest.used) {
				oldest = o
			}
		}
		st.dropLocked(oldest)
	}
	h.idle = time.AfterFunc(st.idle, func() { st.drop(h.id) })
	st.logs[h.id] = h
	st.total += size
	return h, nil
}

// get returns a held log and marks it used.
func (st *logStore) get(id string) *heldLog {
	st.mu.Lock()
	defer st.mu.Unlock()
	h := st.logs[id]
	if h != nil {
		h.used = time.Now()
		h.idle.Reset(st.idle)
	}
	return h
}

func (st *logStore) drop(id string) bool {
	st.mu.Lock()
	defer st.mu.Unlock()
	h := st.logs[id]
	if h != nil {
		st.dropLocked(h)
	}
	return h != nil
}

func (st *logStore) dropLocked(h *heldLog) {
	h.idle.Stop()
	delete(st.logs, h.id)
	st.total -= h.size
}

func (st *logStore) clear() {
	st.mu.Lock()
	defer st.mu.Unlock()
	for _, h := range st.logs {
		st.dropLocked(h)
	}
}

// matches runs a query, or reuses the last one while the user pages.
func (h *heldLog) matches(q logQuery) (*logview.Matches, error) {
	key := q.Q + "\x00" + q.From + "\x00" + q.To
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.m != nil && h.key == key {
		return h.m, nil
	}
	from, to, err := q.times()
	if err != nil {
		return nil, err
	}
	m, err := h.log.Match(q.Q, from, to)
	if err != nil {
		return nil, err
	}
	h.key, h.m = key, m
	return m, nil
}

// logQuery is the filter every read of a log takes.
type logQuery struct {
	Q    string `json:"q"`
	From string `json:"from"` // RFC 3339; "" for no bound
	To   string `json:"to"`
}

func (q logQuery) times() (from, to time.Time, err error) {
	for _, p := range []struct {
		s string
		t *time.Time
	}{{q.From, &from}, {q.To, &to}} {
		if p.s == "" {
			continue
		}
		if *p.t, err = time.Parse(time.RFC3339Nano, p.s); err != nil {
			return from, to, &logview.QueryError{Msg: "time range: " + p.s + " is not an RFC 3339 time"}
		}
	}
	return from, to, nil
}

type logOpened struct {
	ID      string             `json:"id"`
	Files   []logview.FileInfo `json:"files"`
	Summary logview.Summary    `json:"summary"`
	Size    int64              `json:"size"`
}

// POST /api/v2/logs: read logs and hold them. The body is either
// multipart form data, each part a file (its filename) or pasted text
// (a part without one), or JSON {"paths": [...]} naming log files in
// the open folder. Several logs are merged by time into one.
func (s *Server) handleLogsOpen(w http.ResponseWriter, r *http.Request) {
	select {
	case logBuilds <- struct{}{}:
	case <-r.Context().Done():
		return
	}
	defer func() { <-logBuilds }()
	defer func() {
		if p := recover(); p != nil {
			log.Printf("panic: %v\n%s", p, debug.Stack())
			writeError(w, http.StatusInternalServerError, "the log could not be read (internal error)")
		}
	}()
	files, he := s.logFiles(w, r)
	if he != nil {
		if he.status != 0 { // 0: decode has answered already
			writeError(w, he.status, he.msg)
		}
		return
	}
	l, err := logview.Build(files, logview.BuildOptions{Limit: maxLogText})
	switch {
	case errors.Is(err, logview.ErrTooLarge):
		writeError(w, http.StatusRequestEntityTooLarge, "the log is over 256 MB of text; open the part that matters")
		return
	case errors.Is(err, logview.ErrNotText):
		writeError(w, http.StatusUnprocessableEntity, err.Error())
		return
	case err != nil:
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	h, err := s.logs.add(l)
	if err != nil {
		writeError(w, http.StatusRequestEntityTooLarge, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, logOpened{ID: h.id, Files: l.Files, Summary: l.Summary(), Size: h.size})
}

// logFiles reads the request's logs.
func (s *Server) logFiles(w http.ResponseWriter, r *http.Request) ([]logview.File, *httpError) {
	ct, _, _ := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if ct == "application/json" {
		var req struct {
			Paths []string `json:"paths"`
		}
		if !decode(w, r, &req) {
			return nil, &httpError{0, ""}
		}
		ws := s.current()
		if ws == nil {
			return nil, &httpError{http.StatusConflict, "no folder is open"}
		}
		if len(req.Paths) == 0 {
			return nil, &httpError{http.StatusBadRequest, "no log was given"}
		}
		var files []logview.File
		for _, p := range req.Paths {
			data, err := ws.ReadLog(p, maxLogText)
			if errors.Is(err, workspace.ErrOverLimit) {
				return nil, &httpError{http.StatusRequestEntityTooLarge, p + " is over 256 MB; open the part that matters"}
			}
			if err != nil {
				return nil, readError(err)
			}
			files = append(files, logview.File{Name: path.Base(p), Data: data})
		}
		return files, nil
	}
	mr, err := r.MultipartReader()
	if err != nil {
		return nil, &httpError{http.StatusBadRequest, "send the logs as form data or JSON {\"paths\": [...]}"}
	}
	body := http.MaxBytesReader(w, r.Body, maxLogText+1<<20)
	r.Body = body
	var files []logview.File
	pasted := 0
	for {
		part, err := mr.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			if errors.As(err, new(*http.MaxBytesError)) {
				return nil, &httpError{http.StatusRequestEntityTooLarge, "the logs are over 256 MB; open the part that matters"}
			}
			return nil, &httpError{http.StatusBadRequest, "reading the logs: " + err.Error()}
		}
		data, err := io.ReadAll(part)
		if err != nil {
			if errors.As(err, new(*http.MaxBytesError)) {
				return nil, &httpError{http.StatusRequestEntityTooLarge, "the logs are over 256 MB; open the part that matters"}
			}
			return nil, &httpError{http.StatusBadRequest, "reading the logs: " + err.Error()}
		}
		name := part.FileName()
		if name == "" {
			pasted++
			name = "Pasted"
			if pasted > 1 {
				name += " " + strconv.Itoa(pasted)
			}
		}
		files = append(files, logview.File{Name: path.Base(strings.ReplaceAll(name, `\`, "/")), Data: data})
	}
	if len(files) == 0 {
		return nil, &httpError{http.StatusBadRequest, "no log was given"}
	}
	if len(files) > 1<<16-1 {
		return nil, &httpError{http.StatusBadRequest, "too many files at once"}
	}
	return files, nil
}

// held finds the log named in the path, or answers 410 Gone.
func (s *Server) held(w http.ResponseWriter, r *http.Request) *heldLog {
	h := s.logs.get(r.PathValue("id"))
	if h == nil {
		writeJSON(w, http.StatusGone, struct {
			Error   string `json:"error"`
			Expired bool   `json:"expired"`
		}{"this log is no longer held (idle for an hour, or codec restarted); open it again", true})
	}
	return h
}

// writeQueryError answers a query that can't be read with 422 and
// where the problem is.
func writeQueryError(w http.ResponseWriter, err error) {
	var qe *logview.QueryError
	if errors.As(err, &qe) {
		writeJSON(w, http.StatusUnprocessableEntity, struct {
			Error string `json:"error"`
			Pos   int    `json:"pos"`
		}{qe.Msg, qe.Pos})
		return
	}
	writeError(w, http.StatusInternalServerError, err.Error())
}

// logRow is an entry in a list: its core fields, the first line of its
// message, and the field columns asked for.
type logRow struct {
	I       int               `json:"i"`
	File    int               `json:"file"`
	Line    int               `json:"line"`
	Lines   int               `json:"lines,omitempty"`
	Time    string            `json:"time,omitempty"`
	Level   string            `json:"level,omitempty"`
	Source  string            `json:"source,omitempty"`
	Message string            `json:"message"`
	More    bool              `json:"more,omitempty"` // the message goes on (more lines, or cut)
	Format  string            `json:"format"`
	Error   string            `json:"error,omitempty"`
	Stack   bool              `json:"stack,omitempty"`
	Cols    map[string]string `json:"cols,omitempty"`
}

const rowText = 500 // characters of a message or error in a list

func cut(s string, n int) (string, bool) {
	more := false
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		s, more = strings.TrimRight(s[:i], "\r"), true
	}
	if utf8.RuneCountInString(s) > n {
		r := []rune(s)
		s, more = string(r[:n]), true
	}
	return s, more
}

func row(l *logview.Log, i int, cols []string) logRow {
	e := l.Entry(i)
	r := logRow{I: i, File: l.File(i), Line: e.Line, Level: e.Level.String(), Source: e.Source,
		Format: e.Format.String(), Stack: e.Stack != ""}
	if e.Lines > 1 {
		r.Lines = e.Lines
	}
	if !e.Time.IsZero() {
		r.Time = e.Time.Format(time.RFC3339Nano)
	}
	r.Message, r.More = cut(e.Message, rowText)
	r.Error, _ = cut(e.Error, rowText)
	for _, c := range cols {
		for _, f := range e.Fields {
			if f.Key == c {
				if r.Cols == nil {
					r.Cols = map[string]string{}
				}
				r.Cols[c], _ = cut(f.Text, rowText)
				break
			}
		}
	}
	return r
}

type entriesRequest struct {
	logQuery
	Offset    int      `json:"offset"`
	Limit     int      `json:"limit"`
	Columns   []string `json:"columns"`
	Histogram int      `json:"histogram"` // buckets wanted; 0 for none
	// At asks for the page holding this entry index instead of Offset
	// (to show an entry picked elsewhere, such as an error group's first).
	At *int `json:"at"`
}

type entriesResponse struct {
	*logview.Matches
	Offset    int                `json:"offset"`
	Entries   []logRow           `json:"entries"`
	Histogram *logview.Histogram `json:"histogram,omitempty"`
}

// POST /api/v2/logs/{id}/entries {q, from, to, offset, limit, columns,
// histogram, at}: a page of the entries a query matches.
func (s *Server) handleLogEntries(w http.ResponseWriter, r *http.Request) {
	h := s.held(w, r)
	if h == nil {
		return
	}
	var req entriesRequest
	if !decode(w, r, &req) {
		return
	}
	m, err := h.matches(req.logQuery)
	if err != nil {
		writeQueryError(w, err)
		return
	}
	limit := req.Limit
	if limit <= 0 || limit > maxPage {
		limit = 200
	}
	off := max(req.Offset, 0)
	if req.At != nil {
		// The page that holds the entry, or the next one after it.
		pos, _ := slices.BinarySearch(m.IDs, uint32(max(*req.At, 0)))
		off = pos / limit * limit
	}
	off = min(off, len(m.IDs))
	resp := entriesResponse{Matches: m, Offset: off, Entries: []logRow{}}
	for _, i := range m.IDs[off:min(off+limit, len(m.IDs))] {
		resp.Entries = append(resp.Entries, row(h.log, int(i), req.Columns))
	}
	if req.Histogram > 0 {
		from, to, _ := req.times()
		hg := h.log.Histogram(m.IDs, from, to, min(req.Histogram, 500))
		resp.Histogram = &hg
	}
	writeJSON(w, http.StatusOK, resp)
}

type entryField struct {
	Key  string `json:"key"`
	Kind string `json:"kind"`
	Text string `json:"text"`
}

type entryView struct {
	I        int          `json:"i"`
	File     string       `json:"file"`
	Line     int          `json:"line"`
	Lines    int          `json:"lines"`
	Format   string       `json:"format"`
	Source   string       `json:"source,omitempty"`
	Time     string       `json:"time,omitempty"`
	Level    string       `json:"level,omitempty"`
	RawLevel string       `json:"rawLevel,omitempty"`
	Message  string       `json:"message"`
	Logger   string       `json:"logger,omitempty"`
	Caller   string       `json:"caller,omitempty"`
	Error    string       `json:"error,omitempty"`
	Stack    string       `json:"stack,omitempty"`
	TraceID  string       `json:"traceId,omitempty"`
	Fields   []entryField `json:"fields"`
	Raw      string       `json:"raw"`
	RawCut   bool         `json:"rawCut,omitempty"`
}

var kindNames = [...]string{"string", "number", "bool", "null", "array", "object"}

const maxRaw = 1 << 20

// GET /api/v2/logs/{id}/entries/{n}?before=&after=: entry n in full,
// with the entries around it whatever the query (what led up to it).
func (s *Server) handleLogEntry(w http.ResponseWriter, r *http.Request) {
	h := s.held(w, r)
	if h == nil {
		return
	}
	n, err := strconv.Atoi(r.PathValue("n"))
	if err != nil || n < 0 || n >= h.log.Len() {
		writeError(w, http.StatusNotFound, "no such entry")
		return
	}
	num := func(k string) int {
		v, _ := strconv.Atoi(r.URL.Query().Get(k))
		return min(max(v, 0), 200)
	}
	e := h.log.Entry(n)
	v := entryView{I: n, File: h.log.Files[h.log.File(n)].Name, Line: e.Line, Lines: e.Lines, Format: e.Format.String(),
		Source: e.Source, Level: e.Level.String(), RawLevel: e.RawLevel, Message: e.Message, Logger: e.Logger,
		Caller: e.Caller, Error: e.Error, Stack: e.Stack, TraceID: e.TraceID, Fields: []entryField{}, Raw: e.Raw}
	if !e.Time.IsZero() {
		v.Time = e.Time.Format(time.RFC3339Nano)
	}
	if len(v.Raw) > maxRaw {
		v.Raw, v.RawCut = strings.ToValidUTF8(v.Raw[:maxRaw], ""), true
	}
	for _, f := range e.Fields {
		v.Fields = append(v.Fields, entryField{f.Key, kindNames[f.Kind], f.Text})
	}
	from, to := h.log.Around(n, num("before"), num("after"))
	around := []logRow{}
	for i := from; i < to; i++ {
		if i != n {
			around = append(around, row(h.log, i, nil))
		}
	}
	writeJSON(w, http.StatusOK, struct {
		Entry  entryView `json:"entry"`
		Around []logRow  `json:"around"`
	}{v, around})
}

// POST /api/v2/logs/{id}/fields {q, from, to, top}: every field of the
// matching entries with its values.
func (s *Server) handleLogFields(w http.ResponseWriter, r *http.Request) {
	h := s.held(w, r)
	if h == nil {
		return
	}
	var req struct {
		logQuery
		Top int `json:"top"`
	}
	if !decode(w, r, &req) {
		return
	}
	m, err := h.matches(req.logQuery)
	if err != nil {
		writeQueryError(w, err)
		return
	}
	top := req.Top
	if top <= 0 || top > 100 {
		top = 10
	}
	writeJSON(w, http.StatusOK, struct {
		Total  int                 `json:"total"`
		Fields []logview.FieldStat `json:"fields"`
	}{m.Total, h.log.FieldStats(m.IDs, top)})
}

// POST /api/v2/logs/{id}/groups {q, from, to}: the error groups of the
// matching entries, most frequent first.
func (s *Server) handleLogGroups(w http.ResponseWriter, r *http.Request) {
	h := s.held(w, r)
	if h == nil {
		return
	}
	var req logQuery
	if !decode(w, r, &req) {
		return
	}
	m, err := h.matches(req)
	if err != nil {
		writeQueryError(w, err)
		return
	}
	groups := h.log.Groups(m.IDs)
	writeJSON(w, http.StatusOK, struct {
		Groups []logview.Group `json:"groups"`
	}{groups[:min(len(groups), 1000)]})
}

// POST /api/v2/logs/{id}/export {q, from, to, format, columns}: the
// matching entries as a file: "text" (the lines as read), "ndjson"
// (one object per entry, in the common shape) or "csv" (time, level,
// source, message, then the columns).
func (s *Server) handleLogExport(w http.ResponseWriter, r *http.Request) {
	h := s.held(w, r)
	if h == nil {
		return
	}
	var req struct {
		logQuery
		Format  string   `json:"format"`
		Columns []string `json:"columns"`
	}
	if !decode(w, r, &req) {
		return
	}
	m, err := h.matches(req.logQuery)
	if err != nil {
		writeQueryError(w, err)
		return
	}
	name := "log"
	if len(h.log.Files) == 1 {
		name = strings.TrimSuffix(h.log.Files[0].Name, path.Ext(h.log.Files[0].Name))
	}
	kinds := map[string][2]string{
		"text":   {".log", "text/plain; charset=utf-8"},
		"ndjson": {".ndjson", "application/x-ndjson"},
		"csv":    {".csv", "text/csv; charset=utf-8"},
	}
	kind, ok := kinds[req.Format]
	if !ok {
		writeError(w, http.StatusBadRequest, "format is text, ndjson or csv")
		return
	}
	ext, ctype := kind[0], kind[1]
	w.Header().Set("Content-Type", ctype)
	w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": name + "-filtered" + ext}))
	w.WriteHeader(http.StatusOK)
	if err := exportLog(w, h.log, m.IDs, req.Format, req.Columns); err != nil {
		log.Printf("export: %v", err)
	}
}

func exportLog(w io.Writer, l *logview.Log, ids []uint32, format string, cols []string) error {
	switch format {
	case "text":
		for _, i := range ids {
			if _, err := io.WriteString(w, l.Entry(int(i)).Raw+"\n"); err != nil {
				return err
			}
		}
		return nil
	case "ndjson":
		enc := json.NewEncoder(w)
		enc.SetEscapeHTML(false)
		for _, i := range ids {
			e := l.Entry(int(i))
			o := struct {
				Time    string          `json:"time,omitempty"`
				Level   string          `json:"level,omitempty"`
				Source  string          `json:"source,omitempty"`
				File    string          `json:"file"`
				Line    int             `json:"line"`
				Message string          `json:"message"`
				Logger  string          `json:"logger,omitempty"`
				Caller  string          `json:"caller,omitempty"`
				Error   string          `json:"error,omitempty"`
				Stack   string          `json:"stack,omitempty"`
				TraceID string          `json:"traceId,omitempty"`
				Fields  json.RawMessage `json:"fields,omitempty"`
			}{Level: e.Level.String(), Source: e.Source, File: l.Files[l.File(int(i))].Name, Line: e.Line,
				Message: e.Message, Logger: e.Logger, Caller: e.Caller, Error: e.Error, Stack: e.Stack, TraceID: e.TraceID}
			if !e.Time.IsZero() {
				o.Time = e.Time.Format(time.RFC3339Nano)
			}
			if len(e.Fields) > 0 {
				o.Fields = orderedFields(e.Fields)
			}
			if err := enc.Encode(o); err != nil {
				return err
			}
		}
		return nil
	}
	cw := csv.NewWriter(w)
	if err := cw.Write(append([]string{"time", "level", "source", "message"}, cols...)); err != nil {
		return err
	}
	for _, i := range ids {
		e := l.Entry(int(i))
		rec := []string{"", e.Level.String(), e.Source, e.Message}
		if !e.Time.IsZero() {
			rec[0] = e.Time.Format(time.RFC3339Nano)
		}
		for _, c := range cols {
			v := ""
			for _, f := range e.Fields {
				if f.Key == c {
					v = f.Text
					break
				}
			}
			rec = append(rec, v)
		}
		if err := cw.Write(rec); err != nil {
			return err
		}
	}
	cw.Flush()
	return cw.Error()
}

// orderedFields writes fields as a JSON object in the order they were
// logged (a Go map would sort them).
func orderedFields(fields []logview.Field) json.RawMessage {
	var b bytes.Buffer
	b.WriteByte('{')
	for i, f := range fields {
		if i > 0 {
			b.WriteByte(',')
		}
		k, _ := json.Marshal(f.Key)
		b.Write(k)
		b.WriteByte(':')
		switch f.Kind {
		case logview.Number, logview.Bool, logview.Null, logview.Array, logview.Object:
			b.WriteString(f.Text) // valid JSON as read
		default:
			v, _ := json.Marshal(f.Text)
			b.Write(v)
		}
	}
	b.WriteByte('}')
	return b.Bytes()
}

// DELETE /api/v2/logs/{id}: let go of a log (the tab closed it).
func (s *Server) handleLogDelete(w http.ResponseWriter, r *http.Request) {
	if !s.logs.drop(r.PathValue("id")) {
		writeError(w, http.StatusNotFound, "no such log")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
