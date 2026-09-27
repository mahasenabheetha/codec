// CodeMirror extension for the merged-values view: a gutter marker per
// line coloured by the layer that won, and a hover with the full chain
// ("3 ← values-prod.yaml:12, overrides values.yaml:4").

import { gutter, GutterMarker, hoverTooltip, EditorView } from '@codemirror/view'
import type { Extension } from '@codemirror/state'
import type { HelmResult, Origin } from '../../lib/api/helm'
import { tooltipTheme } from '../editor/intel'

/** Layer colours, in order of first appearance; defaults stay grey. */
const palette = ['var(--accent)', 'var(--ok)', 'var(--warn)', 'var(--syn-bool)', 'var(--syn-anchor)', 'var(--info)', 'var(--syn-number)']

export function layerColors(result: HelmResult | null, layers: string[]): Map<string, string> {
  const colors = new Map<string, string>()
  layers.forEach((l, i) => colors.set(l, palette[i % palette.length]))
  let next = layers.length
  for (const chain of Object.values(result?.provenance ?? {})) {
    for (const o of chain) {
      if (colors.has(o.layer)) continue
      colors.set(o.layer, o.kind === 'default' || o.kind === 'computed' ? 'var(--fg-2)' : palette[next++ % palette.length])
    }
  }
  return colors
}

/** Where an origin is, e.g. "values-prod.yaml:12". */
export function where(o: Origin): string {
  return o.line ? `${o.layer}:${o.line}` : o.layer
}

export function formatValue(v: unknown): string {
  if (v === undefined) return ''
  if (typeof v === 'string') return JSON.stringify(v)
  return String(v)
}

class LayerMarker extends GutterMarker {
  constructor(
    readonly color: string,
    readonly title: string,
  ) {
    super()
  }
  eq(other: LayerMarker) {
    return other.color === this.color && other.title === this.title
  }
  toDOM() {
    const el = document.createElement('div')
    el.className = 'cm-prov-marker'
    el.style.background = this.color
    el.title = this.title
    return el
  }
}

/** The path a line belongs to: its own key, else the nearest key above
 *  (list items belong to their list). */
function pathAt(result: HelmResult, line: number): string | undefined {
  for (let l = line; l >= 1; l--) {
    const p = result.valuesLines[String(l)]
    if (p) return p
  }
  return undefined
}

export function provenanceView(result: HelmResult, colors: Map<string, string>): Extension {
  return [
    tooltipTheme,
    gutter({
      class: 'cm-prov-gutter',
      lineMarker(view, block) {
        const line = view.state.doc.lineAt(block.from).number
        const p = result.valuesLines[String(line)]
        const winner = p ? result.provenance[p]?.[0] : undefined
        return winner ? new LayerMarker(colors.get(winner.layer) ?? 'var(--fg-2)', where(winner)) : null
      },
    }),
    hoverTooltip((view, pos) => {
      const line = view.state.doc.lineAt(pos)
      const p = pathAt(result, line.number)
      const chain = p ? result.provenance[p] : undefined
      if (!p || !chain?.length) return null
      return {
        pos: line.from,
        end: line.to,
        above: true,
        create: () => {
          const el = document.createElement('div')
          el.className = 'cm-hover'
          const title = document.createElement('div')
          title.className = 'cm-hover-title'
          title.textContent = p
          el.append(title)
          const dl = document.createElement('dl')
          chain.forEach((o, i) => {
            const dt = document.createElement('dt')
            dt.textContent = i === 0 ? (o.deleted ? 'Removed by' : 'Set by') : 'Overrides'
            const dd = document.createElement('dd')
            dd.textContent = `${where(o)}${o.value !== undefined ? ' = ' + formatValue(o.value) : ''}`
            const dot = document.createElement('span')
            dot.className = 'cm-prov-dot'
            dot.style.background = colors.get(o.layer) ?? 'var(--fg-2)'
            dd.prepend(dot)
            dl.append(dt, dd)
          })
          el.append(dl)
          return { dom: el }
        },
      }
    }),
    EditorView.theme({
      '.cm-prov-gutter': { width: '6px', paddingLeft: '2px' },
      '.cm-prov-marker': { width: '3px', height: '100%', minHeight: '14px', borderRadius: '2px' },
      '.cm-prov-dot': { display: 'inline-block', width: '7px', height: '7px', borderRadius: '50%', marginRight: '6px' },
    }),
  ]
}
