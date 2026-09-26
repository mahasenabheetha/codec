package yamlkit

import (
	"bytes"
	"sort"
	"unicode/utf8"
)

var bom = []byte{0xEF, 0xBB, 0xBF}

// stripBOM removes a leading UTF-8 byte-order mark. Editors and browser
// decoders hide it, so positions are reported without it too.
func stripBOM(src []byte) ([]byte, bool) {
	if bytes.HasPrefix(src, bom) {
		return src[len(bom):], true
	}
	return src, false
}

// lineIndex converts between byte offsets and line/column positions.
// Columns count runes, so non-ASCII text lines up with what an editor
// shows; a trailing \r (CRLF files) is not part of a line's text.
type lineIndex struct {
	src    []byte
	starts []int // byte offset where each line begins; starts[0] == 0
}

func newLineIndex(src []byte) *lineIndex {
	starts := []int{0}
	for i, b := range src {
		if b == '\n' {
			starts = append(starts, i+1)
		}
	}
	return &lineIndex{src: src, starts: starts}
}

func (li *lineIndex) lines() int { return len(li.starts) }

// line returns the text of a 1-based line without its line ending.
func (li *lineIndex) line(n int) string {
	if n < 1 || n > len(li.starts) {
		return ""
	}
	start := li.starts[n-1]
	end := len(li.src)
	if n < len(li.starts) {
		end = li.starts[n] - 1 // drop \n
	}
	if end > start && li.src[end-1] == '\r' {
		end-- // drop \r of CRLF
	}
	return string(li.src[start:end])
}

// pos converts a byte offset into a Pos.
func (li *lineIndex) pos(offset int) Pos {
	offset = max(0, min(offset, len(li.src)))
	// The line is the last start at or before offset.
	i := sort.Search(len(li.starts), func(i int) bool { return li.starts[i] > offset }) - 1
	col := utf8.RuneCount(li.src[li.starts[i]:offset]) + 1
	return Pos{Offset: offset, Line: i + 1, Col: col}
}

// at converts a 1-based line and rune column into a Pos, clamping to
// the line's end.
func (li *lineIndex) at(line, col int) Pos {
	line = max(1, min(line, len(li.starts)))
	off := li.starts[line-1]
	end := len(li.src)
	if line < len(li.starts) {
		end = li.starts[line] - 1
	}
	for c := 1; c < col && off < end; c++ {
		_, size := utf8.DecodeRune(li.src[off:end])
		off += size
	}
	return li.pos(off)
}

// span builds a Range from byte offsets.
func (li *lineIndex) span(start, end int) Range {
	return Range{Start: li.pos(start), End: li.pos(end)}
}
