// Marks on the command line: the part the pointer is on in the
// explanation, risky parts, parts codec's reference doesn't have, and a
// recipe's {placeholders} still to fill in.

import { Prec, RangeSetBuilder, type Extension } from '@codemirror/state'
import { Decoration, EditorView, keymap } from '@codemirror/view'
import type { Part } from '../../lib/api/commands'

/** A value to fill in, as recipes write it: {branch}. */
export const placeholderRe = /\{[a-z][a-z0-9-]*\}/g

/** The placeholders in a line, by their names. */
export function placeholders(line: string): string[] {
  return [...line.matchAll(placeholderRe)].map((m) => m[0])
}

/** Selects the next placeholder after pos (dir 1) or the one before it
 *  (dir -1), so typing replaces it. false when there is none. */
export function selectPlaceholder(view: EditorView, pos: number, dir: 1 | -1 = 1): boolean {
  const all = [...view.state.doc.toString().matchAll(placeholderRe)].map((m) => ({ from: m.index, to: m.index + m[0].length }))
  const next = dir === 1 ? all.find((p) => p.from >= pos) : all.findLast((p) => p.to <= pos)
  if (!next) return false
  view.dispatch({ selection: { anchor: next.from, head: next.to }, scrollIntoView: true })
  return true
}

/** Tab and Shift+Tab move between placeholders while there are any;
 *  otherwise the keys do what they always do. */
export const placeholderKeys: Extension = Prec.high(
  keymap.of([
    { key: 'Tab', run: (v) => selectPlaceholder(v, v.state.selection.main.to) },
    { key: 'Shift-Tab', run: (v) => selectPlaceholder(v, v.state.selection.main.from, -1) },
  ]),
)

const hole = Decoration.mark({ class: 'cm-cmd-hole' })

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
    // Placeholders from the text itself: they show at once, before the
    // line is explained again.
    EditorView.decorations.of((view) => {
      const b = new RangeSetBuilder<Decoration>()
      for (const m of view.state.doc.toString().matchAll(placeholderRe)) b.add(m.index, m.index + m[0].length, hole)
      return b.finish()
    }),
    EditorView.theme({
      '.cm-cmd-hover': { backgroundColor: 'var(--accent-soft)', borderRadius: '2px', boxShadow: 'inset 0 -2px 0 var(--accent)' },
      '.cm-cmd-danger': { textDecoration: 'underline wavy var(--err)', textUnderlineOffset: '3px' },
      '.cm-cmd-caution': { textDecoration: 'underline wavy var(--warn)', textUnderlineOffset: '3px' },
      '.cm-cmd-hole': { outline: '1px dashed var(--accent)', outlineOffset: '-1px', borderRadius: '2px', color: 'var(--accent)' },
      '.cm-cmd-unknown': { textDecoration: 'underline dotted var(--fg-2)', textUnderlineOffset: '3px' },
    }),
  ]
}
