<script lang="ts">
  import ArrowDown from '@lucide/svelte/icons/arrow-down'
  import ArrowUp from '@lucide/svelte/icons/arrow-up'
  import type { TableColumn } from '../../lib/api/table'
  import { copyText } from '../../lib/utils/clipboard'

  // Rows of a table, only those in view drawn (fixed row height), with a
  // sticky header to sort by and the row numbers kept on the left. The
  // filter matches any cell, ignoring case. A double click copies a cell.
  interface Props {
    columns: TableColumn[]
    rows: string[][]
    filter: string
    /** The rows shown after filtering, for the caller's counts. */
    shown?: number
  }

  let { columns, rows, filter, shown = $bindable(0) }: Props = $props()

  const RH = 28 // px per row
  const OVERSCAN = 12

  let sortBy = $state(-1)
  let desc = $state(false)
  $effect(() => {
    void columns
    sortBy = -1
  })

  function sortOn(c: number) {
    if (sortBy !== c) [sortBy, desc] = [c, false]
    else if (!desc) desc = true
    else sortBy = -1
  }

  const collator = new Intl.Collator(undefined, { numeric: true, sensitivity: 'base' })

  // Indexes into rows: filtered, then sorted. Empty cells sort last.
  const order = $derived.by(() => {
    const q = filter.trim().toLowerCase()
    let idx = rows.map((_, i) => i)
    if (q) idx = idx.filter((i) => rows[i].some((v) => v.toLowerCase().includes(q)))
    const c = sortBy
    if (c >= 0 && c < columns.length) {
      const numeric = columns[c].type === 'integer' || columns[c].type === 'number'
      const dir = desc ? -1 : 1
      idx.sort((a, b) => {
        const x = rows[a][c].trim()
        const y = rows[b][c].trim()
        if (!x || !y) return x ? -1 : y ? 1 : a - b
        const d = numeric ? Number(x) - Number(y) : collator.compare(x, y)
        return d ? d * dir : a - b
      })
    }
    return idx
  })
  $effect(() => {
    shown = order.length
  })

  // Column widths in characters from the header (name, type and the
  // sort arrow) and the first rows, plus room for the padding.
  const widths = $derived(
    columns.map((col, c) => {
      let w = Math.min(col.name.length, 32) + col.type.length + 3
      for (let i = 0; i < Math.min(rows.length, 200); i++) w = Math.max(w, Math.min(rows[i][c].length, 48))
      return Math.max(6, Math.min(w, 48)) + 3
    }),
  )
  const numWidth = $derived(String(rows.length).length + 3)
  const template = $derived(`${numWidth}ch ${widths.map((w) => `${w}ch`).join(' ')}`)

  let scroller = $state<HTMLDivElement>()
  let top = $state(0)
  let height = $state(400)
  $effect(() => {
    void order
    if (scroller) scroller.scrollTop = Math.min(scroller.scrollTop, order.length * RH)
  })
  const first = $derived(Math.max(0, Math.floor(top / RH) - OVERSCAN))
  const last = $derived(Math.min(order.length, Math.ceil((top + height) / RH) + OVERSCAN))
  const visible = $derived(order.slice(first, last))

  const typeLabel = (c: TableColumn) => (c.type === 'empty' ? 'no values' : c.type)
  const about = (c: TableColumn) =>
    `${c.name}: ${typeLabel(c)} · ${c.filled.toLocaleString()} filled · ${c.empty.toLocaleString()} empty · ${c.distinct.toLocaleString()}${c.distinctMore ? '+' : ''} distinct`
</script>

<div class="scroll" bind:this={scroller} bind:clientHeight={height} onscroll={() => (top = scroller!.scrollTop)} role="table" aria-label="Rows" aria-rowcount={order.length + 1}>
  <div class="inner" style:--cols={template}>
    <div class="row head" role="row">
      <span class="num" role="columnheader">#</span>
      {#each columns as col, c (c)}
        <button
          type="button"
          class="th"
          class:sorted={sortBy === c}
          role="columnheader"
          aria-sort={sortBy !== c ? 'none' : desc ? 'descending' : 'ascending'}
          title="{about(col)} · click to sort"
          onclick={() => sortOn(c)}
        >
          <span class="name">{col.name}</span>
          <span class="type {col.type}">{col.type}</span>
          {#if sortBy === c}
            {#if desc}<ArrowDown size={12} strokeWidth={2} />{:else}<ArrowUp size={12} strokeWidth={2} />{/if}
          {/if}
        </button>
      {/each}
    </div>
    <div class="rows" style:height="{order.length * RH}px">
      {#each visible as r, j (r)}
        <div class="row" role="row" style:top="{(first + j) * RH}px">
          <span class="num" role="rowheader">{r + 1}</span>
          {#each rows[r] as v, c (c)}
            <!-- svelte-ignore a11y_interactive_supports_focus -->
            <span class="td {columns[c]?.type}" class:blank={!v} role="cell" title={v.length > widths[c] - 3 || v.includes('\n') ? v : undefined} ondblclick={() => copyText(v, 'Cell')}>{v.replaceAll('\n', ' ⏎ ')}</span>
          {/each}
        </div>
      {/each}
    </div>
  </div>
</div>

<style>
  .scroll {
    flex: 1;
    min-height: 0;
    overflow: auto;
    font-family: var(--font-mono);
    font-size: var(--fs-sm);
  }
  .inner {
    width: max-content;
    min-width: 100%;
  }
  .row {
    display: grid;
    grid-template-columns: var(--cols);
    height: 28px;
    align-items: center;
    border-bottom: 1px solid var(--border);
  }
  .rows {
    position: relative;
  }
  .rows .row {
    position: absolute;
    left: 0;
    right: 0;
  }
  .rows .row:hover {
    background: var(--bg-3);
  }
  .head {
    position: sticky;
    top: 0;
    z-index: 2;
    background: var(--bg-1);
    border-bottom-color: var(--border-strong);
  }
  .num {
    position: sticky;
    left: 0;
    z-index: 1;
    height: 100%;
    display: flex;
    align-items: center;
    justify-content: flex-end;
    padding: 0 var(--s-2);
    color: var(--fg-3);
    background: var(--bg-1);
    border-right: 1px solid var(--border);
  }
  .th,
  .td {
    height: 100%;
    min-width: 0;
    padding: 0 var(--s-2);
    overflow: hidden;
    white-space: nowrap;
    text-overflow: ellipsis;
    border-right: 1px solid var(--border);
  }
  .td {
    line-height: 28px;
    color: var(--fg-0);
  }
  .th {
    display: flex;
    align-items: center;
    gap: var(--s-1);
    font: inherit;
    font-weight: var(--fw-semibold);
    text-align: left;
    color: var(--fg-0);
    background: none;
    border-top: none;
    border-bottom: none;
    border-left: none;
  }
  .th:hover,
  .th.sorted {
    background: var(--bg-3);
  }
  .name {
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .type {
    flex: none;
    font-size: var(--fs-xs);
    font-weight: var(--fw-regular, 400);
    color: var(--fg-3);
  }
  .type.integer,
  .type.number,
  .td.integer,
  .td.number {
    color: var(--syn-number);
  }
  .type.boolean,
  .td.boolean {
    color: var(--syn-bool);
  }
  .type.date,
  .td.date {
    color: var(--syn-string);
  }
  .td.integer,
  .td.number {
    text-align: right;
  }
  .td.blank {
    background: color-mix(in srgb, var(--bg-3) 50%, transparent);
  }
</style>
