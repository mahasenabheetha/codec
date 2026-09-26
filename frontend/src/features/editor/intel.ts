// CodeMirror extensions that turn the viewer into a YAML editor backed
// by the engine: expression overlays, diagnostics, hover, go to
// definition, completion, cursor tracking and go to line.
//
// Positions from the API are 1-based line and rune column; CodeMirror
// uses offsets in UTF-16 units. They agree except for characters
// outside the BMP (emoji), which decision #20 accepts.

import { autocompletion, type CompletionContext, type CompletionResult } from '@codemirror/autocomplete'
import { gotoLine } from '@codemirror/search'
import { lintGutter, setDiagnostics, type Diagnostic as CmDiagnostic } from '@codemirror/lint'
import { StateEffect, StateField, type Extension, type Text } from '@codemirror/state'
import { Decoration, EditorView, hoverTooltip, keymap, type DecorationSet } from '@codemirror/view'
import type { Analysis, Completion, Hover, Location, Pos } from '../../lib/api/yaml'

/** Offset of a 1-based line/column, clamped to the document. */
export function offsetOf(doc: Text, p: Pick<Pos, 'line' | 'col'>): number {
  const line = doc.line(Math.min(Math.max(p.line, 1), doc.lines))
  return Math.min(line.from + Math.max(p.col - 1, 0), line.to)
}

/** 1-based line/column of an offset. */
export function lineCol(doc: Text, offset: number): { line: number; col: number } {
  const line = doc.lineAt(offset)
  return { line: line.number, col: offset - line.from + 1 }
}

// --- expression overlays ---

const setExpressions = StateEffect.define<DecorationSet>()

// Kept in a field so the marks move with edits until the next analysis
// replaces them.
const expressionField = StateField.define<DecorationSet>({
  create: () => Decoration.none,
  update(deco, tr) {
    for (const e of tr.effects) if (e.is(setExpressions)) return e.value
    return deco.map(tr.changes)
  },
  provide: (f) => EditorView.decorations.from(f),
})

const exprMark = {
  template: Decoration.mark({ class: 'cm-expr cm-expr-template' }),
  runtime: Decoration.mark({ class: 'cm-expr cm-expr-runtime' }),
}

/** Push an analysis into the editor: overlays and diagnostics. Only
 *  call it when the analysis was computed for the current text. */
export function applyAnalysis(view: EditorView, a: Analysis | null) {
  const doc = view.state.doc
  const marks = (a?.expressions ?? [])
    .map((e) => ({ from: offsetOf(doc, e.range.start), to: offsetOf(doc, e.range.end), phase: e.phase }))
    .filter((m) => m.to > m.from)
    .sort((x, y) => x.from - y.from)
    .map((m) => exprMark[m.phase].range(m.from, m.to))

  const diagnostics: CmDiagnostic[] = (a?.diagnostics ?? []).map((d) => {
    const from = offsetOf(doc, d.range.start)
    return {
      from,
      to: Math.max(from, offsetOf(doc, d.range.end)),
      severity: d.severity === 'info' ? 'info' : d.severity,
      source: d.code,
      message: d.message,
      renderMessage: () => {
        const el = document.createElement('div')
        el.className = 'cm-diag'
        el.append(d.message)
        if (d.hint) {
          const hint = document.createElement('div')
          hint.className = 'cm-diag-hint'
          hint.textContent = d.hint
          el.append(hint)
        }
        return el
      },
    }
  })

  view.dispatch(setDiagnostics(view.state, diagnostics), {
    effects: setExpressions.of(Decoration.set(marks, true)),
  })
}

// --- hooks into the app ---

export interface IntelHooks {
  hover(line: number, col: number): Promise<Hover | null>
  definition(line: number, col: number): Promise<Location[]>
  complete(line: number, col: number): Promise<Completion[]>
  cursor(line: number, col: number): void
}

function hoverCard(h: Hover): HTMLElement {
  const el = document.createElement('div')
  el.className = 'cm-hover'
  const title = document.createElement('div')
  title.className = 'cm-hover-title'
  title.textContent = h.title
  el.append(title)
  if (h.rows?.length) {
    const dl = document.createElement('dl')
    for (const r of h.rows) {
      const dt = document.createElement('dt')
      dt.textContent = r.label
      const dd = document.createElement('dd')
      dd.textContent = r.value
      dl.append(dt, dd)
    }
    el.append(dl)
  }
  if (h.code) {
    const pre = document.createElement('pre')
    pre.textContent = h.code
    el.append(pre)
  }
  return el
}

async function goToDefinition(view: EditorView, hooks: IntelHooks, at: number): Promise<boolean> {
  const { line, col } = lineCol(view.state.doc, at)
  const locs = await hooks.definition(line, col).catch(() => [])
  const target = locs.find((l) => !l.path)
  if (!target) return false
  const from = offsetOf(view.state.doc, target.range.start)
  const to = Math.min(offsetOf(view.state.doc, target.range.end), view.state.doc.lineAt(from).to)
  view.dispatch({ selection: { anchor: from, head: to }, scrollIntoView: true })
  view.focus()
  return true
}

/** The editor extensions, wired to the app through hooks. */
export function yamlIntel(hooks: IntelHooks): Extension {
  return [
    expressionField,
    lintGutter(),

    hoverTooltip(
      async (view, pos) => {
        const { line, col } = lineCol(view.state.doc, pos)
        const h = await hooks.hover(line, col).catch(() => null)
        if (!h) return null
        const doc = view.state.doc
        return {
          pos: offsetOf(doc, h.range.start),
          end: Math.max(offsetOf(doc, h.range.start), offsetOf(doc, h.range.end)),
          above: true,
          create: () => ({ dom: hoverCard(h) }),
        }
      },
      { hoverTime: 350 },
    ),

    autocompletion({
      icons: false,
      override: [
        async (ctx: CompletionContext): Promise<CompletionResult | null> => {
          // Only aliases for now (anchors defined above); schema-driven
          // keys and values arrive with the lenses.
          const word = ctx.matchBefore(/\*[\w.-]*/)
          if (!word) return null
          const { line, col } = lineCol(ctx.state.doc, ctx.pos)
          const items = await hooks.complete(line, col).catch(() => [])
          if (ctx.aborted || items.length === 0) return null
          return {
            from: offsetOf(ctx.state.doc, items[0].range.start),
            options: items.map((i) => ({ label: i.label, detail: i.detail, type: i.kind })),
            validFor: /^[\w.-]*$/,
          }
        },
      ],
    }),

    keymap.of([
      { key: 'F12', run: (view) => (goToDefinition(view, hooks, view.state.selection.main.head), true) },
      { key: 'Mod-g', run: gotoLine },
    ]),

    EditorView.domEventHandlers({
      // Ctrl/⌘+click jumps to a definition, like in VS Code.
      mousedown(e, view) {
        if (!(e.ctrlKey || e.metaKey) || e.button !== 0) return false
        const at = view.posAtCoords({ x: e.clientX, y: e.clientY })
        if (at === null) return false
        e.preventDefault()
        goToDefinition(view, hooks, at)
        return true
      },
    }),

    EditorView.updateListener.of((u) => {
      if (u.selectionSet || u.docChanged) {
        const { line, col } = lineCol(u.state.doc, u.state.selection.main.head)
        hooks.cursor(line, col)
      }
    }),

    EditorView.theme({
      '.cm-expr': { borderRadius: '3px' },
      '.cm-expr-template': { backgroundColor: 'var(--syn-expr-tpl-bg)' },
      '.cm-expr-template, .cm-expr-template *': { color: 'var(--syn-expr-tpl) !important' },
      '.cm-expr-runtime': { backgroundColor: 'var(--syn-expr-rt-bg)' },
      '.cm-expr-runtime, .cm-expr-runtime *': { color: 'var(--syn-expr-rt) !important' },

      // Squiggles in the theme colours (the default SVG can't use tokens).
      '.cm-lintRange': { backgroundImage: 'none', textDecorationLine: 'underline', textDecorationStyle: 'wavy', textUnderlineOffset: '3px', textDecorationSkipInk: 'none' },
      '.cm-lintRange-error': { textDecorationColor: 'var(--err)' },
      '.cm-lintRange-warning': { textDecorationColor: 'var(--warn)' },
      '.cm-lintRange-info': { textDecorationColor: 'var(--info)' },
      '.cm-lintPoint:after': { borderBottomColor: 'var(--err)' },
      '.cm-gutter-lint': { width: '14px' },
      '.cm-lint-marker': { width: '10px', height: '10px' },
    }),
    tooltipTheme,
  ]
}

/** Tooltip and hover-card styles, shared by every editor view that
 *  shows tooltips (this one, the Helm values view). */
export const tooltipTheme = EditorView.theme({
  '.cm-tooltip': {
    backgroundColor: 'var(--bg-2)',
    color: 'var(--fg-0)',
    border: 'none',
    borderRadius: 'var(--r-md)',
    boxShadow: 'var(--shadow-pop)',
    fontFamily: 'var(--font-ui)',
    fontSize: 'var(--fs-sm)',
  },
  '.cm-tooltip-lint': { padding: 0 },
  '.cm-diagnostic': { padding: 'var(--s-2) var(--s-3)', borderLeftWidth: '3px' },
  '.cm-diagnostic-error': { borderLeftColor: 'var(--err)' },
  '.cm-diagnostic-warning': { borderLeftColor: 'var(--warn)' },
  '.cm-diagnostic-info': { borderLeftColor: 'var(--info)' },
  '.cm-diag-hint': { marginTop: 'var(--s-1)', color: 'var(--fg-1)' },
  '.cm-diagnosticSource': { color: 'var(--fg-2)', fontFamily: 'var(--font-mono)', fontSize: 'var(--fs-xs)' },

  '.cm-hover': { maxWidth: '480px', padding: 'var(--s-2) var(--s-3)' },
  '.cm-hover-title': { fontFamily: 'var(--font-mono)', color: 'var(--syn-key)', marginBottom: 'var(--s-1)', wordBreak: 'break-all' },
  '.cm-hover dl': { display: 'grid', gridTemplateColumns: 'auto 1fr', gap: '2px var(--s-3)', margin: 0 },
  '.cm-hover dt': { color: 'var(--fg-2)' },
  '.cm-hover dd': { margin: 0 },
  '.cm-hover pre': {
    margin: 'var(--s-2) 0 0',
    padding: 'var(--s-2)',
    fontFamily: 'var(--font-mono)',
    fontSize: 'var(--fs-xs)',
    background: 'var(--bg-0)',
    borderRadius: 'var(--r-sm)',
    whiteSpace: 'pre-wrap',
    maxHeight: '220px',
    overflow: 'auto',
  },

  '.cm-tooltip-autocomplete > ul': { fontFamily: 'var(--font-mono)', fontSize: 'var(--fs-sm)' },
  '.cm-tooltip-autocomplete > ul > li[aria-selected]': { backgroundColor: 'var(--accent-soft)', color: 'var(--fg-0)' },
  '.cm-completionDetail': { color: 'var(--fg-2)', fontStyle: 'normal', marginLeft: 'var(--s-2)' },
})
