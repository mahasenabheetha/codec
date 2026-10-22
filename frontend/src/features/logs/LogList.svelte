<script lang="ts">
  import Layers from '@lucide/svelte/icons/layers'
  import VirtualList from '../../lib/components/VirtualList.svelte'
  import { formatTime } from './format'
  import { prefs, viewer } from './viewer.svelte'

  // The matching entries, one row each: time, level, source, message
  // and any field columns. Only rows in view are drawn, and only the
  // pages they are on are fetched, so 100k entries scroll like 100.
  interface Props {
    onselect: (i: number) => void
  }

  let { onselect }: Props = $props()

  let list = $state<VirtualList>()
  const s = $derived(viewer.opened?.summary)
  const start = $derived(s?.from ? new Date(s.from).getTime() : 0)
  const days = $derived(!!s?.from && !!s?.to && new Date(s.to).getTime() - start > 86_400_000)
  const many = $derived((s?.sources?.length ?? 0) > 1)
  const cols = $derived(viewer.columns)
  const wrap = $derived(prefs.wrap.value)
  const template = $derived(
    ['var(--time-w)', '52px', ...(many ? ['minmax(80px, 14%)'] : []), 'minmax(0, 1fr)', ...cols.map(() => 'minmax(60px, 12%)')].join(' '),
  )

  /** Bring a position in the matches into view. */
  export function reveal(pos: number) {
    list?.scrollToIndex(pos, 'center')
  }
</script>

<div class="log-list" style:--cols={template} style:--time-w={prefs.time.value === 'relative' ? '92px' : days ? '150px' : '104px'}>
  <div class="head row" role="row">
    <span>Time</span>
    <span>Level</span>
    {#if many}<span>Source</span>{/if}
    <span>Message</span>
    {#each cols as c (c)}<span class="col" title={c}>{c}</span>{/each}
  </div>
  <div class="scroll" role="grid" aria-label="Log entries" aria-rowcount={viewer.total}>
    <VirtualList
      bind:this={list}
      count={viewer.total}
      estimate={wrap ? 40 : 24}
      windowFrom={0}
      overscan={800}
      onrange={(a, b) => viewer.ensure(a, b)}
    >
      {#snippet row(pos)}
        {@const r = viewer.rowAt(pos)}
        {#if r}
          <button
            type="button"
            class="row lv-{r.level || 'none'}"
            class:wrap
            class:sel={viewer.selected === r.i}
            role="row"
            onclick={() => onselect(r.i)}
          >
            <span class="time">{formatTime(r.time, prefs.time.value, start, days)}</span>
            <span class="level">{r.level || ''}</span>
            {#if many}<span class="src" title={r.source}>{r.source ?? ''}</span>{/if}
            <span class="msg">
              {r.message || r.error || ''}{#if r.more}<span class="more">…</span>{/if}
              {#if r.stack}<Layers size={11} class="stack-icon" aria-label="has a stack trace" />{/if}
            </span>
            {#each cols as c (c)}<span class="col" title={r.cols?.[c]}>{r.cols?.[c] ?? ''}</span>{/each}
          </button>
        {:else}
          <div class="row placeholder" aria-hidden="true"><span class="bar"></span></div>
        {/if}
      {/snippet}
    </VirtualList>
  </div>
</div>

<style>
  .log-list {
    height: 100%;
    display: flex;
    flex-direction: column;
    min-height: 0;
    background: var(--bg-1);
    border: 1px solid var(--border);
    border-radius: var(--r-md);
    overflow: hidden;
  }
  .scroll {
    flex: 1;
    min-height: 0;
    overflow: auto;
  }
  .row {
    display: grid;
    grid-template-columns: var(--cols);
    gap: var(--s-3);
    align-items: baseline;
    width: 100%;
    min-height: 24px;
    padding: 3px var(--s-3);
    font-family: var(--font-mono);
    font-size: var(--fs-sm);
    line-height: 18px;
    text-align: left;
    color: var(--fg-0);
    background: none;
    border: none;
    border-left: 2px solid transparent;
  }
  button.row:hover {
    background: var(--bg-2);
  }
  button.row.sel {
    background: var(--accent-soft);
    border-left-color: var(--accent);
  }
  .head {
    flex: none;
    font-family: var(--font-ui);
    font-size: var(--fs-xs);
    font-weight: var(--fw-semibold);
    color: var(--fg-2);
    border-bottom: 1px solid var(--border);
  }
  .row > span {
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .row.wrap .msg {
    white-space: pre-wrap;
    overflow-wrap: anywhere;
  }
  .time {
    color: var(--fg-2);
  }
  .level {
    font-size: var(--fs-xs);
    text-transform: uppercase;
    color: var(--fg-2);
  }
  .lv-fatal .level,
  .lv-error .level {
    color: var(--err);
    font-weight: var(--fw-semibold);
  }
  .lv-fatal,
  .lv-error {
    background: var(--err-soft);
  }
  .lv-warn .level {
    color: var(--warn);
    font-weight: var(--fw-semibold);
  }
  .lv-info .level {
    color: var(--info);
  }
  .lv-debug .msg,
  .lv-trace .msg {
    color: var(--fg-1);
  }
  .src,
  .col {
    color: var(--fg-1);
  }
  .more {
    color: var(--fg-2);
  }
  .msg :global(.stack-icon) {
    margin-left: var(--s-1);
    color: var(--fg-2);
    vertical-align: -1px;
  }
  .placeholder .bar {
    grid-column: 1 / -1;
    height: 8px;
    margin: 5px 0;
    border-radius: var(--r-sm);
    background: var(--bg-2);
  }
</style>
