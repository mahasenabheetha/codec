package logview

import (
	"bytes"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"unicode/utf16"
	"unicode/utf8"
	"unsafe"
)

// ErrTooLarge is returned when a log, once decompressed, is over the limit.
var ErrTooLarge = errors.New("the log is too large")

// Decode turns a log's bytes into text: gzip is decompressed, UTF-8 and
// UTF-16 byte order marks are removed, and UTF-16 (as PowerShell's ">"
// writes) becomes UTF-8. A limit above zero caps the decompressed size.
// The returned text may share data's memory, so data must not change
// afterwards. encoding names what was undone ("gzip", "utf-16le"), for
// the summary.
func Decode(data []byte, limit int64) (text string, encoding []string, err error) {
	if len(data) >= 2 && data[0] == 0x1f && data[1] == 0x8b {
		zr, err := gzip.NewReader(bytes.NewReader(data))
		if err != nil {
			return "", nil, fmt.Errorf("reading gzip: %w", err)
		}
		r := io.Reader(zr)
		if limit > 0 {
			r = io.LimitReader(zr, limit+1)
		}
		out, err := io.ReadAll(r)
		if err != nil {
			return "", nil, fmt.Errorf("reading gzip: %w", err)
		}
		if limit > 0 && int64(len(out)) > limit {
			return "", nil, ErrTooLarge
		}
		data = out
		encoding = append(encoding, "gzip")
	}
	switch {
	case bytes.HasPrefix(data, []byte{0xef, 0xbb, 0xbf}):
		data = data[3:]
	case bytes.HasPrefix(data, []byte{0xff, 0xfe}):
		data = fromUTF16(data[2:], false)
		encoding = append(encoding, "utf-16le")
	case bytes.HasPrefix(data, []byte{0xfe, 0xff}):
		data = fromUTF16(data[2:], true)
		encoding = append(encoding, "utf-16be")
	}
	if len(data) == 0 {
		return "", encoding, nil
	}
	// data is never written again, so the text can share its bytes.
	return unsafe.String(&data[0], len(data)), encoding, nil
}

func fromUTF16(b []byte, bigEndian bool) []byte {
	units := make([]uint16, len(b)/2)
	for i := range units {
		if bigEndian {
			units[i] = uint16(b[2*i])<<8 | uint16(b[2*i+1])
		} else {
			units[i] = uint16(b[2*i]) | uint16(b[2*i+1])<<8
		}
	}
	out := make([]byte, 0, len(units)+len(units)/2)
	for _, r := range utf16.Decode(units) {
		out = utf8.AppendRune(out, r)
	}
	return out
}
