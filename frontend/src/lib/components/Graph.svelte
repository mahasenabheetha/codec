<script lang="ts" module>
  export interface GraphNode {
    id: string
    label: string
    sub?: string // small line above the label, e.g. the kind
    group?: string // colour group, see groupColor
    missing?: boolean // referenced but absent: drawn dashed
  }
  export interface GraphEdge {
    from: string
    to: string
    label?: string
  }
</script>

<script lang="ts">
  import { Graph as DagreGraph, layout } from '@dagrejs/dagre'
  import Maximize from '@lucide/svelte/icons/maximize'
  import Minus from '@lucide/svelte/icons/minus'
  import Plus from '@lucide/svelte/icons/plus'
  import IconButton from './IconButton.svelte'

  // A directed graph laid out left to right with dagre, drawn as SVG.
  // Drag to pan, wheel to zoom; hovering a node highlights its edges.
  // Shared by the Kubernetes, Argo and CI lenses.
  interface Props {
    nodes: GraphNode[]
    edges: GraphEdge[]
    label: string
    legend?: { group: string; label: string }[]
    onselect?: (id: string) => void
  }

  let { nodes, edges, label, legend = [], onselect }: Props = $props()

  const NODE_H = 46
  const width = (n: GraphNode) => Math.min(280, Math.max(120, Math.max(n.label.length, (n.sub ?? '').length) * 7.2 + 32))

  const laid = $derived.by(() => {
    const g = new DagreGraph({ multigraph: true })
    g.setGraph({ rankdir: 'LR', nodesep: 16, ranksep: 70, marginx: 20, marginy: 20 })
    g.setDefaultEdgeLabel(() => ({}))
    for (const n of nodes) g.setNode(n.id, { width: width(n), height: NODE_H })
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
    const gw = g.graph().width ?? 0
    const gh = g.graph().height ?? 0
    return { pos, paths, w: gw, h: gh }
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
  let view = $state({ x: 0, y: 0, k: 1 })
  let drag: { x: number; y: number; vx: number; vy: number } | null = null

  // Fit the graph, but never below a zoom where labels stay readable:
  // a big graph then starts at its top, and the rest is a drag away.
  const READABLE = 0.6
  function fit() {
    if (!host || !laid.w) return
    const r = host.getBoundingClientRect()
    const k = Math.min(1.2, Math.max(READABLE, Math.min(r.width / laid.w, r.height / laid.h) * 0.95))
    const x = laid.w * k < r.width ? (r.width - laid.w * k) / 2 : 8
    const y = laid.h * k < r.height ? (r.height - laid.h * k) / 2 : 8
    view = { k, x, y }
  }
  $effect(() => {
    void laid
    queueMicrotask(fit)
  })

  function zoom(by: number, cx?: number, cy?: number) {
    const r = host?.getBoundingClientRect()
    const x = cx ?? (r ? r.width / 2 : 0)
    const y = cy ?? (r ? r.height / 2 : 0)
    const k = Math.min(3, Math.max(0.15, view.k * by))
    view = { k, x: x - ((x - view.x) * k) / view.k, y: y - ((y - view.y) * k) / view.k }
  }
  function onwheel(e: WheelEvent) {
    e.preventDefault()
    const r = host!.getBoundingClientRect()
    zoom(e.deltaY < 0 ? 1.12 : 1 / 1.12, e.clientX - r.left, e.clientY - r.top)
  }
  function onpointerdown(e: PointerEvent) {
    if ((e.target as Element).closest('.node')) return
    drag = { x: e.clientX, y: e.clientY, vx: view.x, vy: view.y }
    ;(e.currentTarget as Element).setPointerCapture(e.pointerId)
  }
  function onpointermove(e: PointerEvent) {
    if (drag) view = { ...view, x: drag.vx + e.clientX - drag.x, y: drag.vy + e.clientY - drag.y }
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
</script>

<div
  class="graph"
  bind:this={host}
  role="img"
  aria-label={label}
  {onwheel}
  {onpointerdown}
  {onpointermove}
  onpointerup={() => (drag = null)}
>
  <svg width="100%" height="100%">
    <defs>
      <marker id="arrow" viewBox="0 0 10 10" refX="9" refY="5" markerWidth="7" markerHeight="7" orient="auto-start-reverse">
        <path d="M0,0 L10,5 L0,10 z" class="arrowhead" />
      </marker>
    </defs>
    <g transform="translate({view.x},{view.y}) scale({view.k})">
      {#each laid.paths as p, i (i)}
        {@const lit = hover && (p.e.from === hover || p.e.to === hover)}
        <path d={p.d} class="edge" class:lit class:dim={hover && !lit} marker-end="url(#arrow)" />
        {#if lit && p.e.label && p.mid}
          <text x={p.mid.x} y={p.mid.y - 4} class="edge-label">{p.e.label}</text>
        {/if}
      {/each}
      {#each nodes as n (n.id)}
        {@const p = laid.pos.get(n.id)}
        {#if p}
          <g
            class="node g-{n.group ?? 'other'}"
            class:missing={n.missing}
            class:dim={near && !near.has(n.id)}
            transform="translate({p.x - p.w / 2},{p.y - NODE_H / 2})"
            role="button"
            tabindex="0"
            aria-label="{n.sub ?? ''} {n.label}"
            onpointerenter={() => (hover = n.id)}
            onpointerleave={() => (hover = null)}
            onclick={() => onselect?.(n.id)}
            onkeydown={(e) => (e.key === 'Enter' || e.key === ' ') && onselect?.(n.id)}
          >
            <rect width={p.w} height={NODE_H} rx="7" />
            <rect class="stripe" width="4" height={NODE_H - 12} x="6" y="6" rx="2" />
            {#if n.sub}<text x="18" y="18" class="sub">{n.sub}</text>{/if}
            <text x="18" y={n.sub ? 35 : 28} class="name">{n.label.length > 34 ? n.label.slice(0, 33) + '…' : n.label}</text>
          </g>
        {/if}
      {/each}
    </g>
  </svg>

  <div class="controls">
    <IconButton icon={Plus} label="Zoom in" size="sm" onclick={() => zoom(1.25)} />
    <IconButton icon={Minus} label="Zoom out" size="sm" onclick={() => zoom(0.8)} />
    <IconButton icon={Maximize} label="Fit to view" size="sm" onclick={fit} />
  </div>
  {#if legend.length}
    <div class="legend">
      {#each legend as l (l.group)}
        <span class="g-{l.group}"><i></i>{l.label}</span>
      {/each}
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
</style>
