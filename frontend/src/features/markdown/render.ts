// Markdown to safe HTML (decision 84). The text is untrusted: a README
// in a cloned repository can hold anything. marked parses GitHub-flavoured
// Markdown (tables, task lists, strikethrough) with raw HTML turned into
// plain text, and DOMPurify then strips whatever is left that could run:
// scripts, event handlers, javascript: links, frames, forms, styles.
// Images never load from here: their address moves to data-src, and the
// preview decides what to show (remote images stay blocked).
import { Marked, type Tokens } from 'marked'
import DOMPurify, { type WindowLike } from 'dompurify'

const escapes: Record<string, string> = { '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }
const escape = (s: string) => s.replace(/[&<>"']/g, (c) => escapes[c])

const marked = new Marked({
  gfm: true,
  async: false,
  renderer: {
    // Raw HTML shows as the text it is, block or inline.
    html: ({ text, block }: Tokens.HTML | Tokens.Tag) => (block ? `<p>${escape(text.trim())}</p>\n` : escape(text)),
  },
})

const purifiers = new WeakMap<WindowLike, ReturnType<typeof DOMPurify>>()

function purifier(win: WindowLike) {
  const p = DOMPurify(win)
  p.addHook('afterSanitizeAttributes', harden)
  return p
}

function harden(node: Element) {
  if (node.tagName === 'IMG') {
    const src = node.getAttribute('src')
    node.removeAttribute('src')
    node.removeAttribute('srcset')
    if (src) node.setAttribute('data-src', src)
  }
  // Task-list boxes are the only inputs marked makes; keep them inert.
  if (node.tagName === 'INPUT') {
    if (node.getAttribute('type') !== 'checkbox') node.remove()
    else node.setAttribute('disabled', '')
  }
  if (node.tagName === 'A' && node.hasAttribute('href')) node.setAttribute('rel', 'noopener noreferrer')
}

const config = {
  USE_PROFILES: { html: true },
  FORBID_TAGS: ['style', 'form', 'button', 'select', 'textarea', 'iframe', 'object', 'embed', 'base', 'link', 'meta'],
  FORBID_ATTR: ['style', 'srcset', 'formaction', 'target'],
}

/**
 * renderMarkdown turns Markdown into HTML that is safe to put in the page.
 * win is the window DOMPurify works in; tests pass their own.
 */
export function renderMarkdown(src: string, win: WindowLike = globalThis.window): string {
  let p = purifiers.get(win)
  if (!p) purifiers.set(win, (p = purifier(win)))
  return p.sanitize(marked.parse(src) as string, config) as string
}
