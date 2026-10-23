<script lang="ts">
  import Layers from '@lucide/svelte/icons/layers'
  import VirtualList from '../../lib/components/VirtualList.svelte'
  import { formatTime } from './format'
  import { marks, textTerms } from './terms'
  import { prefs, useViewer } from './viewer.svelte'

  // The matching entries, one row each: time, level, source, any field
  // columns, then the message. Cells wrap by default, so a row shows all
  // it has; on one line, the list scrolls sideways instead of cutting
  // text off. Words the query searches for are marked. Only rows in
  // view are drawn and only their pages fetched, so 100k entries
  // scroll like 100.
  interface Props {
    onselect: (i: number, pos: number) => void
  }

  let { onselect }: Props = $props()
  const viewer = useViewer()

  let list = $state<VirtualList>()
  const s = $derived(viewer.opened?.summary)
  const start = $derived(s?.from ? new Date(s.from).getTime() : 0)
  const days = $derived(!!s?.from && !!s?.to && new Date(s.to).getTime() - start > 86_400_000)
  const timed = $derived((s?.timed ?? 0) > 0)
  const many = $derived((s?.sources?.length ?? 0) > 1)
  const cols = $derived(viewer.columns)
  const wrap = $derived(prefs.lines.value === 'wrap')
  const terms = $derived(textTerms(viewer.applied.q))
  const timeW = $derived(prefs.time.value === 'relative' ? '92px' : days ? '150px' : '104px')
  const template = $derived(
    [
      ...(timed ? [timeW] : []),
      '52px',
      ...(many ? [wrap ? 'minmax(80px, 14%)' : '180px'] : []),
      ...cols.map(() => (wrap ? 'minmax(80px, 16%)' : '160px')),
      wrap ? 'minmax(0, 1fr)' : 'max-content',
    ].join(' '),
  )

  /** Bring a position in the matches into view. */
  export function reveal(pos: number) {
    list?.scrollToIndex(pos, 'center')
  }
</script>

{#snippet text(t: string)}
  {#each marks(t, terms) as m, j (j)}{#if m.hit}<mark>{m.t}</mark>{:else}{m.t}{/if}{/each}
{/snippet}

<div class="log-list" class:wrap style:--cols={template}>
  <div class="scroll" role="grid" aria-label="Log entries" aria-rowcount={viewer.total}>
    <div class="inner">
      <div class="head row" role="row">
        {#if timed}<span>Time</span>{/if}
        <span>Level</span>
        {#if many}<span>Source</span>{/if}
        {#each cols as c (c)}<span class="col" title={c}>{c}</span>{/each}
        <span>Message <span class="hint">· click a row for the whole entry</span></span>
      </div>
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
              class:sel={viewer.selected === r.i}
              role="row"
              onclick={() => onselect(r.i, pos)}
            >
              {#if timed}<span class="time">{formatTime(r.time, prefs.time.value, start, days)}</span>{/if}
              <span class="level">{r.level || ''}</span>
              {#if many}<span class="src" title={r.source}>{r.source ?? ''}</span>{/if}
              {#each cols as c (c)}<span class="col" title={r.cols?.[c]}>{@render text(r.cols?.[c] ?? '')}</span>{/each}
              <span class="msg">
                {@render text(r.message || r.error || '')}{#if r.more}<span class="more"> …</span>{/if}
                {#if r.stack}<Layers size={11} class="stack-icon" aria-label="has a stack trace" />{/if}
              </span>
            </button>
          {:else}
            <div class="row placeholder" aria-hidden="true"><span class="bar"></span></div>
          {/if}
        {/snippet}
      </VirtualList>
    </div>
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
  /* On one line the rows are as wide as their content and the list
     scrolls sideways; wrapped, they fit the list's width. */
  .inner {
    min-width: 100%;
    width: max-content;
  }
  .wrap .inner {
    width: 100%;
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
    position: sticky;
    top: 0;
    z-index: 1;
    font-family: var(--font-ui);
    font-size: var(--fs-xs);
    font-weight: var(--fw-semibold);
    color: var(--fg-2);
    background: var(--bg-1);
    border-bottom: 1px solid var(--border);
  }
  .hint {
    font-weight: var(--fw-regular);
  }
  .row > span {
    min-width: 0;
    white-space: nowrap;
  }
  /* One line: fixed columns cut with …, the message runs on. */
  .src,
  .col {
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .wrap .row > span {
    white-space: pre-wrap;
    overflow-wrap: anywhere;
  }
  .wrap .src,
  .wrap .col {
    overflow: visible;
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
  mark {
    color: inherit;
    background: var(--editor-search-match);
    border-radius: 2px;
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
