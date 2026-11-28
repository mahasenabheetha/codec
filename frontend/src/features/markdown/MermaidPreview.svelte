<script lang="ts">
  import { untrack } from 'svelte'
  import Maximize from '@lucide/svelte/icons/maximize'
  import Minus from '@lucide/svelte/icons/minus'
  import Plus from '@lucide/svelte/icons/plus'
  import ErrorNote from '../../lib/components/ErrorNote.svelte'
  import IconButton from '../../lib/components/IconButton.svelte'
  import Working from '../../lib/components/Working.svelte'
  import { toolColours } from '../../lib/stores/colour.svelte'
  import { theme } from '../../lib/stores/theme.svelte'
  import { toast } from '../../lib/stores/toast.svelte'
  import { copyText } from '../../lib/utils/clipboard'
  import { drawMermaid } from './mermaid'

  // A Mermaid file drawn (3.10.3): the diagram fitted to the pane, to
  // drag, zoom (wheel, + and -) and fit again (0), redrawn a moment
  // after the text changes (what-if edits too) and when the theme does.
  // A diagram that doesn't parse shows why, over the last one that did.
  // The file's header copies it as SVG and saves it as PNG (3.11.0).
  interface Props {
    text: string
    /** The file's name, for the PNG's. */
    name?: string
    /** The kind of diagram drawn (flowchart, sequence…), for the header. */
    kind?: string
  }
  let { text, name = 'diagram', kind = $bindable('') }: Props = $props()

  let svg = $state('')
  let error = $state('')
  let busy = $state(false)

  export function copySVG() {
    if (svg) copyText(svg, 'SVG')
  }

  // Drawn in the page, nothing fetched: the diagram as it shows (its
  // size set below) on a canvas at twice its size, on the pane's ground
  // so its text stays readable whatever opens the file.
  export async function savePNG() {
    const el = stage?.querySelector('svg')
    if (!el || !host) return
    const w = Number(el.getAttribute('width')) || el.getBoundingClientRect().width
    const h = Number(el.getAttribute('height')) || el.getBoundingClientRect().height
    const scale = Math.min(2, 8192 / Math.max(w, h))
    const img = new Image()
    img.src = 'data:image/svg+xml;charset=utf-8,' + encodeURIComponent(new XMLSerializer().serializeToString(el))
    try {
      await img.decode()
      const canvas = Object.assign(document.createElement('canvas'), { width: Math.round(w * scale), height: Math.round(h * scale) })
      const g = canvas.getContext('2d')!
      g.fillStyle = getComputedStyle(host).backgroundColor
      g.fillRect(0, 0, canvas.width, canvas.height)
      g.drawImage(img, 0, 0, canvas.width, canvas.height)
      const blob = await new Promise<Blob | null>((done) => canvas.toBlob(done, 'image/png'))
      if (!blob) throw new Error('the browser could not make the PNG')
      const url = URL.createObjectURL(blob)
      Object.assign(document.createElement('a'), { href: url, download: `${name.replace(/\.[^.]+$/, '')}.png` }).click()
      setTimeout(() => URL.revokeObjectURL(url), 1000)
    } catch (e) {
      toast(`Couldn't save the PNG: ${e instanceof Error ? e.message : String(e)}`, 'err')
    }
  }

  $effect(() => {
    const src = text
    void theme.resolved
    void toolColours.on
    // The drawing before is read untracked: a new drawing must not ask
    // for another one.
    const first = !untrack(() => svg)
    busy = true
    let live = true
    const t = setTimeout(async () => {
      const r = await drawMermaid(src)
      if (!live) return
      busy = false
      if ('error' in r) error = r.error
      else ((svg = r.svg), (kind = r.kind.replace(/-v\d+$/, '')), (error = ''))
    }, first ? 0 : 250)
    return () => {
      live = false
      clearTimeout(t)
    }
  })

  // --- pan and zoom, as in the graphs ---
  let host = $state<HTMLDivElement>()
  let stage = $state<HTMLDivElement>()
  let view = $state({ x: 0, y: 0, k: 1 })
  let size = { w: 0, h: 0 }
  let moved = false
  let drag: { x: number; y: number; vx: number; vy: number } | null = null

  function fit() {
    const el = stage?.querySelector('svg')
    if (!host || !el) return
    const r = host.getBoundingClientRect()
    if (!r.width || !r.height) return
    const vb = el.viewBox.baseVal
    size = vb && vb.width ? { w: vb.width, h: vb.height } : { w: el.getBoundingClientRect().width / view.k, h: el.getBoundingClientRect().height / view.k }
    moved = false
    // Never below a size where its words stay readable, as the graphs
    // do: a big diagram (a wide Gantt chart) then starts at its top left,
    // and the rest is a drag away.
    const k = Math.min(1.5, Math.max(READABLE, Math.min((r.width - 32) / size.w, (r.height - 32) / size.h)))
    const x = size.w * k < r.width ? (r.width - size.w * k) / 2 : 16
    const y = size.h * k < r.height ? (r.height - size.h * k) / 2 : 16
    view = { k, x, y }
  }
  const READABLE = 0.75
  // A new drawing fits, unless the view was arranged and it is the same
  // diagram redrawn (an edit, a theme): then the view stays.
  let lastKind = ''
  $effect(() => {
    void svg
    queueMicrotask(() => {
      const el = stage?.querySelector('svg')
      if (!el) return
      // Mermaid sizes its SVG to its container; the view does the sizing.
      const vb = el.viewBox.baseVal
      if (vb?.width) {
        el.setAttribute('width', String(vb.width))
        el.setAttribute('height', String(vb.height))
      }
      el.style.maxWidth = 'none'
      const kind = el.getAttribute('aria-roledescription') ?? ''
      if (!moved || kind !== lastKind) fit()
      lastKind = kind
    })
  })
  $effect(() => {
    if (!host) return
    const ro = new ResizeObserver(() => {
      if (!moved) fit()
    })
    ro.observe(host)
    return () => ro.disconnect()
  })
  function zoom(by: number, cx?: number, cy?: number) {
    const r = host?.getBoundingClientRect()
    const x = cx ?? (r ? r.width / 2 : 0)
    const y = cy ?? (r ? r.height / 2 : 0)
    const k = Math.min(6, Math.max(0.1, view.k * by))
    moved = true
    view = { k, x: x - ((x - view.x) * k) / view.k, y: y - ((y - view.y) * k) / view.k }
  }
  function onwheel(e: WheelEvent) {
    e.preventDefault()
    const r = host!.getBoundingClientRect()
    zoom(e.deltaY < 0 ? 1.12 : 1 / 1.12, e.clientX - r.left, e.clientY - r.top)
  }
  function onpointerdown(e: PointerEvent) {
    if ((e.target as Element).closest('.controls, .problem')) return
    drag = { x: e.clientX, y: e.clientY, vx: view.x, vy: view.y }
    ;(e.currentTarget as Element).setPointerCapture(e.pointerId)
  }
  function onpointermove(e: PointerEvent) {
    if (!drag) return
    moved = true
    view = { ...view, x: drag.vx + e.clientX - drag.x, y: drag.vy + e.clientY - drag.y }
  }
  function onkeydown(e: KeyboardEvent) {
    const step = e.shiftKey ? 120 : 40
    if (e.key === '+' || e.key === '=') zoom(1.25)
    else if (e.key === '-' || e.key === '_') zoom(0.8)
    else if (e.key === '0') fit()
    else if (e.key === 'ArrowLeft') ((moved = true), (view = { ...view, x: view.x + step }))
    else if (e.key === 'ArrowRight') ((moved = true), (view = { ...view, x: view.x - step }))
    else if (e.key === 'ArrowUp') ((moved = true), (view = { ...view, y: view.y + step }))
    else if (e.key === 'ArrowDown') ((moved = true), (view = { ...view, y: view.y - step }))
    else return
    e.preventDefault()
  }
</script>

<!-- A focusable picture you can move and zoom; the keys are listed in its name. -->
<!-- svelte-ignore a11y_no_noninteractive_tabindex, a11y_no_noninteractive_element_interactions -->
<div
  class="mermaid"
  bind:this={host}
  role="img"
  aria-label="Mermaid diagram. Drag or use the arrow keys to move, plus and minus zoom, 0 fits."
  tabindex="0"
  {onwheel}
  {onpointerdown}
  {onpointermove}
  onpointerup={() => (drag = null)}
  onpointercancel={() => (drag = null)}
  {onkeydown}
>
  {#if svg}
    <div class="stage" class:stale={!!error} bind:this={stage} style:transform="translate({view.x}px, {view.y}px) scale({view.k})">
      {@html svg /* cleaned by drawMermaid */}
    </div>
  {:else if busy}
    <div class="center"><Working label="Drawing the diagram…" /></div>
  {/if}
  {#if error}
    <div class="problem">
      <ErrorNote message={svg ? "This version doesn't parse; showing the last one that did." : "Mermaid couldn't draw this diagram."} />
      <pre>{error}</pre>
    </div>
  {/if}
  {#if svg}<div class="legend">{kind ? `${kind} · ` : ''}{Math.round(view.k * 100)}%</div>{/if}
  <div class="controls">
    <IconButton icon={Plus} label="Zoom in" shortcut="+" size="sm" side="left" onclick={() => zoom(1.25)} />
    <IconButton icon={Minus} label="Zoom out" shortcut="-" size="sm" side="left" onclick={() => zoom(0.8)} />
    <IconButton icon={Maximize} label="Fit to view" shortcut="0" size="sm" side="left" onclick={fit} />
  </div>
</div>

<style>
  .mermaid {
    position: relative;
    flex: 1;
    min-height: 0;
    height: 100%;
    overflow: hidden;
    /* A faint grid behind the diagram, as on graph paper. */
    background-color: var(--bg-2);
    background-image:
      linear-gradient(color-mix(in srgb, var(--border) 70%, transparent) 1px, transparent 1px),
      linear-gradient(90deg, color-mix(in srgb, var(--border) 70%, transparent) 1px, transparent 1px);
    background-size: var(--s-6) var(--s-6);
    background-position: -1px -1px;
    cursor: grab;
    touch-action: none;
  }
  .mermaid:active {
    cursor: grabbing;
  }
  .mermaid:focus-visible {
    outline: none;
    box-shadow: inset var(--focus-ring);
  }
  .stage {
    position: absolute;
    left: 0;
    top: 0;
    transform-origin: 0 0;
  }
  .stage.stale {
    opacity: 0.45;
  }
  .stage :global(svg) {
    display: block;
  }
  .center {
    display: grid;
    place-items: center;
    height: 100%;
  }
  .problem {
    position: absolute;
    left: var(--s-3);
    right: var(--s-3);
    top: var(--s-3);
    padding: var(--s-2) var(--s-3);
    background: var(--bg-1);
    border: 1px solid var(--border);
    border-radius: var(--r-md);
    box-shadow: var(--shadow-raise);
    cursor: auto;
  }
  /* Mermaid's own message: the line and a caret under the fault, so
     letters keep their width and --> stays three characters. */
  .problem pre {
    margin: var(--s-2) 0 0;
    font-variant-ligatures: none;
    max-height: 9em;
    overflow: auto;
    font-family: var(--font-mono);
    font-size: var(--fs-sm);
    color: var(--fg-1);
    white-space: pre-wrap;
  }
  .controls,
  .legend {
    position: absolute;
    bottom: var(--s-4);
    background: var(--bg-1);
    border: 1px solid var(--border);
    border-radius: var(--r-lg);
    cursor: auto;
  }
  .controls {
    right: var(--s-4);
    display: flex;
    flex-direction: column;
    gap: 2px;
    padding: var(--s-1);
  }
  .legend {
    left: var(--s-4);
    padding: var(--s-1) var(--s-3);
    font-size: var(--fs-xs);
    color: var(--fg-1);
  }
</style>
