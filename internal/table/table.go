// Package table is the engine behind the CSV & TSV tab (3.7.0): delimited
// text read with quotes and embedded newlines, the delimiter and the
// header row guessed when not given, a type and counts per column, and
// conversions to JSON, YAML, Markdown and back to CSV or TSV. It has no
// input or output of its own.
package table

import (
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"strings"
	"unicode/utf8"
)

// Delimiters are the ones guessed between, in order of preference.
var Delimiters = []rune{',', '\t', ';', '|'}

// DistinctLimit caps the distinct values counted per column.
const DistinctLimit = 10000

// Options says how to read; the zero value guesses everything.
type Options struct {
	Delim  rune   // 0: guess
	Header string // "yes", "no", or "" / "auto": guess
}

// Column is one column: its name and what its values look like.
type Column struct {
	Name string `json:"name"`
	// Type is integer, number, boolean, date or text, or empty when no
	// row has a value.
	Type         string `json:"type"`
	Filled       int    `json:"filled"`
	Empty        int    `json:"empty"`
	Distinct     int    `json:"distinct"`
	DistinctMore bool   `json:"distinctMore,omitempty"` // more than DistinctLimit
}

// Table is delimited text read into rows of the same width.
type Table struct {
	Delim         rune
	Header        bool // the first row named the columns
	DelimGuessed  bool
	HeaderGuessed bool
	Columns       []Column
	Rows          [][]string // without the header; each len(Columns) long
	Ragged        int        // rows that had more or fewer fields than the rest
}

// SyntaxError is delimited text that can't be read, with its place
// (line and column counted in characters, from 1).
type SyntaxError struct {
	Line, Column int
	Msg          string
}

func (e *SyntaxError) Error() string {
	return fmt.Sprintf("line %d, column %d: %s", e.Line, e.Column, e.Msg)
}

const bom = "\xef\xbb\xbf"

// Parse reads src as delimited text.
func Parse(src string, o Options) (*Table, error) {
	src = strings.TrimPrefix(src, bom)
	t := &Table{Delim: o.Delim}
	if t.Delim == 0 {
		t.Delim, t.DelimGuessed = guessDelim(src), true
	}
	recs, err := read(src, t.Delim)
	if err != nil {
		return nil, err
	}
	switch o.Header {
	case "yes":
		t.Header = true
	case "no":
	case "", "auto":
		t.Header, t.HeaderGuessed = guessHeader(recs), true
	default:
		return nil, fmt.Errorf("header is yes, no or auto, not %q", o.Header)
	}
	t.build(recs)
	return t, nil
}

// build names the columns, pads the rows and counts what they hold.
func (t *Table) build(recs [][]string) {
	width, want := 0, 0
	for _, r := range recs {
		width = max(width, len(r))
	}
	var names []string
	body := recs
	if t.Header && len(recs) > 0 {
		names, body = recs[0], recs[1:]
		want = len(names)
	} else {
		want = modeWidth(recs)
	}
	t.Columns = make([]Column, width)
	seen := map[string]int{}
	for i := range t.Columns {
		name := ""
		if i < len(names) {
			name = strings.TrimSpace(names[i])
		}
		if name == "" {
			name = fmt.Sprintf("column %d", i+1)
		}
		if n := seen[name]; n > 0 {
			seen[name] = n + 1
			name = fmt.Sprintf("%s_%d", name, n+1)
		}
		seen[name]++
		t.Columns[i].Name = name
	}
	t.Rows = make([][]string, len(body))
	for i, r := range body {
		if len(r) != want {
			t.Ragged++
		}
		if len(r) < width {
			r = append(r, make([]string, width-len(r))...)
		}
		t.Rows[i] = r
	}
	t.count()
}

// count fills in each column's type and counts.
func (t *Table) count() {
	for c := range t.Columns {
		col := &t.Columns[c]
		var k kind
		distinct := map[string]struct{}{}
		for _, r := range t.Rows {
			v := r[c]
			vk := kindOf(v)
			if vk == empty {
				col.Empty++
				continue
			}
			col.Filled++
			k = merge(k, vk)
			if len(distinct) < DistinctLimit {
				distinct[v] = struct{}{}
			} else if _, ok := distinct[v]; !ok {
				col.DistinctMore = true
			}
		}
		col.Type = k.String()
		col.Distinct = len(distinct)
	}
}

// read splits src into records. Quotes are optional in TSV, so a quote
// inside a TSV field is just a character.
func read(src string, delim rune) ([][]string, error) {
	r := csv.NewReader(strings.NewReader(src))
	r.Comma = delim
	r.FieldsPerRecord = -1
	r.LazyQuotes = delim == '\t'
	var recs [][]string
	for {
		rec, err := r.Read()
		if err == io.EOF {
			return recs, nil
		}
		if err != nil {
			return nil, syntaxError(src, err)
		}
		recs = append(recs, rec)
	}
}

// syntaxError turns the csv package's error into one with a character
// column and plain words.
func syntaxError(src string, err error) error {
	var pe *csv.ParseError
	if !errors.As(err, &pe) {
		return err
	}
	msg := pe.Err.Error()
	switch {
	case errors.Is(pe.Err, csv.ErrQuote):
		msg = "a quoted field isn't closed, or has text after its closing quote (a quote inside a quoted field is written twice: \"\")"
	case errors.Is(pe.Err, csv.ErrBareQuote):
		msg = "a quote inside a field without quotes (put the field in quotes and write the quote twice)"
	}
	line := ""
	if lines := strings.SplitN(src, "\n", pe.Line+1); pe.Line >= 1 && pe.Line <= len(lines) {
		line = lines[pe.Line-1]
	}
	col := pe.Column
	if col >= 1 && col-1 <= len(line) {
		col = utf8.RuneCountInString(line[:col-1]) + 1
	}
	return &SyntaxError{Line: pe.Line, Column: max(col, 1), Msg: msg}
}

// guessDelim picks the delimiter that splits the first rows most evenly
// into two or more fields; a comma when none does.
func guessDelim(src string) rune {
	sample := src
	if len(sample) > 64<<10 {
		sample = sample[:64<<10]
		if i := strings.LastIndexByte(sample, '\n'); i > 0 {
			sample = sample[:i]
		}
	}
	best, bestScore := ',', 0.0
	for _, d := range Delimiters {
		recs := sampleRecords(sample, d, 50)
		width := modeWidth(recs)
		if width < 2 {
			continue
		}
		same := 0
		for _, r := range recs {
			if len(r) == width {
				same++
			}
		}
		// How evenly first; more fields only to break a tie.
		score := float64(same)/float64(len(recs))*100 + float64(min(width, 50))/100
		if score > bestScore {
			best, bestScore = d, score
		}
	}
	return best
}

// sampleRecords reads up to n records, stopping quietly at an error (the
// sample may end inside a quoted field).
func sampleRecords(src string, delim rune, n int) [][]string {
	r := csv.NewReader(strings.NewReader(src))
	r.Comma = delim
	r.FieldsPerRecord = -1
	r.LazyQuotes = true
	var recs [][]string
	for len(recs) < n {
		rec, err := r.Read()
		if err != nil {
			break
		}
		recs = append(recs, rec)
	}
	return recs
}

// modeWidth is the most common number of fields (the larger on a tie).
func modeWidth(recs [][]string) int {
	counts := map[int]int{}
	best := 0
	for _, r := range recs {
		counts[len(r)]++
		n := len(r)
		if counts[n] > counts[best] || counts[n] == counts[best] && n > best {
			best = n
		}
	}
	return best
}

// guessHeader says whether the first row names the columns: its cells
// are filled, different and text, and below it a column holds numbers,
// dates or booleans, or text of one length the name doesn't have. When
// nothing tells either way (text below text) it is a header, unless a
// name also turns up as a value in its column.
func guessHeader(recs [][]string) bool {
	if len(recs) == 0 {
		return false
	}
	first := recs[0]
	seen := map[string]bool{}
	for _, c := range first {
		c = strings.TrimSpace(c)
		if c == "" || seen[c] || kindOf(c) != text {
			return false
		}
		seen[c] = true
	}
	body := recs[1:min(len(recs), 101)]
	votes := 0
	for i, name := range first {
		var k kind
		length, sameLength, inColumn := -1, true, false
		for _, r := range body {
			if i >= len(r) {
				continue
			}
			v := r[i]
			if vk := kindOf(v); vk != empty {
				k = merge(k, vk)
			}
			n := utf8.RuneCountInString(v)
			if length == -1 {
				length = n
			} else if n != length {
				sameLength = false
			}
			if strings.TrimSpace(v) == strings.TrimSpace(name) {
				inColumn = true
			}
		}
		switch {
		case k != text && k != empty:
			votes++
		case inColumn:
			votes--
		case length >= 0 && sameLength && utf8.RuneCountInString(name) != length:
			votes++
		}
	}
	return votes >= 0
}
