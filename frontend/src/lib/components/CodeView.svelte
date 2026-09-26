<script module lang="ts">
  export type Language = 'json' | 'yaml' | 'text'
</script>

<script lang="ts">
  import { untrack } from 'svelte'
  import { EditorState, Compartment, type Extension } from '@codemirror/state'
  import {
    EditorView,
    keymap,
    lineNumbers,
    highlightActiveLine,
    highlightActiveLineGutter,
    highlightSpecialChars,
    drawSelection,
    placeholder as placeholderExt,
  } from '@codemirror/view'
  import { defaultKeymap, history, historyKeymap } from '@codemirror/commands'
  import { searchKeymap, highlightSelectionMatches } from '@codemirror/search'
  import { syntaxHighlighting, bracketMatching, foldGutter, foldKeymap, HighlightStyle } from '@codemirror/language'
  import { json } from '@codemirror/lang-json'
  import { yaml } from '@codemirror/lang-yaml'
  import { tags as t } from '@lezer/highlight'

  // CodeMirror 6 wrapper used for every code/text surface. Phase 04
  // extends it with engine-driven diagnostics and overlays.
  interface Props {
    value: string
    label: string
    language?: Language
    readonly?: boolean
    placeholder?: string
    wrap?: boolean
    onpaste?: () => void
    /** Extra CodeMirror extensions (e.g. YAML intelligence). */
    extensions?: Extension
    /** Debounce (ms) for copying edits back into `value`. Big documents
     *  use it so a keystroke never copies the whole text. 0 = at once. */
    syncDelay?: number
  }

  let {
    value = $bindable(''),
    label,
    language = 'text',
    readonly = false,
    placeholder = '',
    wrap = false,
    onpaste,
    extensions = [],
    syncDelay = 0,
  }: Props = $props()

  let host = $state<HTMLDivElement>()
  let view: EditorView | undefined
  let syncTimer: ReturnType<typeof setTimeout> | undefined

  const langConf = new Compartment()
  const readonlyConf = new Compartment()
  const wrapConf = new Compartment()
  const placeholderConf = new Compartment()
  const extraConf = new Compartment()

  // Colors come from the syntax tokens in tokens.css.
  const highlight = HighlightStyle.define([
    { tag: [t.propertyName, t.definition(t.propertyName)], color: 'var(--syn-key)' },
    { tag: [t.string, t.special(t.string)], color: 'var(--syn-string)' },
    { tag: t.number, color: 'var(--syn-number)' },
    { tag: [t.bool, t.null, t.atom, t.keyword], color: 'var(--syn-bool)' },
    { tag: [t.comment, t.lineComment, t.blockComment], color: 'var(--syn-comment)', fontStyle: 'italic' },
    { tag: t.labelName, color: 'var(--syn-anchor)' },
    { tag: [t.typeName, t.tagName], color: 'var(--syn-tag)' },
    { tag: [t.punctuation, t.separator, t.brace, t.squareBracket, t.bracket], color: 'var(--syn-punct)' },
    { tag: [t.meta, t.processingInstruction], color: 'var(--accent)' },
    { tag: t.invalid, color: 'var(--err)' },
  ])

  const theme = EditorView.theme(
    {
      '&': { height: '100%', fontSize: 'var(--fs-md)', color: 'var(--fg-0)', backgroundColor: 'var(--bg-2)' },
      '.cm-scroller': { fontFamily: 'var(--font-mono)', lineHeight: 'var(--lh-code)' },
      '.cm-content': { padding: 'var(--s-2) 0', caretColor: 'var(--accent)' },
      '.cm-line': { padding: '0 var(--s-3) 0 var(--s-2)' },
      '.cm-gutters': { backgroundColor: 'var(--bg-2)', color: 'var(--fg-2)', border: 'none' },
      '.cm-lineNumbers .cm-gutterElement': { padding: '0 var(--s-1) 0 var(--s-3)', minWidth: '32px' },
      '.cm-activeLine': { backgroundColor: 'var(--editor-active-line)' },
      '.cm-activeLineGutter': { backgroundColor: 'transparent', color: 'var(--fg-1)' },
      '&.cm-focused .cm-cursor': { borderLeftColor: 'var(--accent)', borderLeftWidth: '2px' },
      '.cm-selectionBackground': { backgroundColor: 'var(--editor-selection)' },
      '&.cm-focused > .cm-scroller > .cm-selectionLayer .cm-selectionBackground': {
        backgroundColor: 'var(--editor-selection-focused)',
      },
      '.cm-selectionMatch': { backgroundColor: 'var(--editor-selection-match)' },
      '.cm-matchingBracket, &.cm-focused .cm-matchingBracket': {
        backgroundColor: 'var(--accent-soft)',
        outline: '1px solid var(--accent)',
      },
      '.cm-placeholder': { color: 'var(--fg-2)', fontFamily: 'var(--font-ui)' },
      '.cm-foldGutter .cm-gutterElement': { color: 'var(--fg-2)', padding: '0 var(--s-1)' },
      '.cm-foldPlaceholder': { backgroundColor: 'var(--bg-3)', border: 'none', color: 'var(--fg-1)' },
      '.cm-panels': { backgroundColor: 'var(--bg-1)', color: 'var(--fg-0)', borderColor: 'var(--border)' },
      '.cm-panels.cm-panels-bottom': { borderTop: '1px solid var(--border)' },
      '.cm-searchMatch': { backgroundColor: 'var(--editor-search-match)' },
      '.cm-searchMatch-selected': { backgroundColor: 'var(--editor-search-match-current)' },
      '.cm-textfield': {
        backgroundColor: 'var(--bg-0)',
        border: '1px solid var(--border-strong)',
        borderRadius: 'var(--r-sm)',
        color: 'var(--fg-0)',
      },
      '.cm-button': {
        backgroundImage: 'none',
        backgroundColor: 'var(--bg-3)',
        border: '1px solid var(--border-strong)',
        borderRadius: 'var(--r-sm)',
        color: 'var(--fg-0)',
      },
    },
    { dark: true },
  )

  // Mod-Enter is the app-wide "run" shortcut, so drop CodeMirror's
  // default binding (insert blank line) and let the key reach the app.
  const editorKeymap = defaultKeymap.filter((b) => b.key !== 'Mod-Enter')

  function langExt(l: Language): Extension {
    if (l === 'json') return json()
    if (l === 'yaml') return yaml()
    return []
  }

  function readonlyExt(ro: boolean): Extension {
    return [EditorState.readOnly.of(ro), EditorView.editable.of(!ro)]
  }

  $effect(() => {
    if (!host) return
    // Read the props untracked: this effect must run once per mount,
    // not every time the text or an option changes (the effects below
    // handle those updates without rebuilding the editor).
    const init = untrack(() => ({ value, language, readonly, wrap, placeholder, label, extensions }))
    view = new EditorView({
      parent: host,
      state: EditorState.create({
        doc: init.value,
        extensions: [
          lineNumbers(),
          highlightActiveLineGutter(),
          highlightSpecialChars(),
          foldGutter(),
          drawSelection(),
          history(),
          bracketMatching(),
          highlightActiveLine(),
          highlightSelectionMatches(),
          syntaxHighlighting(highlight),
          theme,
          keymap.of([...editorKeymap, ...historyKeymap, ...searchKeymap, ...foldKeymap]),
          langConf.of(langExt(init.language)),
          readonlyConf.of(readonlyExt(init.readonly)),
          wrapConf.of(init.wrap ? EditorView.lineWrapping : []),
          placeholderConf.of(init.placeholder ? placeholderExt(init.placeholder) : []),
          extraConf.of(init.extensions),
          EditorView.contentAttributes.of({ 'aria-label': init.label }),
          EditorView.domEventHandlers({
            paste: () => {
              // The document updates after the event; run on the next tick.
              if (onpaste) setTimeout(onpaste, 0)
              return false
            },
          }),
          EditorView.updateListener.of((u) => {
            if (!u.docChanged) return
            if (syncDelay <= 0) {
              value = u.state.doc.toString()
              return
            }
            clearTimeout(syncTimer)
            syncTimer = setTimeout(() => {
              if (view) value = view.state.doc.toString()
            }, syncDelay)
          }),
        ],
      }),
    })
    return () => {
      clearTimeout(syncTimer)
      view?.destroy()
      view = undefined
    }
  })

  // Push outside changes (clear, swap, samples, a file reloaded from
  // disk) into the editor. Only the differing middle is replaced, so
  // the selection, scroll position and folds outside it survive.
  $effect(() => {
    const next = value
    if (!view) return
    const cur = view.state.doc.toString()
    if (next === cur) return
    let from = 0
    const max = Math.min(next.length, cur.length)
    while (from < max && next.charCodeAt(from) === cur.charCodeAt(from)) from++
    let toCur = cur.length
    let toNext = next.length
    while (toCur > from && toNext > from && cur.charCodeAt(toCur - 1) === next.charCodeAt(toNext - 1)) {
      toCur--
      toNext--
    }
    view.dispatch({ changes: { from, to: toCur, insert: next.slice(from, toNext) } })
  })

  $effect(() => {
    view?.dispatch({ effects: langConf.reconfigure(langExt(language)) })
  })
  $effect(() => {
    view?.dispatch({ effects: readonlyConf.reconfigure(readonlyExt(readonly)) })
  })
  $effect(() => {
    view?.dispatch({ effects: wrapConf.reconfigure(wrap ? EditorView.lineWrapping : []) })
  })
  $effect(() => {
    view?.dispatch({ effects: placeholderConf.reconfigure(placeholder ? placeholderExt(placeholder) : []) })
  })

  $effect(() => {
    const ext = extensions
    view?.dispatch({ effects: extraConf.reconfigure(ext) })
  })

  /** The underlying editor, for callers that add their own behaviour. */
  export function getView(): EditorView | undefined {
    return view
  }

  export function focus() {
    view?.focus()
  }

  /** Select the character at a 1-based line/column and scroll to it. */
  export function selectPosition(line: number, column: number) {
    if (!view) return
    const doc = view.state.doc
    const ln = doc.line(Math.min(Math.max(line, 1), doc.lines))
    const from = Math.min(ln.from + Math.max(column - 1, 0), ln.to)
    const to = Math.min(from + 1, ln.to)
    view.dispatch({ selection: { anchor: from, head: to }, scrollIntoView: true })
    view.focus()
  }
</script>

<div class="codeview" bind:this={host}></div>

<style>
  .codeview {
    flex: 1;
    min-height: 0;
    overflow: hidden;
  }
</style>
