// Syntax colours, one list for every place that highlights code: the
// editor (CodeView's HighlightStyle) and static blocks such as Markdown
// code (highlightTo, whose syn-* classes the page that shows them styles).
import { HighlightStyle, type Language } from '@codemirror/language'
import { json } from '@codemirror/lang-json'
import { yaml } from '@codemirror/lang-yaml'
import { highlightCode, tagHighlighter, tags as t, type Tag } from '@lezer/highlight'
import { codeLang, loadedLanguage, loadLanguage } from './languages'

interface Rule {
  tag: Tag | Tag[]
  /** A colour token in tokens.css, also the class static blocks get. */
  token: string
  italic?: boolean
}

// The first rules colour code (3.9.0 T7): names that are called or
// defined, built-ins, classes; they are more specific than the YAML and
// JSON rules after them, which they would otherwise take.
const rules: Rule[] = [
  { tag: [t.function(t.variableName), t.function(t.propertyName), t.definition(t.variableName), t.definition(t.function(t.variableName))], token: 'syn-func' },
  { tag: [t.standard(t.variableName), t.special(t.variableName), t.self], token: 'syn-anchor' },
  { tag: [t.className, t.namespace, t.definition(t.typeName)], token: 'syn-tag' },
  { tag: [t.modifier, t.operatorKeyword], token: 'syn-bool' },
  { tag: [t.attributeName], token: 'syn-key' },
  { tag: [t.regexp, t.inserted], token: 'syn-string' },
  { tag: [t.escape, t.character], token: 'syn-number' },
  { tag: [t.operator], token: 'syn-punct' },
  { tag: [t.deleted], token: 'err' },
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
    return { yaml: y, yml: y, json: j, jsonc: j, json5: j }
  })()
  const code = codeLang(name)
  return langs[name.toLowerCase()] ?? (code && loadedLanguage(code))
}

/** highlightToAsync is highlightTo for any language codec has, loading
 *  it first when needed; false for a language it doesn't have. */
export async function highlightToAsync(el: Element, code: string, language: string): Promise<boolean> {
  const c = codeLang(language)
  if (c) await loadLanguage(c)
  return highlightTo(el, code, language)
}

/** A piece of a highlighted line: its text and its syn-* class, if any. */
export interface Piece {
  text: string
  cls?: string
}

/**
 * highlightLines cuts code into lines of highlighted pieces, for views
 * that draw lines themselves (the Log viewer's source peek); null when
 * the language isn't loaded or codec doesn't have it.
 */
export function highlightLines(code: string, language: string): Piece[][] | null {
  const lang = languageOf(language)
  if (!lang) return null
  const lines: Piece[][] = [[]]
  highlightCode(
    code,
    lang.parser.parse(code),
    classes,
    (text, cls) => lines[lines.length - 1].push(cls ? { text, cls } : { text }),
    () => lines.push([]),
  )
  return lines
}

/**
 * highlightTo fills el with code highlighted as language (yaml, json,
 * and the code languages of languages.ts once loaded), built from text
 * nodes and spans only. Returns false, leaving el alone, for any other
 * language.
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
