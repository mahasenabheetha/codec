<script lang="ts">
  import X from '@lucide/svelte/icons/x'
  import { levels, type Histogram } from '../../lib/api/logs'
  import { count, formatTime, levelTone } from './format'
  import type { TimeMode } from './viewer.svelte'

  // Entries over time, stacked by level (errors on top, where the eye
  // goes). Drag across it to keep only that time range.
  //
  // 3.10.2:
  //  - a warning or an error is never lost under a tall bar of info:
  //    its part of a bar is at least a few pixels high;
  //  - the scale is named (the tallest bar's count) and the time axis
  //    has ticks between its ends;
  //  - with a range applied, the chart still shows the whole log
  //    (`overview`), the range as a window over it and the rest dimmed:
  //    drag again for another range, or clear it with the ×.
  interface Props {
    histogram: Histogram
    /** The whole log's histogram, kept from before a range was applied. */
    overview?: Histogram | null
    /** The range applied, if any (ISO times). */
    range?: { from?: string; to?: string }
    mode: TimeMode
    start: number // the log's first time, for relative times
    onrange: (from: string, to: string) => void
    onclear?: () => void
  }

  let { histogram, overview = null, range = {}, mode, start, onrange, onclear }: Props = $props()

  const H = 64
  const FLOOR = 3 // px: the least a warning's or an error's part of a bar gets
  let width = $state(600)
  let el = $state<HTMLDivElement>()
  $effect(() => {
    if (!el) return
    const ro = new ResizeObserver(() => (width = el!.clientWidth))
    ro.observe(el)
    return () => ro.disconnect()
  })

  const ranged = $derived(!!range.from || !!range.to)
  // What is drawn: the whole log when a range is applied and its
  // histogram was kept; else the answer to the current query.
  const hist = $derived(ranged && overview?.buckets.length ? overview : histogram)
  const windowed = $derived(ranged && hist === overview)

  const n = $derived(hist.buckets.length)
  const t0 = $derived(hist.from ? new Date(hist.from).getTime() : 0)
  const totals = $derived(hist.buckets.map((b) => b.reduce((a, c) => a + c, 0)))
  const peak = $derived(Math.max(1, ...totals))
  const bw = $derived(n ? width / n : 0)
  const serious = (lv: string) => lv === 'warn' || lv === 'error' || lv === 'fatal'
  // Draw order bottom to top: none, trace … fatal.
  const stack = $derived(
    hist.buckets.map((b) => {
      const hs = levels.map((lv, j) => {
        const h = (b[j] / peak) * (H - 2)
        return b[j] > 0 && serious(lv) ? Math.max(h, FLOOR) : h
      })
      // The floors may push a full bar past the top: scale it back.
      const sum = hs.reduce((a, c) => a + c, 0)
      const k = sum > H - 1 ? (H - 1) / sum : 1
      let y = H
      return levels.map((lv, j) => {
        const h = hs[j] * k
        y -= h
        return { lv, y, h }
      })
    }),
  )

  const bucketAt = (x: number) => Math.min(n - 1, Math.max(0, Math.floor(x / Math.max(bw, 1e-9))))
  const timeOf = (i: number) => new Date(t0 + i * hist.stepMs).toISOString()
  /** A time's place along the chart, in buckets (may fall outside). */
  const at = (iso: string | undefined, fallback: number) => (iso ? (new Date(iso).getTime() - t0) / hist.stepMs : fallback)
  const win = $derived.by(() => {
    if (!windowed) return null
    const lo = Math.max(0, Math.min(n, at(range.from, 0)))
    const hi = Math.max(lo, Math.min(n, at(range.to, n)))
    return { lo, hi }
  })

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
    const dragged = drag.a !== drag.b
    drag = null
    // A plain click is not a range (it used to filter to one bucket).
    if (dragged) onrange(timeOf(lo), timeOf(hi + 1))
  }

  const days = $derived(n * hist.stepMs > 86_400_000)
  const label = (i: number) => formatTime(timeOf(i), mode, start, days)
  const sel = $derived(drag ? { lo: Math.min(drag.a, drag.b), hi: Math.max(drag.a, drag.b) } : null)
  // Ticks between the ends: as many as fit, at whole buckets.
  const ticks = $derived.by(() => {
    const want = Math.max(0, Math.min(5, Math.floor(width / 170) - 1))
    const out: number[] = []
    for (let k = 1; k <= want; k++) {
      const i = Math.round((n * k) / (want + 1))
      if (i > 0 && i < n && !out.includes(i)) out.push(i)
    }
    return out
  })
</script>

<div class="chart" bind:this={el}>
  <div class="plot">
    <svg
      width="100%"
      height={H}
      role="img"
      aria-label="Entries over time, {count(totals.reduce((a, c) => a + c, 0))} in {n} steps, at most {count(peak)} in one; drag to pick a time range"
      onpointerdown={down}
      onpointermove={move}
      onpointerup={up}
      onpointercancel={() => (drag = null)}
      onpointerleave={() => (hover = null)}
    >
      {#each ticks as i (i)}<line class="grid" x1={i * bw} x2={i * bw} y1="0" y2={H} />{/each}
      {#each stack as bars, i (i)}
        {#each bars as b (b.lv)}
          {#if b.h > 0}
            <rect x={i * bw + 0.5} y={b.y} width={Math.max(bw - 1, 1)} height={b.h} fill={levelTone[b.lv]} />
            <!-- Fatal is an error with a cap, so the two are told apart
                 without a colour of their own. -->
            {#if b.lv === 'fatal'}<rect class="cap" x={i * bw + 0.5} y={b.y} width={Math.max(bw - 1, 1)} height="2" />{/if}
          {/if}
        {/each}
      {/each}
      {#if win}
        <rect class="out" x="0" y="0" width={win.lo * bw} height={H} />
        <rect class="out" x={win.hi * bw} y="0" width={Math.max(0, (n - win.hi) * bw)} height={H} />
        <rect class="win" x={win.lo * bw} y="0.5" width={Math.max(2, (win.hi - win.lo) * bw)} height={H - 1} />
      {/if}
      {#if sel}<rect class="sel" x={sel.lo * bw} y="0" width={(sel.hi - sel.lo + 1) * bw} height={H} />{/if}
      {#if hover !== null && !drag}<rect class="hover" x={hover * bw} y="0" width={Math.max(bw, 1)} height={H} />{/if}
    </svg>
    <span class="peak" title="The tallest bar">{count(peak)}</span>
    {#if win && onclear}
      <button type="button" class="clear" style:left="{Math.min(width - 22, win.hi * bw + 2)}px" aria-label="Show the whole time range again" title="Show the whole time range again" onclick={onclear}>
        <X size={11} strokeWidth={2.5} />
      </button>
    {/if}
  </div>
  <div class="axis">
    {#if n}
      <span>{label(0)}</span>
      {#if hover !== null}
        {@const b = hist.buckets[hover]}
        <span class="tip">
          {sel ? `${label(sel.lo)} – ${label(sel.hi + 1)}` : label(hover)} · {count(totals[hover])}
          {#if b[5] + b[6]}<b class="err">· {count(b[5] + b[6])} errors</b>{/if}
          {#if b[4]}<b class="warn">· {count(b[4])} warnings</b>{/if}
        </span>
      {:else}
        <span class="ticks" aria-hidden="true">
          {#each ticks as i (i)}<span style:left="{(i / n) * 100}%">{label(i)}</span>{/each}
        </span>
      {/if}
      <span>{label(n)}</span>
    {/if}
  </div>
  <p class="hint">{windowed ? 'The whole log, with the range you picked; drag for another, × for all of it' : 'Drag to pick a time range'}</p>
</div>

<style>
  .chart {
    padding: var(--s-2) var(--s-3) var(--s-1);
    background: var(--bg-1);
    border: 1px solid var(--border);
    border-radius: var(--r-md);
  }
  .plot {
    position: relative;
  }
  svg {
    display: block;
    cursor: crosshair;
    touch-action: none;
  }
  .grid {
    stroke: var(--border);
    stroke-dasharray: 2 3;
  }
  .cap {
    fill: var(--fg-0);
  }
  .sel {
    fill: var(--accent-soft);
    stroke: var(--tool-accent, var(--accent));
  }
  .hover {
    fill: var(--accent-soft);
  }
  /* Outside the range picked: dimmed; the range itself: a frame. */
  .out {
    fill: var(--bg-1);
    opacity: 0.72;
  }
  .win {
    fill: none;
    stroke: var(--tool-accent, var(--accent));
    stroke-width: 1.5;
  }
  .peak {
    position: absolute;
    left: 0;
    top: 0;
    padding: 0 var(--s-1);
    font-size: var(--fs-xs);
    color: var(--fg-2);
    background: color-mix(in srgb, var(--bg-1) 80%, transparent);
    border-radius: var(--r-sm);
    pointer-events: none;
  }
  .clear {
    position: absolute;
    top: 2px;
    display: grid;
    place-items: center;
    width: 18px;
    height: 18px;
    padding: 0;
    color: var(--accent-fg);
    background: var(--tool-accent, var(--accent));
    border: none;
    border-radius: var(--r-full);
  }
  .axis {
    position: relative;
    display: flex;
    justify-content: space-between;
    gap: var(--s-3);
    margin-top: var(--s-1);
    font-family: var(--font-mono);
    font-size: var(--fs-xs);
    color: var(--fg-2);
  }
  /* Over the whole axis, so a tick's label sits under its line. */
  .ticks {
    position: absolute;
    left: 0;
    right: 0;
    top: 0;
  }
  .ticks span {
    position: absolute;
    transform: translateX(-50%);
    white-space: nowrap;
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
  .hint {
    margin: 2px 0 0;
    text-align: center;
    font-size: var(--fs-xs);
    color: var(--fg-2);
  }
</style>
