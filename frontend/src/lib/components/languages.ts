// Code languages for highlighting (3.9.0 T7): CodeMirror's legacy modes,
// line tokenisers that come with @codemirror/legacy-modes, each in a
// chunk of its own loaded the first time a file needs it. Files are
// read-only in codec, so colours are what matters, not a full parser;
// YAML and JSON keep their own parsers (CodeView, syntax.ts).
import { foldService, StreamLanguage, type Language, type StreamParser } from '@codemirror/language'

type Loader = () => Promise<StreamParser<unknown>>

// A language's name and how to load its mode. Names are what a file's
// extension and a Markdown fence map to.
const loaders: Record<string, Loader> = {
  go: () => import('@codemirror/legacy-modes/mode/go').then((m) => m.go),
  java: () => import('@codemirror/legacy-modes/mode/clike').then((m) => m.java),
  kotlin: () => import('@codemirror/legacy-modes/mode/clike').then((m) => m.kotlin),
  scala: () => import('@codemirror/legacy-modes/mode/clike').then((m) => m.scala),
  csharp: () => import('@codemirror/legacy-modes/mode/clike').then((m) => m.csharp),
  c: () => import('@codemirror/legacy-modes/mode/clike').then((m) => m.c),
  cpp: () => import('@codemirror/legacy-modes/mode/clike').then((m) => m.cpp),
  dart: () => import('@codemirror/legacy-modes/mode/clike').then((m) => m.dart),
  objc: () => import('@codemirror/legacy-modes/mode/clike').then((m) => m.objectiveC),
  javascript: () => import('@codemirror/legacy-modes/mode/javascript').then((m) => m.javascript),
  typescript: () => import('@codemirror/legacy-modes/mode/javascript').then((m) => m.typescript),
  python: () => import('@codemirror/legacy-modes/mode/python').then((m) => m.python),
  ruby: () => import('@codemirror/legacy-modes/mode/ruby').then((m) => m.ruby),
  rust: () => import('@codemirror/legacy-modes/mode/rust').then((m) => m.rust),
  swift: () => import('@codemirror/legacy-modes/mode/swift').then((m) => m.swift),
  groovy: () => import('@codemirror/legacy-modes/mode/groovy').then((m) => m.groovy),
  lua: () => import('@codemirror/legacy-modes/mode/lua').then((m) => m.lua),
  perl: () => import('@codemirror/legacy-modes/mode/perl').then((m) => m.perl),
  r: () => import('@codemirror/legacy-modes/mode/r').then((m) => m.r),
  haskell: () => import('@codemirror/legacy-modes/mode/haskell').then((m) => m.haskell),
  erlang: () => import('@codemirror/legacy-modes/mode/erlang').then((m) => m.erlang),
  elm: () => import('@codemirror/legacy-modes/mode/elm').then((m) => m.elm),
  julia: () => import('@codemirror/legacy-modes/mode/julia').then((m) => m.julia),
  clojure: () => import('@codemirror/legacy-modes/mode/clojure').then((m) => m.clojure),
  vb: () => import('@codemirror/legacy-modes/mode/vb').then((m) => m.vb),
  shell: () => import('@codemirror/legacy-modes/mode/shell').then((m) => m.shell),
  powershell: () => import('@codemirror/legacy-modes/mode/powershell').then((m) => m.powerShell),
  sql: () => import('@codemirror/legacy-modes/mode/sql').then((m) => m.standardSQL),
  xml: () => import('@codemirror/legacy-modes/mode/xml').then((m) => m.xml),
  html: () => import('@codemirror/legacy-modes/mode/xml').then((m) => m.html),
  css: () => import('@codemirror/legacy-modes/mode/css').then((m) => m.css),
  scss: () => import('@codemirror/legacy-modes/mode/css').then((m) => m.sCSS),
  less: () => import('@codemirror/legacy-modes/mode/css').then((m) => m.less),
  toml: () => import('@codemirror/legacy-modes/mode/toml').then((m) => m.toml),
  ini: () => import('@codemirror/legacy-modes/mode/properties').then((m) => m.properties),
  dockerfile: () => import('@codemirror/legacy-modes/mode/dockerfile').then((m) => m.dockerFile),
  nginx: () => import('@codemirror/legacy-modes/mode/nginx').then((m) => m.nginx),
  cmake: () => import('@codemirror/legacy-modes/mode/cmake').then((m) => m.cmake),
  protobuf: () => import('@codemirror/legacy-modes/mode/protobuf').then((m) => m.protobuf),
  diff: () => import('@codemirror/legacy-modes/mode/diff').then((m) => m.diff),
  jinja: () => import('@codemirror/legacy-modes/mode/jinja2').then((m) => m.jinja2),
  hcl: async () => hcl,
}

export type CodeLang = keyof typeof loaders

// Markdown fences and other names for the languages above.
const aliases: Record<string, CodeLang> = {
  golang: 'go', js: 'javascript', jsx: 'javascript', mjs: 'javascript', cjs: 'javascript', node: 'javascript',
  ts: 'typescript', tsx: 'typescript', py: 'python', python3: 'python', rb: 'ruby', rs: 'rust', kt: 'kotlin',
  kts: 'kotlin', cs: 'csharp', 'c#': 'csharp', 'c++': 'cpp', cc: 'cpp', cxx: 'cpp', h: 'c', hpp: 'cpp',
  sh: 'shell', bash: 'shell', zsh: 'shell', console: 'shell', ps1: 'powershell', pwsh: 'powershell',
  postgres: 'sql', postgresql: 'sql', mysql: 'sql', plsql: 'sql', svg: 'xml', xhtml: 'html', htm: 'html',
  properties: 'ini', conf: 'ini', cfg: 'ini', env: 'ini', docker: 'dockerfile', containerfile: 'dockerfile',
  terraform: 'hcl', tf: 'hcl', tfvars: 'hcl', proto: 'protobuf', patch: 'diff', j2: 'jinja', jinja2: 'jinja',
  gradle: 'groovy', jenkinsfile: 'groovy', clj: 'clojure', erl: 'erlang', jl: 'julia', hs: 'haskell', pl: 'perl',
}

/** The language a name (a fence, a language id) means, if codec has it. */
export function codeLang(name: string): CodeLang | undefined {
  const n = name.toLowerCase()
  return n in loaders ? (n as CodeLang) : aliases[n]
}

// Extensions that differ from a language name or alias.
const byExt: Record<string, CodeLang> = {
  java: 'java', scala: 'scala', sc: 'scala', dart: 'dart', m: 'objc', mm: 'objc', c: 'c', cpp: 'cpp',
  vue: 'html', svelte: 'html', xaml: 'xml', csproj: 'xml', fsproj: 'xml', props: 'xml', targets: 'xml',
  resx: 'xml', plist: 'xml', pom: 'xml', bat: 'shell', psm1: 'powershell', psd1: 'powershell', lua: 'lua',
  sql: 'sql', css: 'css', scss: 'scss', less: 'less', swift: 'swift', r: 'r', groovy: 'groovy', cmake: 'cmake',
  hcl: 'hcl', nomad: 'hcl', toml: 'toml', ini: 'ini', vb: 'vb', bas: 'vb', go: 'go', elm: 'elm', ex: 'ruby',
  exs: 'ruby', tpl: 'jinja', gotmpl: 'jinja',
}

// Files known by their name.
const byName: Record<string, CodeLang> = {
  dockerfile: 'dockerfile', containerfile: 'dockerfile', jenkinsfile: 'groovy', makefile: 'shell',
  gemfile: 'ruby', rakefile: 'ruby', vagrantfile: 'ruby', 'cmakelists.txt': 'cmake', 'nginx.conf': 'nginx',
  '.bashrc': 'shell', '.zshrc': 'shell', '.profile': 'shell', '.env': 'ini', '.gitconfig': 'ini',
  '.editorconfig': 'ini', '.npmrc': 'ini',
}

/** The code language of a file, by its name; undefined for plain text
 *  and for YAML and JSON, which have their own parsers. */
export function langForPath(path: string): CodeLang | undefined {
  const name = path.slice(path.lastIndexOf('/') + 1).toLowerCase()
  if (byName[name]) return byName[name]
  if (name.startsWith('dockerfile.') || name.endsWith('.dockerfile')) return 'dockerfile'
  const dot = name.lastIndexOf('.')
  if (dot < 0) return undefined
  const ext = name.slice(dot + 1)
  return byExt[ext] ?? codeLang(ext)
}

const loading = new Map<CodeLang, Promise<Language>>()
const ready = new Map<CodeLang, Language>()

/** Loads a language once; every editor and block shares it. */
export function loadLanguage(name: CodeLang): Promise<Language> {
  let p = loading.get(name)
  if (!p) {
    p = loaders[name]().then((mode) => {
      const lang = StreamLanguage.define(withPlainDots(mode))
      ready.set(name, lang)
      return lang
    })
    loading.set(name, p)
  }
  return p
}

/** Some modes (Go's) read a lone "." as the start of a number and style
 *  it so: every fmt.Println would show an orange dot. */
function withPlainDots<S>(mode: StreamParser<S>): StreamParser<S> {
  return {
    ...mode,
    token(stream, state) {
      const style = mode.token(stream, state)
      return style === 'number' && stream.current() === '.' ? null : style
    },
  }
}

/** The language if it is loaded already, to draw with at once. */
export const loadedLanguage = (name: CodeLang): Language | undefined => ready.get(name)

/**
 * Folding by indentation, for languages without a parser's structure:
 * a line folds over the lines after it that are indented further (blank
 * lines inside go with them).
 */
export const indentFolding = foldService.of((state, from) => {
  const line = state.doc.lineAt(from)
  const indent = (s: string) => (s.trim() === '' ? -1 : s.length - s.trimStart().length)
  const own = indent(line.text)
  if (own < 0 || line.number === state.doc.lines) return null
  let end = line
  for (let n = line.number + 1; n <= state.doc.lines; n++) {
    const next = state.doc.line(n)
    const i = indent(next.text)
    if (i === -1) continue
    if (i <= own) break
    end = next
  }
  return end.number > line.number ? { from: line.to, to: end.to } : null
})

// HCL (Terraform, OpenTofu, Packer, Nomad): no legacy mode has it, and
// this is all highlighting needs: comments, strings with ${…}, numbers,
// block keywords and attribute names.
interface HclState {
  inString: boolean
  depth: number // ${ … } nesting inside a string
  heredoc: string | null
}

const hclBlocks = new Set(['resource', 'data', 'module', 'variable', 'output', 'locals', 'provider', 'terraform', 'backend', 'required_providers', 'moved', 'import', 'check', 'dynamic', 'content', 'lifecycle', 'provisioner', 'connection', 'source', 'build', 'job', 'group', 'task'])

const hcl: StreamParser<HclState> = {
  name: 'hcl',
  startState: () => ({ inString: false, depth: 0, heredoc: null }),
  token(stream, st) {
    if (st.heredoc) {
      if (stream.sol() && stream.match(new RegExp(`^\\s*${st.heredoc}\\s*$`))) {
        st.heredoc = null
        return 'string'
      }
      stream.skipToEnd()
      return 'string'
    }
    if (st.inString) {
      if (stream.match('${') || stream.match('%{')) {
        st.inString = false
        st.depth = 1
        return 'meta'
      }
      while (!stream.eol()) {
        const c = stream.next()
        if (c === '\\') stream.next()
        else if (c === '"') {
          st.inString = false
          return 'string'
        } else if ((c === '$' || c === '%') && stream.peek() === '{') {
          stream.backUp(1)
          return 'string'
        }
      }
      return 'string'
    }
    if (stream.eatSpace()) return null
    if (stream.match('#') || stream.match('//')) {
      stream.skipToEnd()
      return 'comment'
    }
    if (stream.match('/*')) {
      while (!stream.eol() && !stream.match('*/')) stream.next()
      return 'comment'
    }
    const here = stream.match(/^<<-?([A-Z_]+)\s*$/)
    if (here) {
      st.heredoc = (here as RegExpMatchArray)[1]
      return 'string'
    }
    if (st.depth > 0) {
      if (stream.eat('{')) st.depth++
      else if (stream.eat('}')) {
        if (--st.depth === 0) st.inString = true
        return st.depth === 0 ? 'meta' : 'bracket'
      }
    }
    if (stream.eat('"')) {
      st.inString = true
      return 'string'
    }
    if (stream.match(/^-?\d+(\.\d+)?([eE][+-]?\d+)?/)) return 'number'
    if (stream.match(/^(true|false|null)\b/)) return 'atom'
    if (stream.match(/^(for|in|if|else|endif|endfor)\b/)) return 'keyword'
    const word = stream.match(/^[A-Za-z_][\w-]*/)
    if (word) {
      const w = (word as RegExpMatchArray)[0]
      if (stream.sol() || stream.string.slice(0, stream.start).trim() === '') {
        if (hclBlocks.has(w)) return 'keyword'
        if (/^\s*=(?!=)/.test(stream.string.slice(stream.pos))) return 'propertyName'
      }
      if (stream.peek() === '(') return 'variableName.function'
      return 'variableName'
    }
    stream.next()
    return null
  },
  languageData: { commentTokens: { line: '#', block: { open: '/*', close: '*/' } } },
}
