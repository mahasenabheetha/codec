<script lang="ts">
  import { isAbort } from '../../lib/api/client'
  import { logLanes, type LogLanes } from '../../lib/api/logs'
  import { count, formatTime } from './format'
  import { term } from './terms'
  import { prefs, useViewer } from './viewer.svelte'

  // The matching entries over time, a lane per step of an Argo run, else
  // per source: when each logged (the busier, the darker), where its
  // warnings and errors fall. A cell goes to its entry (an error's, if it
  // has one); a lane's name shows only that lane.
  interface Props {
    start: number // the log's first time, for relative times
    onjump: (i: number) => void
  }

  let { start, onjump }: Props = $props()
  const viewer = useViewer()

  const BUCKETS = 120
  const ROW = 14
  let lanes = $state.raw<LogLanes | null>(null)
  let error = $state<string | null>(null)
  $effect(() => {
    const q = { ...viewer.applied }
    void viewer.id
    const c = new AbortController()
    viewer
      .withLog((id) => logLanes(id, { ...q, buckets: BUCKETS }, c.signal))
      .then((l) => {
        lanes = l
        error = null
      })
      .catch((e) => {
        if (!isAbort(e)) error = e instanceof Error ? e.message : String(e)
      })
    return () => c.abort()
  })

  let width = $state(600)
  let el = $state<HTMLDivElement>()
  $effect(() => {
    if (!el) return
    const ro = new ResizeObserver(() => (width = el!.clientWidth))
    ro.observe(el)
    return () => ro.disconnect()
  })

  const n = $derived(lanes?.n ?? 0)
  const bw = $derived(n ? width / n : 0)
  const peak = $derived(Math.max(1, ...(lanes?.lanes ?? []).flatMap((l) => l.cells.map((c) => c[1]))))
  const t0 = $derived(lanes?.from ? new Date(lanes.from).getTime() : 0)
  const timeOf = (i: number) => new Date(t0 + i * (lanes?.stepMs ?? 0)).toISOString()
  const days = $derived(n * (lanes?.stepMs ?? 0) > 86_400_000)
  const label = (i: number) => formatTime(timeOf(i), prefs.time.value, start, days)
  // Busier cells darker, on a square-root scale so quiet lanes still show.
  const shade = (c: number) => 0.25 + 0.75 * Math.sqrt(c / peak)

  let hover = $state<{ lane: number; cell: [number, number, number, number, number] } | null>(null)
</script>

<div class="lanes">
  {#if error}
    <p class="err">{error}</p>
  {:else if lanes && !lanes.lanes.length}
    <p class="muted">No {lanes.by === 'step' ? 'steps' : 'sources'} with times among the entries the query matches.</p>
  {:else if lanes}
    <div class="body" style:max-height="{Math.min(lanes.lanes.length, 12) * ROW + 4}px">
      <div class="names">
        {#each lanes.lanes as l (l.name)}
          <button
            type="button"
            class:failed={l.errors > 0}
            style:height="{ROW}px"
            title="{l.name} · {count(l.entries)} entries{l.errors ? ` · ${count(l.errors)} errors` : ''} — show only this {lanes.by}"
            onclick={() => viewer.follow(term(lanes!.by, l.name))}>{l.name}</button
          >
        {/each}
      </div>
      <div class="track" bind:this={el}>
        <svg width="100%" height={lanes.lanes.length * ROW} role="img" aria-label="Entries over time, a lane per {lanes.by}" onpointerleave={() => (hover = null)}>
          {#each lanes.lanes as l, k (l.name)}
            <rect class="lane" x="0" y={k * ROW + 1} width="100%" height={ROW - 2} />
            {#each l.cells as c (c[0])}
              <!-- svelte-ignore a11y_click_events_have_key_events, a11y_no_static_element_interactions -->
              <rect
                class="cell"
                class:e={c[3] > 0}
                class:w={!c[3] && c[2] > 0}
                x={c[0] * bw}
                y={k * ROW + 2}
                width={Math.max(bw - 0.5, 1.5)}
                height={ROW - 4}
                fill-opacity={c[3] || c[2] ? 1 : shade(c[1])}
                onpointerenter={() => (hover = { lane: k, cell: c })}
                onclick={() => onjump(c[4])}
              />
            {/each}
          {/each}
        </svg>
      </div>
    </div>
    <div class="foot"><span></span><div class="axis">
      <span>{label(0)}</span>
      {#if hover}
        {@const c = hover.cell}
        <span class="tip">
          {lanes.lanes[hover.lane].name} · {label(c[0])} · {count(c[1])}
          {#if c[3]}<b class="err">{` · ${count(c[3])} errors`}</b>{/if}
          {#if c[2]}<b class="warn">{` · ${count(c[2])} warnings`}</b>{/if}
        </span>
      {:else}
        <span class="muted">a lane per {lanes.by}{lanes.more ? ` · ${lanes.more} more not shown` : ''} · click a cell to go there</span>
      {/if}
      <span>{label(n)}</span>
    </div></div>
  {/if}
</div>

<style>
  .lanes {
    padding: var(--s-2) var(--s-3) var(--s-1);
    background: var(--bg-1);
    border: 1px solid var(--border);
    border-radius: var(--r-md);
  }
  .body {
    display: grid;
    grid-template-columns: minmax(80px, 180px) minmax(0, 1fr);
    gap: var(--s-2);
    overflow-y: auto;
  }
  .names {
    display: flex;
    flex-direction: column;
    min-width: 0;
  }
  .names button {
    flex: none;
    padding: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    font-family: var(--font-mono);
    font-size: var(--fs-xs);
    line-height: 14px;
    text-align: right;
    color: var(--fg-1);
    background: none;
    border: none;
  }
  .names button:hover {
    color: var(--fg-0);
    text-decoration: underline;
  }
  .names button.failed {
    color: var(--err);
  }
  svg {
    display: block;
  }
  .lane {
    fill: var(--bg-2);
  }
  .cell {
    fill: var(--accent);
    cursor: pointer;
  }
  .cell.w {
    fill: var(--warn);
  }
  .cell.e {
    fill: var(--err);
  }
  .foot {
    display: grid;
    grid-template-columns: minmax(80px, 180px) minmax(0, 1fr);
    gap: var(--s-2);
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
    margin: 0;
    font-family: var(--font-ui);
    font-size: var(--fs-xs);
    color: var(--fg-2);
  }
</style>
