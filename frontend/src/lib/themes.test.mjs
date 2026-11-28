// The theme cards' colours are copies of tokens.css: npm test.
import { test } from 'node:test'
import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { themes } from './themes.ts'

const css = readFileSync(new URL('./styles/tokens.css', import.meta.url), 'utf8')

// As in scripts/contrast.mjs: a theme's tokens are :root's, then every
// block whose selector list names it, in file order.
function block(selector) {
  const tokens = {}
  for (let at = css.indexOf(selector); at >= 0; at = css.indexOf(selector, at + 1)) {
    const next = css[at + selector.length]
    if (next !== ' ' && next !== ',') continue
    const open = css.indexOf('{', at)
    const body = css.slice(open, css.indexOf('\n}', open))
    for (const m of body.matchAll(/^\s*(--[\w-]+):\s*([^;]+);/gm)) tokens[m[1]] = m[2].trim()
  }
  return tokens
}
const base = block(':root')
const tokensOf = (t) => (t === 'dark' ? base : { ...base, ...block(`:root[data-theme='${t}']`) })

function value(tokens, name) {
  let v = tokens[name]
  for (let i = 0; v?.startsWith('var(') && i < 10; i++) v = tokens[v.slice(4, -1).split(',')[0].trim()]
  return v?.toLowerCase()
}

const hueNames = ['indigo', 'violet', 'orchid', 'pink', 'peach', 'lime', 'teal', 'cyan']

for (const t of themes) {
  test(`${t.label}: swatch and hues match tokens.css`, () => {
    const tokens = tokensOf(t.value)
    const [bg, fg, accent, hue] = t.swatch
    assert.equal(bg, value(tokens, '--bg-2'), '--bg-2')
    assert.equal(fg, value(tokens, '--fg-0'), '--fg-0')
    assert.equal(accent, value(tokens, '--accent'), '--accent')
    assert.deepEqual(
      t.hues,
      hueNames.map((h) => value(tokens, `--hue-${h}`)),
    )
    assert.ok(t.hues.includes(hue), 'the swatch hue is one of the theme hues')
  })
}
