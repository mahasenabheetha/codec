// Syntax colours, one list for every place that highlights code: the
// editor (CodeView's HighlightStyle) and static blocks such as Markdown
// code (highlightTo, whose syn-* classes the page that shows them styles).
import { HighlightStyle, type Language, StreamLanguage } from '@codemirror/language'
import { json } from '@codemirror/lang-json'
import { yaml } from '@codemirror/lang-yaml'
import { shell } from '@codemirror/legacy-modes/mode/shell'
import { highlightCode, tagHighlighter, tags as t, type Tag } from '@lezer/highlight'

interface Rule {
  tag: Tag | Tag[]
  /** A colour token in tokens.css, also the class static blocks get. */
  token: string
  italic?: boolean
}

const rules: Rule[] = [
  { tag: [t.propertyName, t.definition(t.propertyName)], token: 'syn-key' },
  { tag: [t.string, t.special(t.string)], token: 'syn-string' },
  { tag: t.number, token: 'syn-number' },
  { tag: [t.bool, t.null, t.atom, t.keyword], token: 'syn-bool' },
  { tag: [t.comment, t.lineComment, t.blockComment], token: 'syn-comment', italic: true },
  { tag: t.labelName, token: 'syn-anchor' },
  { tag: [t.typeName, t.tagName], token: 'syn-tag' },
  { tag: [t.punctuation, t.separator, t.brace, t.squareBracket, t.bracket], token: 'syn-punct' },
  { tag: [t.meta, t.processingInstruction], token: 'accent' },
  { tag: t.invalid, token: 'err' },
]

/** The editor's highlight style. Colours come from the tokens. */
export const highlightStyle = HighlightStyle.define(
  rules.map((r) => ({ tag: r.tag, color: `var(--${r.token})`, ...(r.italic ? { fontStyle: 'italic' } : {}) })),
)

/** Classes for static highlighting: syn-key, syn-string, …, syn-accent, syn-err. */
const classes = tagHighlighter(rules.map((r) => ({ tag: r.tag, class: r.token.startsWith('syn-') ? r.token : `syn-${r.token}` })))

let langs: Record<string, Language> | undefined
function languageOf(name: string): Language | undefined {
  langs ??= (() => {
    const y = yaml().language
    const j = json().language
    const sh = StreamLanguage.define(shell)
    return { yaml: y, yml: y, json: j, jsonc: j, sh: sh, shell: sh, bash: sh, zsh: sh, console: sh }
  })()
  return langs[name.toLowerCase()]
}

/**
 * highlightTo fills el with code highlighted as language (yaml, json or
 * shell and their aliases), built from text nodes and spans only.
 * Returns false, leaving el alone, for any other language.
 */
export function highlightTo(el: Element, code: string, language: string): boolean {
  const lang = languageOf(language)
  if (!lang) return false
  const doc = el.ownerDocument
  const out = doc.createDocumentFragment()
  highlightCode(
    code,
    lang.parser.parse(code),
    classes,
    (text, cls) => {
      if (!cls) return out.append(text)
      const span = doc.createElement('span')
      span.className = cls
      span.textContent = text
      out.append(span)
    },
    () => out.append('\n'),
  )
  el.replaceChildren(out)
  return true
}
