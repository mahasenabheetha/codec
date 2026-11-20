// Mermaid diagrams (3.10.3): .mmd and .mermaid files, and ```mermaid
// blocks in a Markdown preview. The text is untrusted (a repository can
// hold anything), so:
//  - Mermaid runs at its strict security level: no click handlers, no
//    scripts, labels as plain SVG text rather than HTML;
//  - its SVG is then cleaned once more here, and keeps no link that
//    leaves the diagram (an href that isn't #…), so nothing in it can
//    run, load from elsewhere or navigate;
//  - a diagram's own %%{init}%% can't loosen any of that: the security
//    settings, labels, theme CSS and fonts are locked;
//  - nothing is fetched: Mermaid is bundled, loaded the first time a
//    diagram is shown, no icon pack or web font is registered, and CSS
//    in the SVG keeps no url() that leaves it and no @import.
// Colours come from the theme in use, so a diagram matches the page.
import DOMPurify from 'dompurify'
import type { Mermaid } from 'mermaid'

let loading: Promise<Mermaid> | undefined
const load = () => (loading ??= import('mermaid').then((m) => m.default))

/** A colour token as Mermaid can read it: rgb(…), resolved in the page. */
function token(name: string, probe: HTMLElement): string {
  probe.style.color = `var(${name})`
  return getComputedStyle(probe).color
}
function rgb(c: string): number[] {
  return (c.match(/[\d.]+/g) ?? ['0', '0', '0']).slice(0, 3).map(Number)
}
/** a mixed into b, k of a. */
function mix(a: string, b: string, k: number): string {
  const [x, y] = [rgb(a), rgb(b)]
  return `rgb(${x.map((v, i) => Math.round(v * k + y[i] * (1 - k))).join(', ')})`
}

function themeVariables(): Record<string, string | boolean> {
  const probe = document.createElement('span')
  probe.style.display = 'none'
  document.body.append(probe)
  const t = (n: string) => token(n, probe)
  const [bg, panel, fg, muted, border, accent] = ['--bg-2', '--bg-1', '--fg-0', '--fg-1', '--border-strong', '--accent'].map(t)
  const [violet, teal, peach] = ['--hue-violet', '--hue-teal', '--hue-peach'].map(t)
  const font = getComputedStyle(document.documentElement).getPropertyValue('--font-ui').trim()
  probe.remove()
  const dark = rgb(bg).reduce((s, v) => s + v, 0) < 384
  // Nodes are a touch of colour on the page's ground, their text the
  // page's text, so every label keeps the page's contrast.
  const node = mix(accent, bg, 0.16)
  return {
    darkMode: dark,
    background: bg,
    fontFamily: font || 'sans-serif',
    fontSize: '14px',
    primaryColor: node,
    primaryTextColor: fg,
    primaryBorderColor: mix(accent, bg, 0.7),
    secondaryColor: mix(violet, bg, 0.16),
    secondaryTextColor: fg,
    secondaryBorderColor: mix(violet, bg, 0.6),
    tertiaryColor: mix(teal, bg, 0.14),
    tertiaryTextColor: fg,
    tertiaryBorderColor: mix(teal, bg, 0.6),
    lineColor: muted,
    textColor: fg,
    mainBkg: node,
    nodeBorder: mix(accent, bg, 0.7),
    clusterBkg: mix(panel, bg, 0.6),
    clusterBorder: border,
    titleColor: fg,
    edgeLabelBackground: bg,
    noteBkgColor: mix(peach, bg, 0.18),
    noteTextColor: fg,
    noteBorderColor: mix(peach, bg, 0.6),
    actorBkg: node,
    actorBorder: mix(accent, bg, 0.7),
    actorTextColor: fg,
    signalColor: muted,
    signalTextColor: fg,
    labelBoxBkgColor: node,
    labelTextColor: fg,
    loopTextColor: fg,
  }
}

// Mermaid's own SVG, cleaned: the SVG profile (no foreignObject, no
// script), no event handlers, and no link or image source that leaves
// the diagram.
let purifier: ReturnType<typeof DOMPurify> | undefined
const outside = /url\(\s*(?!['"]?#)[^)]*\)/gi
const inert = (css: string) => css.replace(/@import[^;]*;?/gi, '').replace(outside, 'none')
function clean(svg: string): string {
  if (!purifier) {
    purifier = DOMPurify(window)
    purifier.addHook('afterSanitizeAttributes', (node) => {
      for (const a of ['href', 'xlink:href']) {
        const v = node.getAttribute(a)
        if (v !== null && !v.startsWith('#')) node.removeAttribute(a)
      }
      const style = node.getAttribute('style')
      if (style) node.setAttribute('style', inert(style))
      if (node.tagName.toLowerCase() === 'style') node.textContent = inert(node.textContent ?? '')
    })
  }
  return purifier.sanitize(svg, { USE_PROFILES: { svg: true, svgFilters: true }, ADD_TAGS: ['style'] }) as string
}

let seq = 0
let queue: Promise<unknown> = Promise.resolve()

export type Drawn = { svg: string; kind: string } | { error: string }

/**
 * drawMermaid turns Mermaid text into clean SVG in the current theme, or
 * the reason it can't. Renders run one at a time (Mermaid keeps global
 * state while it draws).
 */
export function drawMermaid(text: string): Promise<Drawn> {
  const run = async (): Promise<Drawn> => {
    const mermaid = await load()
    mermaid.initialize({
      startOnLoad: false,
      securityLevel: 'strict',
      secure: ['secure', 'securityLevel', 'startOnLoad', 'maxTextSize', 'suppressErrorRendering', 'maxEdges', 'htmlLabels', 'flowchart', 'themeCSS', 'fontFamily', 'altFontFamily', 'dompurifyConfig'],
      htmlLabels: false,
      flowchart: { htmlLabels: false },
      // A Gantt chart is drawn at a set width with text as big as the
      // other kinds', not across the whole page in 11 px.
      gantt: { useWidth: 960, fontSize: 13, sectionFontSize: 13, barHeight: 24, barGap: 6 },
      theme: 'base',
      themeVariables: themeVariables(),
      deterministicIds: true,
      deterministicIDSeed: `codec-${++seq}`,
      suppressErrorRendering: true,
      maxTextSize: 200_000,
    })
    try {
      const { svg, diagramType } = await mermaid.render(`codec-mermaid-${seq}`, text)
      return { svg: clean(svg), kind: diagramType }
    } catch (e) {
      return { error: (e instanceof Error ? e.message : String(e)).trim() || "Mermaid couldn't read this diagram" }
    } finally {
      // A failed render can leave its scratch element behind.
      document.getElementById(`dcodec-mermaid-${seq}`)?.remove()
    }
  }
  const next = queue.then(run, run)
  queue = next.catch(() => {})
  return next
}
