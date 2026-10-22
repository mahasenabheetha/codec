<script lang="ts">
  import { levels, type Histogram } from '../../lib/api/logs'
  import { count, formatTime, levelTone } from './format'
  import type { TimeMode } from './viewer.svelte'

  // Entries over time, stacked by level (errors on top, where the eye
  // goes). Drag across it to keep only that time range.
  interface Props {
    histogram: Histogram
    mode: TimeMode
    start: number // the log's first time, for relative times
    onrange: (from: string, to: string) => void
  }

  let { histogram, mode, start, onrange }: Props = $props()

  const H = 56
  let width = $state(600)
  let el = $state<HTMLDivElement>()
  $effect(() => {
    if (!el) return
    const ro = new ResizeObserver(() => (width = el!.clientWidth))
    ro.observe(el)
    return () => ro.disconnect()
  })

  const n = $derived(histogram.buckets.length)
  const t0 = $derived(histogram.from ? new Date(histogram.from).getTime() : 0)
  const totals = $derived(histogram.buckets.map((b) => b.reduce((a, c) => a + c, 0)))
  const peak = $derived(Math.max(1, ...totals))
  const bw = $derived(n ? width / n : 0)
  // Draw order bottom to top: none, trace … fatal.
  const stack = $derived(
    histogram.buckets.map((b) => {
      let y = H
      return levels.map((lv, j) => {
        const h = (b[j] / peak) * (H - 2)
        y -= h
        return { lv, y, h }
      })
    }),
  )

  const bucketAt = (x: number) => Math.min(n - 1, Math.max(0, Math.floor(x / Math.max(bw, 1e-9))))
  const timeOf = (i: number) => new Date(t0 + i * histogram.stepMs).toISOString()

  let hover = $state<number | null>(null)
  let drag = $state<{ a: number; b: number } | null>(null)

  function x(e: PointerEvent) {
    return e.clientX - (e.currentTarget as Element).getBoundingClientRect().left
  }
  function down(e: PointerEvent) {
    if (!n) return
    ;(e.currentTarget as Element).setPointerCapture(e.pointerId)
    const i = bucketAt(x(e))
    drag = { a: i, b: i }
  }
  function move(e: PointerEvent) {
    const i = bucketAt(x(e))
    hover = i
    if (drag) drag = { ...drag, b: i }
  }
  function up() {
    if (!drag) return
    const lo = Math.min(drag.a, drag.b)
    const hi = Math.max(drag.a, drag.b)
    drag = null
    onrange(timeOf(lo), timeOf(hi + 1))
  }

  const days = $derived(n * histogram.stepMs > 86_400_000)
  const label = (i: number) => formatTime(timeOf(i), mode, start, days)
  const sel = $derived(drag ? { lo: Math.min(drag.a, drag.b), hi: Math.max(drag.a, drag.b) } : null)
</script>

<div class="chart" bind:this={el}>
  <svg
    width="100%"
    height={H}
    role="img"
    aria-label="Entries over time; drag to pick a time range"
    onpointerdown={down}
    onpointermove={move}
    onpointerup={up}
    onpointerleave={() => (hover = null)}
  >
    {#each stack as bars, i (i)}
      {#each bars as b (b.lv)}
        {#if b.h > 0}<rect x={i * bw + 0.5} y={b.y} width={Math.max(bw - 1, 1)} height={b.h} fill={levelTone[b.lv]} />{/if}
      {/each}
    {/each}
    {#if sel}<rect class="sel" x={sel.lo * bw} y="0" width={(sel.hi - sel.lo + 1) * bw} height={H} />{/if}
    {#if hover !== null && !drag}<rect class="hover" x={hover * bw} y="0" width={Math.max(bw, 1)} height={H} />{/if}
  </svg>
  <div class="axis">
    {#if n}
      <span>{label(0)}</span>
      {#if hover !== null}
        {@const b = histogram.buckets[hover]}
        <span class="tip">
          {sel ? `${label(sel.lo)} – ${label(sel.hi + 1)}` : label(hover)} · {count(totals[hover])}
          {#if b[5] + b[6]}<b class="err">· {count(b[5] + b[6])} errors</b>{/if}
          {#if b[4]}<b class="warn">· {count(b[4])} warnings</b>{/if}
        </span>
      {:else}
        <span class="muted">drag to pick a time range</span>
      {/if}
      <span>{label(n)}</span>
    {/if}
  </div>
</div>

<style>
  .chart {
    padding: var(--s-2) var(--s-3) var(--s-1);
    background: var(--bg-1);
    border: 1px solid var(--border);
    border-radius: var(--r-md);
  }
  svg {
    display: block;
    cursor: crosshair;
    touch-action: none;
  }
  .sel {
    fill: var(--accent-soft);
    stroke: var(--accent);
  }
  .hover {
    fill: var(--accent-soft);
  }
  .axis {
    display: flex;
    justify-content: space-between;
    gap: var(--s-3);
    margin-top: var(--s-1);
    font-family: var(--font-mono);
    font-size: var(--fs-xs);
    color: var(--fg-2);
  }
  .tip {
    color: var(--fg-0);
  }
  .tip b {
    font-weight: var(--fw-medium);
  }
  .err {
    color: var(--err);
  }
  .warn {
    color: var(--warn);
  }
  .muted {
    font-family: var(--font-ui);
  }
</style>
