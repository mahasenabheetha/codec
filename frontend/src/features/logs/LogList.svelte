<script lang="ts">
  import Layers from '@lucide/svelte/icons/layers'
  import PinIcon from '@lucide/svelte/icons/pin'
  import VirtualList from '../../lib/components/VirtualList.svelte'
  import { markError, markPin, markRule, markWarn, type LogRow } from '../../lib/api/logs'
  import { count, firstRule, formatSpan, formatTime, ruleTones } from './format'
  import { marks, textTerms } from './terms'
  import { prefs, useViewer } from './viewer.svelte'

  // The view of the matching entries, one row each: time, level,
  // source, any field columns, then the message. Around matches,
  // context rows are dimmed and gaps say how much is skipped; folded
  // output and repeats are one row each, opened with a click. Rows a
  // highlight rule matches get its colour on the edge; the strip beside
  // the scrollbar shows where errors, highlights and pins are in the
  // whole list. Cells wrap by default; on one line, the list scrolls
  // sideways instead of cutting text off. Only rows in view are drawn
  // and only their pages fetched, so 100k entries scroll like 100.
  interface Props {
    onselect: (i: number, pos: number, last?: number) => void
  }

  let { onselect }: Props = $props()
  const viewer = useViewer()

  let list = $state<VirtualList>()
  let scroller = $state<HTMLDivElement>()
  const s = $derived(viewer.opened?.summary)
  const start = $derived(s?.from ? new Date(s.from).getTime() : 0)
  const days = $derived(!!s?.from && !!s?.to && new Date(s.to).getTime() - start > 86_400_000)
  const timed = $derived((s?.timed ?? 0) > 0)
  const many = $derived((s?.sources?.length ?? 0) > 1)
  // An Argo run names its steps: the column shows the step, the pod on hover.
  const steps = $derived(!!s?.steps?.length)
  const cols = $derived(viewer.columns)
  const wrap = $derived(prefs.lines.value === 'wrap')
  const terms = $derived(textTerms(viewer.applied.q))
  // A query or time range is on: only then is "matching" worth saying.
  const filtering = $derived(!!(viewer.applied.q || viewer.applied.from || viewer.applied.to))
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

  /** Bring a position in the view into view. */
  export function reveal(pos: number) {
    list?.scrollToIndex(pos, 'center')
  }

  function pick(r: LogRow, pos: number) {
    if (r.kind === 'output') viewer.toggleFold(r.i) // folded output is read by opening it
    onselect(r.i, pos, r.lastI ?? r.i)
  }

  const span = (r: LogRow) => (r.time && r.lastTime ? formatSpan(new Date(r.lastTime).getTime() - new Date(r.time).getTime()) : '')

  // Where the list is scrolled, for the box on the marker strip: the
  // rows on screen, as a share of all rows, the way the marks are placed
  // (by pixels, wrapped rows of other heights would put the box apart
  // from the marks of the rows in it).
  let view = $state({ top: 0, height: 1 })
  function onscroll() {
    const el = scroller
    if (!el || !viewer.rowCount) return
    const head = el.querySelector('.head')
    const top = head ? head.getBoundingClientRect().bottom : el.getBoundingClientRect().top // below the sticky header
    const bottom = el.getBoundingClientRect().bottom
    let first = -1
    let last = -1
    for (const r of el.querySelectorAll<HTMLElement>('[data-pos]')) {
      const b = r.getBoundingClientRect()
      if (b.bottom <= top || b.top >= bottom) continue
      const pos = Number(r.dataset.pos)
      if (first < 0) first = pos
      last = pos
    }
    if (first < 0) return
    view = { top: first / viewer.rowCount, height: (last - first + 1) / viewer.rowCount }
  }
  $effect(() => {
    void viewer.rowCount
    onscroll()
  })
  // A click on (or within a few pixels of) a mark goes to the row the
  // mark stands for, and selects it; one marker covers many rows of a
  // long list, so the clicked height alone lands rows away. Elsewhere,
  // the click goes to that height. The row is fetched before scrolling,
  // so its real height doesn't move the list after.
  async function jump(e: MouseEvent) {
    const box = (e.currentTarget as HTMLElement).getBoundingClientRect()
    const y = e.clientY - box.top
    const n = viewer.markers.length
    let target = -1
    for (let d = 0; d <= 4 && target < 0; d++) {
      for (const yy of [y - d, y + d]) {
        const b = Math.floor((yy / box.height) * n)
        if (b >= 0 && b < n && viewer.markers[b] && (viewer.markerRows[b] ?? -1) >= 0) {
          target = viewer.markerRows[b]
          break
        }
      }
    }
    const pos = target >= 0 ? target : Math.max(0, Math.min(viewer.rowCount - 1, Math.floor((y / box.height) * viewer.rowCount)))
    const r = await viewer.rowAsync(pos)
    await list?.scrollToIndex(pos, 'center')
    if (target >= 0 && r && r.kind !== 'gap') onselect(r.i, pos, r.lastI ?? r.i)
  }
  // A marker's colour: errors first, then pins, highlights, warnings.
  function tone(m: number): string {
    if (m & markError) return 'var(--err)'
    if (m & markPin) return 'var(--accent)'
    for (let k = 0; k < 4; k++) if (m & (markRule << k)) return ruleTones[k]
    if (m & markWarn) return 'var(--warn)'
    return ''
  }
</script>

{#snippet text(t: string)}
  {#each marks(t, terms) as m, j (j)}{#if m.hit}<mark>{m.t}</mark>{:else}{m.t}{/if}{/each}
{/snippet}

<div class="log-list" class:wrap style:--cols={template}>
  <div class="body">
    <div class="scroll" role="grid" aria-label="Log entries" aria-rowcount={viewer.rowCount} bind:this={scroller} {onscroll}>
      <div class="inner">
        <div class="head row" role="row">
          {#if timed}<span>Time</span>{/if}
          <span>Level</span>
          {#if many}<span>{steps ? 'Step' : 'Source'}</span>{/if}
          {#each cols as c (c)}<span class="col" title={c}>{c}</span>{/each}
          <span>Message <span class="hint">· click a row for the whole entry</span></span>
        </div>
        <VirtualList
          bind:this={list}
          count={viewer.rowCount}
          estimate={wrap ? 40 : 24}
          windowFrom={0}
          overscan={800}
          onrange={(a, b) => viewer.ensure(a, b)}
        >
          {#snippet row(pos)}
            {@const r = viewer.rowAt(pos)}
            {#if !r}
              <div class="row placeholder" aria-hidden="true" data-pos={pos}><span class="bar"></span></div>
            {:else if r.kind === 'gap'}
              <div class="row gap" role="row" data-pos={pos} title="Entries the query doesn't match, between the ones shown with their context">
                <span>⋯ {count(r.n ?? 0)} {r.n === 1 ? 'entry' : 'entries'}</span>
              </div>
            {:else}
              {@const rule = firstRule(r.hl)}
              <button
                type="button"
                class="row lv-{r.level || 'none'}"
                class:ctx={r.kind === 'context' || (!!r.n && r.n > 1 && !r.matches && filtering)}
                class:fold={r.kind === 'output'}
                class:sel={viewer.selectedPos === pos || (viewer.selectedPos === null && viewer.selected === r.i)}
                class:hl={rule >= 0}
                style:--hl={rule >= 0 ? ruleTones[rule] : null}
                role="row"
                data-pos={pos}
                onclick={() => pick(r, pos)}
              >
                {#if timed}<span class="time">{formatTime(r.time, prefs.time.value, start, days)}</span>{/if}
                <span class="level">{r.level || ''}</span>
                {#if many}<span class="src" title={r.source}>{(steps && r.step) || r.source || ''}</span>{/if}
                {#each cols as c (c)}<span class="col" title={r.cols?.[c]}>{@render text(r.cols?.[c] ?? '')}</span>{/each}
                {#if r.kind === 'output'}
                  <span class="msg">
                    <span class="badge" title="Plain output (text without a level) folded; click to show it, or press O"
                      >{`⋯ ${count(r.n ?? 0)} entries of output${r.matches && filtering ? `, ${r.matches} matching` : ''}${span(r) ? ` over ${span(r)}` : ''}`}</span
                    >
                    <span class="preview">{@render text((r.message || '').split('\n')[0])}</span>
                  </span>
                {:else}
                  <span class="msg">
                    <!-- svelte-ignore a11y_click_events_have_key_events, a11y_no_static_element_interactions -->
                    <span
                      class="pin"
                      class:on={viewer.isPinned(r.i)}
                      title={viewer.isPinned(r.i) ? 'Unpin (P)' : 'Pin this entry (P)'}
                      onclick={(e) => {
                        e.stopPropagation()
                        viewer.togglePin(r.i, r)
                      }}><PinIcon size={11} /></span
                    >
                    {#if r.kind === 'repeat'}
                      <!-- svelte-ignore a11y_click_events_have_key_events, a11y_no_static_element_interactions -->
                      <span
                        class="badge"
                        title="The same line {r.n} times{span(r) ? ' over ' + span(r) : ''}; click to show them all (or press O)"
                        onclick={(e) => {
                          e.stopPropagation()
                          viewer.toggleFold(r.i)
                        }}>×{count(r.n ?? 0)}</span
                      >
                    {:else if viewer.expanded.has(r.i)}
                      <!-- svelte-ignore a11y_click_events_have_key_events, a11y_no_static_element_interactions -->
                      <span
                        class="badge open"
                        title="Fold these lines again (or press O)"
                        onclick={(e) => {
                          e.stopPropagation()
                          viewer.toggleFold(r.i)
                        }}>fold</span
                      >
                    {/if}
                    {@render text(r.message || r.error || '')}{#if r.more}<span class="more"> …</span>{/if}
                    {#if r.stack}<Layers size={11} class="stack-icon" aria-label="has a stack trace" />{/if}
                  </span>
                {/if}
              </button>
            {/if}
          {/snippet}
        </VirtualList>
      </div>
    </div>
    {#if viewer.markers.length && viewer.rowCount > 0}
      <!-- svelte-ignore a11y_click_events_have_key_events, a11y_no_static_element_interactions -->
      <div class="marks" title="Where errors, warnings, highlights and pins are; click to go there" onclick={jump}>
        <div class="where" style:top="{view.top * 100}%" style:height="{Math.max(view.height * 100, 1)}%"></div>
        {#each viewer.markers as m, b (b)}
          {#if m}
            <i style:top="{(b / viewer.markers.length) * 100}%" style:background={tone(m)}></i>
          {/if}
        {/each}
      </div>
    {/if}
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
  .body {
    flex: 1;
    min-height: 0;
    display: flex;
  }
  .scroll {
    flex: 1;
    min-width: 0;
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
  button.row.hl {
    border-left-color: var(--hl);
    box-shadow: inset 3px 0 0 var(--hl);
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
  /* Around the matches: there, but not what was asked for. */
  .row.ctx {
    opacity: 0.6;
  }
  .row.ctx:hover,
  .row.ctx.sel {
    opacity: 1;
  }
  .gap {
    display: block;
    min-height: 0;
    padding: 1px var(--s-3);
    font-family: var(--font-ui);
    font-size: var(--fs-xs);
    color: var(--fg-2);
    background: var(--bg-0);
    border-top: 1px dashed var(--border);
    border-bottom: 1px dashed var(--border);
  }
  .badge {
    display: inline-block;
    margin-right: var(--s-2);
    padding: 0 var(--s-1);
    font-family: var(--font-ui);
    font-size: var(--fs-xs);
    color: var(--fg-1);
    background: var(--bg-3);
    border-radius: var(--r-sm);
    cursor: pointer;
  }
  .badge:hover {
    color: var(--fg-0);
    background: var(--bg-4);
  }
  .badge.open {
    color: var(--fg-2);
  }
  /* A pin: shown on a pinned row, offered on hover. */
  .pin {
    display: inline-grid;
    place-items: center;
    width: 16px;
    margin-right: var(--s-1);
    color: var(--fg-2);
    vertical-align: -1px;
    opacity: 0;
    cursor: pointer;
  }
  button.row:hover .pin,
  button.row.sel .pin {
    opacity: 0.7;
  }
  .pin:hover {
    color: var(--fg-0);
    opacity: 1 !important;
  }
  .pin.on {
    color: var(--accent);
    opacity: 1;
  }
  .fold .preview {
    color: var(--fg-2);
  }
  .wrap .fold .msg {
    white-space: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
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
  .marks {
    position: relative;
    flex: none;
    width: 12px;
    background: var(--bg-0);
    border-left: 1px solid var(--border);
    cursor: pointer;
  }
  .marks i {
    position: absolute;
    left: 2px;
    right: 2px;
    height: 3px;
    border-radius: 1px;
  }
  .where {
    position: absolute;
    left: 0;
    right: 0;
    min-height: 4px;
    background: var(--accent-soft);
    border-top: 1px solid var(--accent);
    border-bottom: 1px solid var(--accent);
    pointer-events: none;
  }
</style>
