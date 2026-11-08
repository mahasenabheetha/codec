package sanitize

import (
	"bytes"
	"io"
)

// chunk is how much a Writer gathers before it sanitizes: enough that a
// value is rarely cut, little enough that a big export streams.
const chunk = 1 << 20

// A Writer sanitizes what is written to it on its way to w, a chunk of
// whole lines at a time, each value the same stand-in throughout (a log
// export of any size, without holding it all). Close writes the rest.
type Writer struct {
	w   io.Writer
	z   *Sanitizer
	buf []byte
}

// NewWriter sanitizes for o what is written, into w.
func NewWriter(w io.Writer, o Options) *Writer { return &Writer{w: w, z: New(o)} }

func (s *Writer) Write(p []byte) (int, error) {
	s.buf = append(s.buf, p...)
	if len(s.buf) < chunk {
		return len(p), nil
	}
	// Up to the last whole line, unless a private key is still open: it
	// is one value over many lines.
	cut := bytes.LastIndexByte(s.buf, '\n') + 1
	if cut == 0 || openKey(s.buf[:cut]) {
		return len(p), nil
	}
	if err := s.flush(cut); err != nil {
		return 0, err
	}
	return len(p), nil
}

// Close sanitizes and writes what is left.
func (s *Writer) Close() error { return s.flush(len(s.buf)) }

// Findings are the values taken out so far.
func (s *Writer) Findings() []Finding { return s.z.Findings() }

func (s *Writer) flush(n int) error {
	if n == 0 {
		return nil
	}
	_, err := io.WriteString(s.w, s.z.Text(string(s.buf[:n])))
	s.buf = append(s.buf[:0], s.buf[n:]...)
	return err
}

// openKey: a private key begins in b and doesn't end there.
func openKey(b []byte) bool {
	begin := bytes.LastIndex(b, []byte("-----BEGIN "))
	return begin >= 0 && bytes.Contains(b[begin:], []byte("PRIVATE KEY")) && !bytes.Contains(b[begin:], []byte("-----END "))
}
