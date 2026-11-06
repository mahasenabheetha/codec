// Marks on the command line: the part the pointer is on in the
// explanation, risky parts, and parts codec's reference doesn't have.

import { RangeSetBuilder, type Extension } from '@codemirror/state'
import { Decoration, EditorView } from '@codemirror/view'
import type { Part } from '../../lib/api/commands'

export function lineMarks(parts: Part[], hovered: number | null): Extension {
  return [
    EditorView.decorations.of((view) => {
      const len = view.state.doc.length
      const b = new RangeSetBuilder<Decoration>()
      parts.forEach((p, i) => {
        const from = Math.min(p.start, len)
        const to = Math.min(p.end, len)
        if (to <= from) return
        const cls = [
          i === hovered ? 'cm-cmd-hover' : '',
          p.danger ? `cm-cmd-${p.danger.level}` : '',
          p.kind === 'unknown' ? 'cm-cmd-unknown' : '',
        ].filter(Boolean)
        if (cls.length) b.add(from, to, Decoration.mark({ class: cls.join(' ') }))
      })
      return b.finish()
    }),
    EditorView.theme({
      '.cm-cmd-hover': { backgroundColor: 'var(--accent-soft)', borderRadius: '2px', boxShadow: 'inset 0 -2px 0 var(--accent)' },
      '.cm-cmd-danger': { textDecoration: 'underline wavy var(--err)', textUnderlineOffset: '3px' },
      '.cm-cmd-caution': { textDecoration: 'underline wavy var(--warn)', textUnderlineOffset: '3px' },
      '.cm-cmd-unknown': { textDecoration: 'underline dotted var(--fg-2)', textUnderlineOffset: '3px' },
    }),
  ]
}
