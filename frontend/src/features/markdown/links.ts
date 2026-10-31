// Where a link or image in a Markdown file points. Relative addresses
// resolve against the file's own folder, a leading / against the open
// folder (as GitHub does for a repository); nothing may climb above it.

export type Target =
  | { kind: 'anchor'; id: string } // a heading in this file
  | { kind: 'file'; path: string; id: string } // a workspace path, maybe with #heading
  | { kind: 'web'; url: string } // http, https or mailto
  | { kind: 'data'; url: string } // an inline data: image
  | { kind: 'none' } // anything else: other schemes, outside the folder

/** linkTarget resolves href, found in the workspace file from. */
export function linkTarget(href: string, from: string): Target {
  href = href.trim()
  if (!href) return { kind: 'none' }
  if (href.startsWith('#')) return { kind: 'anchor', id: decode(href.slice(1)) }
  if (/^(https?:|mailto:)/i.test(href)) return { kind: 'web', url: href }
  if (href.startsWith('//')) return { kind: 'web', url: 'https:' + href }
  if (/^data:image\//i.test(href)) return { kind: 'data', url: href }
  if (/^[a-z][a-z0-9+.-]*:/i.test(href)) return { kind: 'none' }

  const hashAt = href.indexOf('#')
  const id = hashAt < 0 ? '' : decode(href.slice(hashAt + 1))
  let rel = (hashAt < 0 ? href : href.slice(0, hashAt)).replace(/\?.*$/, '')
  rel = decode(rel).replace(/\\/g, '/')
  const parts = rel.startsWith('/') ? [] : from.split('/').slice(0, -1)
  for (const seg of rel.split('/')) {
    if (seg === '' || seg === '.') continue
    if (seg === '..') {
      if (!parts.length) return { kind: 'none' }
      parts.pop()
    } else parts.push(seg)
  }
  const path = parts.join('/')
  if (!path) return { kind: 'none' }
  if (path === from) return id ? { kind: 'anchor', id } : { kind: 'none' }
  return { kind: 'file', path, id }
}

function decode(s: string): string {
  try {
    return decodeURIComponent(s)
  } catch {
    return s
  }
}

/** Cases for linkTarget, run by check() in render.cases.ts. */
export const linkCases: [href: string, from: string, want: Target][] = [
  ['#setup', 'README.md', { kind: 'anchor', id: 'setup' }],
  ['docs/guide.md', 'README.md', { kind: 'file', path: 'docs/guide.md', id: '' }],
  ['./guide.md#install', 'docs/intro.md', { kind: 'file', path: 'docs/guide.md', id: 'install' }],
  ['../README.md', 'docs/intro.md', { kind: 'file', path: 'README.md', id: '' }],
  ['/charts/app/values.yaml', 'docs/deep/x.md', { kind: 'file', path: 'charts/app/values.yaml', id: '' }],
  ['my%20notes.md?plain=1', 'README.md', { kind: 'file', path: 'my notes.md', id: '' }],
  ['intro.md#top', 'docs/intro.md', { kind: 'anchor', id: 'top' }],
  ['../../etc/passwd', 'docs/intro.md', { kind: 'none' }],
  ['..', 'README.md', { kind: 'none' }],
  ['https://example.com/a', 'README.md', { kind: 'web', url: 'https://example.com/a' }],
  ['//cdn.example.com/x.png', 'README.md', { kind: 'web', url: 'https://cdn.example.com/x.png' }],
  ['mailto:a@example.com', 'README.md', { kind: 'web', url: 'mailto:a@example.com' }],
  ['data:image/png;base64,AAAA', 'README.md', { kind: 'data', url: 'data:image/png;base64,AAAA' }],
  ['file:///C:/x.md', 'README.md', { kind: 'none' }],
  ['vscode://file/x', 'README.md', { kind: 'none' }],
  ['C:\\Windows\\x.md', 'README.md', { kind: 'none' }],
  ['docs\\guide.md', 'README.md', { kind: 'file', path: 'docs/guide.md', id: '' }],
]
