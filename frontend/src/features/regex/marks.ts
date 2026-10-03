// Match highlights in the test text: every match, the selected one
// outlined, and the spans of a hovered group.

import { RangeSetBuilder, type Extension } from '@codemirror/state'
import { Decoration, EditorView } from '@codemirror/view'
import type { RegexMatch } from '../../lib/api/regex'

export function matchMarks(matches: RegexMatch[], selected: number, group: number | null): Extension {
  return [
    EditorView.decorations.of((view) => {
      const len = view.state.doc.length
      const ranges: { from: number; to: number; cls: string }[] = []
      matches.forEach((m, i) => {
        const from = Math.min(m.start, len)
        const to = Math.min(m.end, len)
        if (to > from) ranges.push({ from, to, cls: `cm-rx-match cm-rx-${i % 2 ? 'odd' : 'even'}${i === selected ? ' cm-rx-selected' : ''}` })
        if (group === null) return
        const g = m.groups?.find((x) => x.number === group)
        if (g && g.start >= 0 && g.end > g.start) ranges.push({ from: Math.min(g.start, len), to: Math.min(g.end, len), cls: 'cm-rx-group' })
      })
      ranges.sort((a, b) => a.from - b.from || b.to - a.to)
      const b = new RangeSetBuilder<Decoration>()
      for (const r of ranges) b.add(r.from, r.to, Decoration.mark({ class: r.cls }))
      return b.finish()
    }),
    EditorView.theme({
      '.cm-rx-even': { backgroundColor: 'var(--accent-soft)', borderBottom: '1px solid var(--accent)' },
      '.cm-rx-odd': { backgroundColor: 'var(--info-soft)', borderBottom: '1px solid var(--info)' },
      '.cm-rx-selected': { outline: '1px solid var(--accent)', borderRadius: '2px' },
      '.cm-rx-group': { backgroundColor: 'var(--warn-soft)', boxShadow: 'inset 0 -2px 0 var(--warn)' },
    }),
  ]
}
