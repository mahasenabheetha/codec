package jsonkit

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

// Stats is what a scan of strict JSON found, without building a tree:
// for input too large to format or show as a tree in the page.
type Stats struct {
	Bytes    int64 `json:"bytes"`
	Values   int   `json:"values"` // top-level values (JSON Lines has several)
	Objects  int   `json:"objects"`
	Arrays   int   `json:"arrays"`
	Keys     int   `json:"keys"`
	Strings  int   `json:"strings"`
	Numbers  int   `json:"numbers"`
	Bools    int   `json:"bools"`
	Nulls    int   `json:"nulls"`
	MaxDepth int   `json:"maxDepth"`
}

// countingReader counts the bytes read through it.
type countingReader struct {
	r io.Reader
	n int64
}

func (c *countingReader) Read(p []byte) (int, error) {
	n, err := c.r.Read(p)
	c.n += int64(n)
	return n, err
}

// frame is one open object or array while scanning.
type frame struct {
	object  bool
	wantKey bool // in an object: the next string is a key
}

// Scan reads strict JSON (one value or several) token by token, keeping
// only counts: memory stays flat however big the input. An error says
// roughly where (a byte offset); Parse gives the exact line and column.
func Scan(r io.Reader) (Stats, error) {
	cr := &countingReader{r: r}
	dec := json.NewDecoder(cr)
	dec.UseNumber()
	var st Stats
	var stack []frame
	// valueDone: the enclosing object (if any) wants a key next.
	valueDone := func() {
		if n := len(stack); n > 0 && stack[n-1].object {
			stack[n-1].wantKey = true
		}
	}
	for {
		tok, err := dec.Token()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return st, fmt.Errorf("not JSON near byte %d: %w", dec.InputOffset(), err)
		}
		if d, ok := tok.(json.Delim); ok && (d == '}' || d == ']') {
			stack = stack[:len(stack)-1]
			valueDone()
			continue
		}
		if n := len(stack); n > 0 && stack[n-1].object && stack[n-1].wantKey {
			st.Keys++ // the decoder guarantees a key here is a string
			stack[n-1].wantKey = false
			continue
		}
		if len(stack) == 0 {
			st.Values++
		}
		switch t := tok.(type) {
		case json.Delim: // { or [
			if t == '{' {
				st.Objects++
			} else {
				st.Arrays++
			}
			stack = append(stack, frame{object: t == '{', wantKey: t == '{'})
			st.MaxDepth = max(st.MaxDepth, len(stack))
			continue // done when it closes
		case string:
			st.Strings++
		case json.Number:
			st.Numbers++
		case bool:
			st.Bools++
		case nil:
			st.Nulls++
		}
		valueDone()
	}
	st.Bytes = cr.n
	return st, nil
}
