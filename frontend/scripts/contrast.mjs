// Checks the colour tokens in src/lib/styles/tokens.css against the
// design language's contrast rules, for every theme (decision 94).
// `npm run contrast`; exits 1 when a pair fails. No dependencies.
import { readFileSync } from 'node:fs'

const css = readFileSync(new URL('../src/lib/styles/tokens.css', import.meta.url), 'utf8')

// Theme blocks: :root is Dark; the others override it.
function block(selector) {
  const at = css.indexOf(selector + ' {')
  if (at < 0) throw new Error('no block ' + selector)
  const body = css.slice(at, css.indexOf('\n}', at))
  return Object.fromEntries([...body.matchAll(/^\s*(--[\w-]+):\s*([^;]+);/gm)].map((m) => [m[1], m[2].trim()]))
}
const base = block(':root')
const themes = {
  dark: base,
  light: { ...base, ...block(":root[data-theme='light']") },
  dusk: { ...base, ...block(":root[data-theme='dusk']") },
}

// A colour as [r, g, b, a], following var() within the theme.
function color(t, name) {
  let v = t[name]
  for (let i = 0; v?.startsWith('var('); i++) v = t[v.slice(4, -1).split(',')[0].trim()]
  if (!v) throw new Error('no colour ' + name)
  let m = v.match(/^#([0-9a-f]{6})([0-9a-f]{2})?$/i)
  if (m) {
    const n = parseInt(m[1], 16)
    return [n >> 16, (n >> 8) & 255, n & 255, m[2] ? parseInt(m[2], 16) / 255 : 1]
  }
  m = v.match(/^rgb\((\d+) (\d+) (\d+)(?: \/ ([\d.]+))?\)$/)
  if (m) return [+m[1], +m[2], +m[3], m[4] ? +m[4] : 1]
  throw new Error(`can't read ${name}: ${v}`)
}
const over = (fg, bg) => fg.slice(0, 3).map((c, i) => c * fg[3] + bg[i] * (1 - fg[3])).concat(1)
function lum([r, g, b]) {
  const f = (c) => ((c /= 255) <= 0.04045 ? c / 12.92 : ((c + 0.055) / 1.055) ** 2.4)
  return 0.2126 * f(r) + 0.7152 * f(g) + 0.0722 * f(b)
}
function ratio(a, b) {
  const [x, y] = [lum(a), lum(b)].sort((p, q) => q - p)
  return (x + 0.05) / (y + 0.05)
}

let failed = 0
function check(theme, fgName, bgName, min, max = Infinity) {
  const t = themes[theme]
  const bg = bgName.includes('-soft') ? over(color(t, bgName), color(t, '--bg-2')) : color(t, bgName)
  const r = ratio(over(color(t, fgName), bg), bg)
  if (r < min || r > max) {
    failed++
    const want = max < Infinity ? `${min}–${max}` : `≥ ${min}`
    console.log(`${theme}: ${fgName} on ${bgName} is ${r.toFixed(2)}:1, wants ${want}`)
  }
}

const surfaces = ['--bg-0', '--bg-1', '--bg-2', '--bg-3']
const status = ['--ok', '--warn', '--err', '--info']
const hues = ['indigo', 'violet', 'pink', 'peach', 'lime', 'teal', 'cyan'].map((h) => '--hue-' + h)
for (const theme of Object.keys(themes)) {
  const t = themes[theme]
  for (const bg of surfaces) for (const fg of ['--fg-0', '--fg-1', '--fg-2']) check(theme, fg, bg, 4.5)
  for (const fg of ['--accent', ...status]) {
    check(theme, fg, '--bg-2', 4.5)
    check(theme, fg, fg + '-soft', 4.5)
  }
  check(theme, '--accent-fg', '--accent', 4.5)
  for (const syn of Object.keys(t).filter((k) => k.startsWith('--syn-') && !k.endsWith('-bg'))) check(theme, syn, '--bg-2', 4.5)
  for (const logo of Object.keys(t).filter((k) => k.startsWith('--brand-'))) check(theme, logo, '--bg-1', 3)
}
// Dusk shows the hues: as icons and text on panels, and as the fill of a
// tool's primary button. And it stays soft: body text about 10:1, at
// most 12:1 (Dark is about 16:1).
for (const h of hues) {
  check('dusk', h, '--bg-2', 4.5)
  check('dusk', '--accent-fg', h, 4.5)
}
for (const bg of surfaces) check('dusk', '--fg-0', bg, 7, 12)

console.log(failed ? `${failed} pair(s) fail` : 'All contrast pairs pass in Dark, Light and Dusk.')
process.exit(failed ? 1 : 0)
