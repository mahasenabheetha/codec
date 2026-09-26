// Line highlights for the two sides of a comparison: removed lines on
// the left, added lines on the right, changed lines on both, and the
// selected change outlined.

import { RangeSetBuilder, type Extension } from '@codemirror/state'
import { Decoration, EditorView } from '@codemirror/view'
import type { Change } from '../../lib/api/compare'
import { offsetOf } from '../editor/intel'

export function changeMarks(changes: Change[], side: 'old' | 'new', selected: number): Extension {
  return [
    EditorView.decorations.of((view) => {
      const doc = view.state.doc
      const byLine = new Map<number, string>()
      changes.forEach((c, i) => {
        const r = side === 'old' ? c.oldRange : c.newRange
        if (!r) return
        const cls = `cm-change cm-change-${c.kind}${i === selected ? ' cm-change-selected' : ''}`
        const first = doc.lineAt(offsetOf(doc, r.start)).number
        // End is exclusive: a range ending at column 1 stops before that line.
        const endLine = r.end.col <= 1 && r.end.line > r.start.line ? r.end.line - 1 : r.end.line
        const last = Math.min(doc.lines, Math.max(first, endLine))
        for (let n = first; n <= last; n++) {
          // A selected change wins; otherwise the first mark stays.
          if (!byLine.has(n) || i === selected) byLine.set(n, cls)
        }
      })
      const b = new RangeSetBuilder<Decoration>()
      for (const n of [...byLine.keys()].sort((x, y) => x - y)) {
        b.add(doc.line(n).from, doc.line(n).from, Decoration.line({ class: byLine.get(n)! }))
      }
      return b.finish()
    }),
    EditorView.theme({
      '.cm-change-added': { backgroundColor: 'var(--diff-add-bg)' },
      '.cm-change-removed': { backgroundColor: 'var(--diff-del-bg)' },
      '.cm-change-changed, .cm-change-reordered': { backgroundColor: 'var(--diff-chg-bg)' },
      '.cm-change-selected': { boxShadow: 'inset 3px 0 0 var(--accent)' },
    }),
  ]
}

/** Scroll a view to a change's range on one side and select its lines. */
export function reveal(view: EditorView | undefined, c: Change, side: 'old' | 'new') {
  const r = side === 'old' ? c.oldRange : c.newRange
  if (!view || !r) return
  const doc = view.state.doc
  const from = doc.lineAt(offsetOf(doc, r.start)).from
  view.dispatch({ effects: EditorView.scrollIntoView(from, { y: 'center' }) })
}
