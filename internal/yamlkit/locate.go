package yamlkit

// Helpers for editor features (hover, definition, completion) that
// start from a cursor position.

// DocAt returns the document a position falls in: the last one that
// starts at or before its line. Nil for a file without documents.
func (f *File) DocAt(pos Pos) *Document {
	var doc *Document
	for _, d := range f.Docs {
		if d.Range.Start.Line <= pos.Line || doc == nil {
			doc = d
		}
	}
	return doc
}

// PosAt converts a 1-based line and rune column in content (as given
// to Parse, BOM and all) into a Pos with its byte offset.
func PosAt(content []byte, line, col int) Pos {
	src, _ := stripBOM(content)
	return newLineIndex(src).at(line, col)
}

// Text returns the source text a range covers. content is what was
// given to Parse; offsets are relative to it without the BOM.
func Text(content []byte, r Range) string {
	src, _ := stripBOM(content)
	start := max(0, min(r.Start.Offset, len(src)))
	end := max(start, min(r.End.Offset, len(src)))
	return string(src[start:end])
}

// Pair returns the map entry for key, or nil.
func (n *Node) Pair(key string) *Pair {
	if n == nil || n.Kind != KindMap {
		return nil
	}
	for _, p := range n.Pairs {
		if p.Key != nil && p.Key.Kind == KindScalar && p.Key.Value == key {
			return p
		}
	}
	return nil
}

// Walk calls fn for n and every node below it (keys included), depth
// first in document order. Returning false skips a node's children.
func Walk(n *Node, fn func(*Node) bool) {
	if n == nil || !fn(n) {
		return
	}
	for _, p := range n.Pairs {
		Walk(p.Key, fn)
		Walk(p.Value, fn)
	}
	for _, it := range n.Items {
		Walk(it, fn)
	}
}
