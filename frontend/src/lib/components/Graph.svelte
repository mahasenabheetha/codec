<script lang="ts" module>
  export interface GraphNode {
    id: string
    label: string
    sub?: string // small line above the label, e.g. the kind
    group?: string // colour group, see groupColor
    missing?: boolean // referenced but absent: drawn dashed
    status?: 'failed' | 'changed' | 'ok' | 'rescued' | 'never' // from a loaded run
    /** A band the node sits in, with the others of the same lane: a
     *  pipeline's stage. Lanes are drawn in the order first seen. */
    lane?: string
  }
  export interface GraphEdge {
    from: string
    to: string
    label?: string
    dashed?: boolean // decided at run time
  }
  // Several graphs are mounted at once (tabs stay mounted), and an id
  // in a hidden one would win: each graph numbers its own arrowhead.
  let graphs = 0
</script>

<script lang="ts">
  import { Graph as DagreGraph, layout } from '@dagrejs/dagre'
  import Maximize from '@lucide/svelte/icons/maximize'
  import Minus from '@lucide/svelte/icons/minus'
  import Plus from '@lucide/svelte/icons/plus'
  import IconButton from './IconButton.svelte'

  // A directed graph laid out left to right with dagre, drawn as SVG.
  // Shared by the Kubernetes, Argo, CI, Compose and Ansible lenses.
  //  - Drag to pan, wheel to zoom; hovering a node highlights its edges.
  //  - Keyboard (3.10.2): Tab reaches the graph and then each node;
  //    arrows move between nodes (or pan, with the graph itself
  //    focused), + and − zoom, 0 fits, Enter or Space selects.
  //  - A minimap appears when the graph is bigger than its box.
  //  - The view survives a change of the same graph (a what-if value);
  //    a different graph is fitted afresh.
  //  - A run's status is a glyph as well as a colour.
  interface Props {
    nodes: GraphNode[]
    edges: GraphEdge[]
    label: string
    legend?: { group: string; label: string }[]
    /** The node whose details are showing, drawn with a firm outline. */
    selected?: string | null
    /** What a dashed edge and a dashed node mean here, for the legend. */
    dashedEdge?: string
    missingNode?: string
    onselect?: (id: string) => void
  }

  let { nodes, edges, label, legend = [], selected = null, dashedEdge = 'decided at run time', missingNode = 'not found', onselect }: Props = $props()
  const arrow = `graph-arrow-${++graphs}`

  const NODE_H = 46
  const MAX_W = 280
  const CHAR = 7.2
  const width = (n: GraphNode) => Math.min(MAX_W, Math.max(120, Math.max(n.label.length, (n.sub ?? '').length) * CHAR + 32 + (n.status ? 16 : 0)))
  /** Text cut to the node's width; the whole of it is the node's tooltip. */
  const cut = (text: string, w: number, per = CHAR) => {
    const room = Math.max(4, Math.floor((w - 28) / per))
    return text.length > room ? text.slice(0, room - 1) + '…' : text
  }

  const lanes = $derived([...new Set(nodes.map((n) => n.lane).filter((l): l is string => !!l))])
  const laneId = (l: string) => '__lane__:' + l // not a node id any lens uses

  const laid = $derived.by(() => {
    const g = new DagreGraph({ multigraph: true, compound: lanes.length > 0 })
    g.setGraph({ rankdir: 'LR', nodesep: 16, ranksep: lanes.length ? 44 : 70, marginx: 20, marginy: lanes.length ? 28 : 20 })
    g.setDefaultEdgeLabel(() => ({}))
    for (const l of lanes) g.setNode(laneId(l), {})
    for (const n of nodes) {
      g.setNode(n.id, { width: width(n), height: NODE_H })
      if (n.lane) g.setParent(n.id, laneId(n.lane))
    }
    edges.forEach((e, i) => {
      if (g.hasNode(e.from) && g.hasNode(e.to)) g.setEdge(e.from, e.to, {}, String(i))
    })
    layout(g)
    const pos = new Map<string, { x: number; y: number; w: number }>()
    for (const n of nodes) {
      const p = g.node(n.id)
      if (p) pos.set(n.id, { x: p.x, y: p.y, w: width(n) })
    }
    const paths = edges.map((e, i) => {
      const pts = g.hasNode(e.from) && g.hasNode(e.to) ? (g.edge({ v: e.from, w: e.to, name: String(i) })?.points ?? []) : []
      return { e, d: smooth(pts), mid: pts[Math.floor(pts.length / 2)] }
    })
    const bands = lanes
      .map((l) => ({ name: l, box: g.node(laneId(l)) }))
      .filter((b) => b.box && Number.isFinite(b.box.x))
      .map((b) => ({ name: b.name, x: b.box.x - b.box.width / 2, y: b.box.y - b.box.height / 2, w: b.box.width, h: b.box.height }))
    const gw = g.graph().width ?? 0
    const gh = g.graph().height ?? 0
    return { pos, paths, bands, w: gw, h: gh }
  })

  // A curve through dagre's points (quadratic segments via midpoints).
  function smooth(pts: { x: number; y: number }[]): string {
    if (pts.length < 2) return ''
    let d = `M${pts[0].x},${pts[0].y}`
    for (let i = 1; i < pts.length - 1; i++) {
      const mx = (pts[i].x + pts[i + 1].x) / 2
      const my = (pts[i].y + pts[i + 1].y) / 2
      d += ` Q${pts[i].x},${pts[i].y} ${mx},${my}`
    }
    const last = pts[pts.length - 1]
    return d + ` L${last.x},${last.y}`
  }

  // --- pan and zoom ---
  let host = $state<HTMLDivElement>()
  let box = $state({ w: 0, h: 0 })
  let view = $state({ x: 0, y: 0, k: 1 })
  let drag: { x: number; y: number; vx: number; vy: number } | null = null

  // Fit the graph, but never below a zoom where labels stay readable:
  // a big graph then starts at its top, and the rest is a drag away.
  const READABLE = 0.8 // 12px names stay near 10px
  // Whether the user has panned or zoomed since the last fit: a resize
  // refits only a view nobody has arranged.
  let moved = false
  function fit() {
    if (!host || !laid.w) return
    const r = host.getBoundingClientRect()
    if (!r.width || !r.height) return // hidden: fitted when it shows
    moved = false
    const k = Math.min(1.2, Math.max(READABLE, Math.min(r.width / laid.w, r.height / laid.h) * 0.95))
    const x = laid.w * k < r.width ? (r.width - laid.w * k) / 2 : 8
    const y = laid.h * k < r.height ? (r.height - laid.h * k) / 2 : 8
    view = { k, x, y }
  }
  // A new layout: fit it, unless it is the same graph changed a little
  // (a what-if value typed, a file saved) and the user has arranged the
  // view: then the view stays where it was (3.10.2).
  let before = new Set<string>()
  $effect(() => {
    void laid
    const now = new Set(nodes.map((n) => n.id))
    let kept = 0
    for (const idOf of now) if (before.has(idOf)) kept++
    const same = before.size > 0 && kept >= Math.max(before.size, now.size) / 2
    before = now
    if (!(same && moved)) queueMicrotask(fit)
  })
  // The box changes size (a pane dragged, the window, a hidden tab
  // shown): fit again, unless the user has moved the view.
  $effect(() => {
    if (!host) return
    let seen = false
    const ro = new ResizeObserver(() => {
      const r = host!.getBoundingClientRect()
      box = { w: r.width, h: r.height }
      const shown = r.width > 0 && r.height > 0
      if (shown && (!moved || !seen)) fit()
      seen = seen || shown
    })
    ro.observe(host)
    return () => ro.disconnect()
  })

  function zoom(by: number, cx?: number, cy?: number) {
    const r = host?.getBoundingClientRect()
    const x = cx ?? (r ? r.width / 2 : 0)
    const y = cy ?? (r ? r.height / 2 : 0)
    const k = Math.min(3, Math.max(0.15, view.k * by))
    moved = true
    view = { k, x: x - ((x - view.x) * k) / view.k, y: y - ((y - view.y) * k) / view.k }
  }
  function pan(dx: number, dy: number) {
    moved = true
    view = { ...view, x: view.x + dx, y: view.y + dy }
  }
  function onwheel(e: WheelEvent) {
    e.preventDefault()
    const r = host!.getBoundingClientRect()
    zoom(e.deltaY < 0 ? 1.12 : 1 / 1.12, e.clientX - r.left, e.clientY - r.top)
  }
  function onpointerdown(e: PointerEvent) {
    if ((e.target as Element).closest('.node, .controls, .legend, .minimap')) return
    drag = { x: e.clientX, y: e.clientY, vx: view.x, vy: view.y }
    ;(e.currentTarget as Element).setPointerCapture(e.pointerId)
  }
  function onpointermove(e: PointerEvent) {
    if (!drag) return
    moved = true
    view = { ...view, x: drag.vx + e.clientX - drag.x, y: drag.vy + e.clientY - drag.y }
  }

  // --- keyboard ---
  /** The node nearest to `from` in a direction, by the layout. */
  function towards(from: string, dx: number, dy: number): string | null {
    const a = laid.pos.get(from)
    if (!a) return null
    let best: string | null = null
    let score = Infinity
    for (const [idOf, p] of laid.pos) {
      if (idOf === from) continue
      const along = (p.x - a.x) * dx + (p.y - a.y) * dy
      if (along <= 1) continue
      const across = Math.abs((p.x - a.x) * dy) + Math.abs((p.y - a.y) * dx)
      const s = along + across * 2.5
      if (s < score) [best, score] = [idOf, s]
    }
    return best
  }
  /** Bring a node into view, moving the view no more than needed. */
  function reveal(idOf: string) {
    const p = laid.pos.get(idOf)
    if (!p || !box.w) return
    const m = 24
    const left = (p.x - p.w / 2) * view.k + view.x
    const right = (p.x + p.w / 2) * view.k + view.x
    const top = (p.y - NODE_H / 2) * view.k + view.y
    const bottom = (p.y + NODE_H / 2) * view.k + view.y
    let dx = 0
    let dy = 0
    if (left < m) dx = m - left
    else if (right > box.w - m) dx = box.w - m - right
    if (top < m) dy = m - top
    else if (bottom > box.h - m) dy = box.h - m - bottom
    if (dx || dy) pan(dx, dy)
  }
  const arrows: Record<string, [number, number]> = { ArrowLeft: [-1, 0], ArrowRight: [1, 0], ArrowUp: [0, -1], ArrowDown: [0, 1] }
  function onkeydown(e: KeyboardEvent) {
    if (e.ctrlKey || e.metaKey || e.altKey) return
    const node = (e.target as Element).closest<SVGGElement>('.node')
    const dir = arrows[e.key]
    if (dir && node) {
      const next = towards(node.dataset.id!, dir[0], dir[1])
      if (next) {
        const el = host?.querySelector<SVGGElement>(`.node[data-id="${CSS.escape(next)}"]`)
        el?.focus({ preventScroll: true })
        reveal(next)
      }
    } else if (dir) pan(-dir[0] * 60, -dir[1] * 60)
    else if (e.key === '+' || e.key === '=') zoom(1.25)
    else if (e.key === '-' || e.key === '_') zoom(0.8)
    else if (e.key === '0') fit()
    else if (node && (e.key === 'Enter' || e.key === ' ')) onselect?.(node.dataset.id!)
    else return
    e.preventDefault()
    e.stopPropagation()
  }

  // --- hover highlighting ---
  let hover = $state<string | null>(null)
  const near = $derived.by(() => {
    if (!hover) return null
    const s = new Set([hover])
    for (const e of edges) {
      if (e.from === hover) s.add(e.to)
      if (e.to === hover) s.add(e.from)
    }
    return s
  })

  // --- minimap: the whole graph, with the part in view marked ---
  const MINI_W = 148
  const mini = $derived.by(() => {
    if (!laid.w || !box.w) return null
    const overflows = laid.w * view.k > box.w + 1 || laid.h * view.k > box.h + 1
    if (!overflows) return null
    const s = Math.min(MINI_W / laid.w, 90 / laid.h)
    return {
      s,
      w: laid.w * s,
      h: laid.h * s,
      // The box's part of the graph, in graph coordinates.
      vx: -view.x / view.k,
      vy: -view.y / view.k,
      vw: box.w / view.k,
      vh: box.h / view.k,
    }
  })
  function miniTo(e: PointerEvent) {
    if (!mini || !(e.buttons & 1)) return
    const r = (e.currentTarget as Element).getBoundingClientRect()
    const gx = (e.clientX - r.left) / mini.s
    const gy = (e.clientY - r.top) / mini.s
    moved = true
    view = { ...view, x: box.w / 2 - gx * view.k, y: box.h / 2 - gy * view.k }
  }

  // --- what the drawing uses, for the legend ---
  const statusWords: Record<NonNullable<GraphNode['status']>, string> = { failed: 'failed', changed: 'changed', ok: 'ok', rescued: 'rescued', never: 'never ran' }
  // A glyph per status, so a run reads without its colours.
  const glyphs: Record<NonNullable<GraphNode['status']>, string> = {
    failed: 'M-4,-4 L4,4 M4,-4 L-4,4',
    changed: 'M0,-5 L5,4 L-5,4 Z',
    ok: 'M-4.5,0 L-1.5,3.5 L4.5,-3.5',
    rescued: 'M4,-2 A4.5,4.5 0 1 0 4.5,1 M4.5,-5 L4.5,-1.5 L1,-1.5',
    never: 'M-4,0 L4,0',
  }
  const statuses = $derived([...new Set(nodes.map((n) => n.status).filter((s): s is NonNullable<GraphNode['status']> => !!s))])
  const anyDashed = $derived(edges.some((e) => e.dashed))
  const anyMissing = $derived(nodes.some((n) => n.missing))
</script>

<!-- svelte-ignore a11y_no_noninteractive_tabindex, a11y_no_noninteractive_element_interactions -->
<div
  class="graph"
  bind:this={host}
  role="group"
  aria-label="{label}. Arrow keys move between nodes, plus and minus zoom, 0 fits."
  tabindex="0"
  {onwheel}
  {onpointerdown}
  {onpointermove}
  {onkeydown}
  onpointerup={() => (drag = null)}
  onpointercancel={() => (drag = null)}
>
  <svg width="100%" height="100%">
    <defs>
      <marker id={arrow} viewBox="0 0 10 10" refX="9" refY="5" markerWidth="7" markerHeight="7" orient="auto-start-reverse">
        <path d="M0,0 L10,5 L0,10 z" class="arrowhead" />
      </marker>
    </defs>
    <g transform="translate({view.x},{view.y}) scale({view.k})">
      {#each laid.bands as b (b.name)}
        <g class="lane">
          <rect x={b.x} y={b.y} width={b.w} height={b.h} rx="10" />
          <text x={b.x + 10} y={b.y - 8}>{cut(b.name, b.w + 20, 6.4)}<title>{b.name}</title></text>
        </g>
      {/each}
      {#each laid.paths as p, i (i)}
        {@const lit = hover && (p.e.from === hover || p.e.to === hover)}
        <path d={p.d} class="edge" class:lit class:dashed={p.e.dashed} class:dim={hover && !lit} marker-end="url(#{arrow})" />
        {#if lit && p.e.label && p.mid}
          <text x={p.mid.x} y={p.mid.y - 4} class="edge-label">{p.e.label}</text>
        {/if}
      {/each}
      {#each nodes as n (n.id)}
        {@const p = laid.pos.get(n.id)}
        {#if p}
          <!-- Keys are handled on the graph (onkeydown above): Enter and
               Space select the focused node. -->
          <!-- svelte-ignore a11y_click_events_have_key_events -->
          <g
            class="node g-{n.group ?? 'other'} {n.status ? 's-' + n.status : ''}"
            class:missing={n.missing}
            class:selected={n.id === selected}
            aria-pressed={selected === null ? undefined : n.id === selected}
            class:dim={near && !near.has(n.id)}
            transform="translate({p.x - p.w / 2},{p.y - NODE_H / 2})"
            role="button"
            tabindex="0"
            data-id={n.id}
            aria-label="{n.sub ?? ''} {n.label}{n.status ? ', ' + statusWords[n.status] : ''}{n.missing ? ', ' + missingNode : ''}"
            onpointerenter={() => (hover = n.id)}
            onpointerleave={() => (hover = null)}
            onfocus={() => (hover = n.id)}
            onblur={() => (hover = null)}
            onclick={() => onselect?.(n.id)}
          >
            <rect width={p.w} height={NODE_H} rx="8" />
            <rect class="stripe" width="4" height={NODE_H - 12} x="6" y="6" rx="2" />
            {#if n.sub}<text x="18" y="18" class="sub">{cut(n.sub, p.w - (n.status ? 16 : 0), 5.6)}</text>{/if}
            <text x="18" y={n.sub ? 35 : 28} class="name">{cut(n.label, p.w - (n.status ? 16 : 0))}</text>
            {#if n.status}<path class="glyph" d={glyphs[n.status]} transform="translate({p.w - 15},{NODE_H / 2})" />{/if}
            <!-- Last, so the box stays the group's first child (its styles
                 say rect:first-child): the whole text, as a tooltip. -->
            <title>{n.sub ? n.sub + '\n' : ''}{n.label}{n.status ? '\n' + statusWords[n.status] : ''}</title>
          </g>
        {/if}
      {/each}
    </g>
  </svg>

  <div class="controls">
    <IconButton icon={Plus} label="Zoom in" shortcut="+" size="sm" onclick={() => zoom(1.25)} />
    <IconButton icon={Minus} label="Zoom out" shortcut="-" size="sm" onclick={() => zoom(0.8)} />
    <IconButton icon={Maximize} label="Fit to view" shortcut="0" size="sm" onclick={fit} />
  </div>
  {#if legend.length || anyDashed || anyMissing || statuses.length}
    <div class="legend" class:beside={!!mini}>
      {#each legend as l (l.group)}
        <span class="g-{l.group}"><i></i>{l.label}</span>
      {/each}
      {#each statuses.filter((s) => !legend.some((l) => l.group === 's-' + s)) as s (s)}
        <span class="g-s-{s}"><i></i>{statusWords[s]}</span>
      {/each}
      {#if anyMissing}<span><i class="dashed-box"></i>{missingNode}</span>{/if}
      {#if anyDashed}<span><i class="dashed-line"></i>{dashedEdge}</span>{/if}
    </div>
  {/if}
  {#if mini}
    <!-- svelte-ignore a11y_no_static_element_interactions -->
    <div class="minimap" style:width="{mini.w}px" style:height="{mini.h}px" onpointerdown={miniTo} onpointermove={miniTo} aria-hidden="true">
      <svg width={mini.w} height={mini.h} viewBox="0 0 {laid.w} {laid.h}">
        {#each nodes as n (n.id)}
          {@const p = laid.pos.get(n.id)}
          {#if p}<rect class="mnode g-{n.group ?? 'other'} {n.status ? 's-' + n.status : ''}" class:on={n.id === selected} x={p.x - p.w / 2} y={p.y - NODE_H / 2} width={p.w} height={NODE_H} rx="6" />{/if}
        {/each}
        <rect class="mview" x={mini.vx} y={mini.vy} width={mini.vw} height={mini.vh} rx="4" />
      </svg>
    </div>
  {/if}
</div>

<style>
  .graph {
    position: relative;
    width: 100%;
    height: 100%;
    overflow: hidden;
    cursor: grab;
    background: var(--bg-0);
    touch-action: none;
  }
  .graph:active {
    cursor: grabbing;
  }
  .graph:focus-visible {
    outline: 2px solid var(--tool-accent, var(--accent));
    outline-offset: -2px;
  }
  /* A lane: a quiet band behind the nodes of one stage, named above it. */
  .lane rect {
    fill: var(--bg-1);
    stroke: var(--border);
    stroke-dasharray: 2 4;
  }
  .lane text {
    font-size: 11px;
    font-weight: 600;
    letter-spacing: 0.06em;
    text-transform: uppercase;
    fill: var(--fg-2);
  }
  /* A run's status as a glyph, in the status colour. */
  .glyph {
    fill: none;
    stroke: var(--c);
    stroke-width: 1.8;
    stroke-linecap: round;
    stroke-linejoin: round;
  }
  .node:focus-visible {
    outline: none;
  }
  .node:focus-visible rect:first-child {
    stroke: var(--tool-accent, var(--accent));
    stroke-width: 2;
  }
  svg {
    display: block;
    user-select: none;
  }
  .edge {
    fill: none;
    stroke: var(--border-strong);
    stroke-width: 1.4;
    transition: opacity 0.12s;
  }
  .edge.lit {
    stroke: var(--accent);
    stroke-width: 2;
  }
  .edge.dim {
    opacity: 0.2;
  }
  .edge.dashed {
    stroke-dasharray: 5 4;
  }
  .arrowhead {
    fill: var(--fg-2);
  }
  .edge-label {
    font-size: 10px;
    fill: var(--fg-1);
    text-anchor: middle;
    paint-order: stroke;
    stroke: var(--bg-0);
    stroke-width: 3px;
  }
  .node {
    cursor: pointer;
    transition: opacity 0.12s;
    --c: var(--fg-2);
  }
  .node rect:first-child {
    fill: var(--bg-2);
    stroke: var(--border-strong);
  }
  .node:hover rect:first-child,
  .node:focus-visible rect:first-child {
    stroke: var(--c);
  }
  .node .stripe {
    fill: var(--c);
  }
  /* The node whose details are showing: a firm outline in its colour. */
  .node.selected rect:first-child {
    fill: var(--bg-3);
    stroke: var(--c);
    stroke-width: 2;
  }
  .node.missing rect:first-child {
    fill: var(--bg-1);
    stroke-dasharray: 4 3;
  }
  .node.missing text {
    fill: var(--fg-2);
  }
  .node.dim {
    opacity: 0.3;
  }
  .sub {
    font-size: 10px;
    fill: var(--fg-2);
  }
  .name {
    font-size: 12px;
    font-weight: 600;
    fill: var(--fg-0);
  }
  .g-workload {
    --c: var(--accent);
  }
  .g-network {
    --c: var(--info);
  }
  .g-config {
    --c: var(--warn);
  }
  .g-storage {
    --c: var(--ok);
  }
  .g-rbac {
    --c: var(--syn-bool);
  }
  .g-scaling {
    --c: var(--syn-anchor);
  }
  .g-play {
    --c: var(--accent);
  }
  .g-role {
    --c: var(--info);
  }
  .g-tasks {
    --c: var(--syn-bool);
  }
  .g-handler {
    --c: var(--syn-anchor);
  }
  /* Run colours: what a loaded log did on each node. */
  .s-failed,
  .g-s-failed {
    --c: var(--err);
  }
  .s-changed,
  .g-s-changed {
    --c: var(--warn);
  }
  .s-ok,
  .g-s-ok {
    --c: var(--ok);
  }
  .s-rescued,
  .g-s-rescued {
    --c: var(--accent);
  }
  .s-never,
  .g-s-never {
    --c: var(--fg-2);
  }
  .node.s-failed rect:first-child {
    fill: var(--err-soft);
    stroke: var(--err);
  }
  .node.s-changed rect:first-child {
    stroke: var(--warn);
  }
  .node.s-never {
    opacity: 0.55;
  }
  .node.s-never rect:first-child {
    stroke-dasharray: 2 3;
  }
  .controls {
    position: absolute;
    right: var(--s-2);
    top: var(--s-2);
    display: flex;
    gap: 2px;
    padding: 2px;
    background: var(--bg-1);
    border: 1px solid var(--border);
    border-radius: var(--r-md);
  }
  .legend {
    position: absolute;
    left: var(--s-2);
    bottom: var(--s-2);
    display: flex;
    flex-wrap: wrap;
    gap: var(--s-3);
    padding: var(--s-1) var(--s-2);
    font-size: var(--fs-xs);
    color: var(--fg-2);
    background: var(--bg-1);
    border: 1px solid var(--border);
    border-radius: var(--r-md);
  }
  /* Leaves the corner to the minimap. */
  .legend.beside {
    max-width: calc(100% - 180px);
  }
  .legend span {
    display: inline-flex;
    align-items: center;
    gap: 4px;
  }
  .legend i {
    width: 8px;
    height: 8px;
    border-radius: 2px;
    background: var(--c, var(--fg-2));
  }
  .legend .dashed-box {
    width: 12px;
    background: none;
    border: 1px dashed var(--fg-2);
  }
  .legend .dashed-line {
    width: 14px;
    height: 0;
    background: none;
    border-top: 1px dashed var(--fg-2);
    border-radius: 0;
  }
  /* The whole graph in a corner, when it is bigger than its box. */
  .minimap {
    position: absolute;
    right: var(--s-2);
    bottom: var(--s-2);
    padding: 0;
    background: var(--bg-1);
    border: 1px solid var(--border);
    border-radius: var(--r-md);
    overflow: hidden;
    cursor: pointer;
  }
  .minimap svg {
    display: block;
  }
  .mnode {
    fill: var(--c, var(--fg-2));
    opacity: 0.55;
  }
  .mnode.on {
    opacity: 1;
  }
  .mview {
    fill: var(--accent-soft);
    stroke: var(--tool-accent, var(--accent));
    stroke-width: 1.5;
    vector-effect: non-scaling-stroke;
  }
</style>
