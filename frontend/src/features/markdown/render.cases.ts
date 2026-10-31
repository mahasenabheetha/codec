// Cases for renderMarkdown: hostile Markdown that must come out inert, and
// ordinary Markdown that must still render. check() needs only a DOM, so
// it runs in a browser console (in dev: await import the two modules and
// call check(renderMarkdown, document)) or in Node with a DOM library.

import { linkCases, linkTarget } from './links'

export interface Case {
  name: string
  md: string
  /** Each must appear in the HTML. */
  has?: (string | RegExp)[]
  /** None may appear in the HTML. */
  not?: (string | RegExp)[]
}

const badTags = new Set(['SCRIPT', 'IFRAME', 'FRAME', 'OBJECT', 'EMBED', 'STYLE', 'FORM', 'BUTTON', 'TEXTAREA', 'SELECT', 'BASE', 'LINK', 'META', 'SVG', 'MATH', 'DETAILS'])
const badUrl = /^\s*(javascript|vbscript|data):/i

/**
 * problems lists what in the rendered tree could run code or load from
 * elsewhere; every case must come out with none. Checked on elements, not
 * on the HTML text, where escaped raw HTML is harmless.
 */
export function problems(root: Element): string[] {
  const out: string[] = []
  for (const el of root.querySelectorAll('*')) {
    const tag = el.tagName.toUpperCase()
    if (badTags.has(tag)) out.push(`<${tag.toLowerCase()}>`)
    if (tag === 'INPUT' && (el.getAttribute('type') !== 'checkbox' || !el.hasAttribute('disabled'))) out.push('live input')
    for (const { name, value } of el.attributes) {
      if (/^on/i.test(name) || ['style', 'src', 'srcset', 'target', 'formaction'].includes(name)) out.push(`${tag.toLowerCase()}[${name}]`)
      if (['href', 'data-src', 'xlink:href', 'action'].includes(name) && badUrl.test(value)) out.push(`${name}=${value}`)
    }
  }
  return out
}

/**
 * check runs every case through render and returns the failures, empty
 * when all pass. doc supplies the DOM that problems() reads.
 */
export function check(render: (md: string) => string, doc: Document): string[] {
  const fails: string[] = []
  // An inert document: nothing in it loads or runs, even if a case fails.
  const scratch = doc.implementation.createHTMLDocument('')
  for (const c of cases) {
    const html = render(c.md)
    const root = scratch.createElement('div')
    root.innerHTML = html
    const found = (p: string | RegExp) => (typeof p === 'string' ? html.includes(p) : p.test(html))
    const bad = [
      ...problems(root),
      ...(c.has ?? []).filter((p) => !found(p)).map((p) => `missing ${p}`),
      ...(c.not ?? []).filter(found).map((p) => `has ${p}`),
    ]
    if (bad.length) fails.push(`${c.name}: ${bad.join(', ')}\n  ${html}`)
  }
  for (const [href, from, want] of linkCases) {
    const got = JSON.stringify(linkTarget(href, from))
    if (got !== JSON.stringify(want)) fails.push(`link ${href} from ${from}: ${got}, want ${JSON.stringify(want)}`)
  }
  return fails
}

export const cases: Case[] = [
  // Raw HTML shows as text.
  { name: 'script block', md: '<script>alert(1)</script>', has: ['&lt;script&gt;'] },
  { name: 'inline script', md: 'hi <script>alert(1)</script> there', has: ['&lt;script&gt;'] },
  { name: 'img onerror', md: '<img src=x onerror=alert(1)>', has: ['&lt;img'] },
  { name: 'svg onload', md: '<svg onload=alert(1)>', not: [/<svg/i] },
  { name: 'iframe', md: '<iframe src="https://example.com"></iframe>' },
  { name: 'style tag', md: '<style>body{display:none}</style>' },
  { name: 'details', md: '<details open ontoggle=alert(1)>x</details>', not: [/<details/i] },
  { name: 'html comment', md: '<!-- <script>alert(1)</script> -->' },
  { name: 'mixed case', md: '<ScRiPt>alert(1)</sCrIpT>' },
  // Links and images written in Markdown.
  { name: 'javascript link', md: '[x](javascript:alert(1))' },
  { name: 'encoded javascript link', md: '[x](jav&#x61;script:alert(1))' },
  { name: 'spaced javascript link', md: '[x]( javascript:alert(1) )' },
  { name: 'vbscript link', md: '[x](vbscript:msgbox(1))' },
  { name: 'data html link', md: '[x](data:text/html;base64,PHNjcmlwdD5hbGVydCgxKTwvc2NyaXB0Pg==)', not: [/href="data:/i] },
  { name: 'autolink javascript', md: '<javascript:alert(1)>' },
  { name: 'javascript image', md: '![x](javascript:alert(1))' },
  { name: 'title breakout', md: '[x](https://a.example "t\\" onmouseover=\\"alert(1)")' },
  { name: 'reference link', md: '[x][r]\n\n[r]: javascript:alert(1)' },
  // Images never load from the preview itself.
  { name: 'remote image', md: '![logo](https://example.com/logo.png)', has: ['data-src="https://example.com/logo.png"'] },
  { name: 'local image', md: '![d](docs/diagram.png)', has: ['data-src="docs/diagram.png"'] },
  // What should still work.
  { name: 'heading and emphasis', md: '# Title\n\nSome *em* and **strong** and `code`.', has: ['<h1 id="user-content-title">Title<a class="anchor" href="#title"', '<em>em</em>', '<strong>strong</strong>', '<code>code</code>'] },
  { name: 'heading ids', md: '## Getting Started!\n\n## Getting Started\n\n### `npm` & *you*', has: ['id="user-content-getting-started"', 'id="user-content-getting-started-1"', 'id="user-content-npm--you"', 'href="#getting-started-1"'] },
  { name: 'heading id cannot clobber', md: '# location\n\n# cookie', has: ['id="user-content-location"', 'id="user-content-cookie"'], not: [/id="location"/, /id="cookie"/] },
  { name: 'heading with raw html', md: '# Hi <img src=x onerror=alert(1)>', has: ['&lt;img'] },
  { name: 'link', md: '[docs](https://example.com/docs)', has: ['href="https://example.com/docs"', 'rel="noopener noreferrer"'], not: [/target=/i] },
  { name: 'relative link', md: '[next](other.md#setup)', has: ['href="other.md#setup"'] },
  { name: 'table', md: '| a | b |\n|---|---|\n| 1 | 2 |', has: ['<table>', '<th>a</th>', '<td>2</td>'] },
  { name: 'task list', md: '- [x] done\n- [ ] todo', has: [/<input[^>]*checked[^>]*>/, /<input[^>]*disabled/] },
  { name: 'strikethrough', md: '~~gone~~', has: ['<del>gone</del>'] },
  { name: 'code block keeps text', md: '```html\n<script>alert(1)</script>\n```', has: ['&lt;script&gt;', 'language-html'] },
  { name: 'quote and list', md: '> quoted\n\n1. one\n2. two', has: ['<blockquote>', '<ol>', '<li>two</li>'] },
]
