<script module lang="ts">
  export interface Row<D> {
    id: string
    depth: number
    expandable: boolean
    expanded: boolean
    data: D
  }
</script>

<script lang="ts" generics="T">
  import type { Snippet } from 'svelte'
  import ChevronRight from '@lucide/svelte/icons/chevron-right'

  // Keyboard-navigable tree (WAI-ARIA tree pattern) over pre-flattened
  // rows: the caller decides which rows are visible, this component
  // renders them and handles focus, arrows and clicks.
  //   ↑/↓ move · → expand / first child · ← collapse / parent
  //   Enter/Space open or toggle · Home/End

  interface Props {
    rows: Row<T>[]
    label: string
    /** Highlighted row, e.g. the file in the active tab. */
    active?: string | null
    row: Snippet<[T, Row<T>]>
    ontoggle: (id: string) => void
    onopen: (data: T) => void
  }

  let { rows, label, active = null, row, ontoggle, onopen }: Props = $props()

  let focused = $state<string | null>(null)
  let list = $state<HTMLDivElement>()

  // The row that takes Tab focus: the keyboard cursor, else the active
  // row, else the first.
  const cursor = $derived.by(() => {
    if (focused && rows.some((r) => r.id === focused)) return focused
    if (active && rows.some((r) => r.id === active)) return active
    return rows[0]?.id ?? null
  })

  // Keep the active row in view when it changes (e.g. opened via Ctrl+P).
  $effect(() => {
    if (!active || !list) return
    const id = active
    queueMicrotask(() => rowEl(id)?.scrollIntoView({ block: 'nearest' }))
  })

  function rowEl(id: string): HTMLElement | null {
    return list?.querySelector<HTMLElement>(`[data-id="${CSS.escape(id)}"]`) ?? null
  }

  function focusRow(id: string | undefined) {
    if (!id) return
    focused = id
    const el = rowEl(id)
    el?.focus()
    el?.scrollIntoView({ block: 'nearest' })
  }

  function activate(r: Row<T>) {
    focused = r.id
    if (r.expandable) ontoggle(r.id)
    else onopen(r.data)
  }

  function onkeydown(e: KeyboardEvent) {
    const i = rows.findIndex((r) => r.id === cursor)
    if (i < 0) return
    const r = rows[i]
    switch (e.key) {
      case 'ArrowDown':
        focusRow(rows[i + 1]?.id)
        break
      case 'ArrowUp':
        focusRow(rows[i - 1]?.id)
        break
      case 'Home':
        focusRow(rows[0]?.id)
        break
      case 'End':
        focusRow(rows[rows.length - 1]?.id)
        break
      case 'ArrowRight':
        if (r.expandable && !r.expanded) ontoggle(r.id)
        else if (r.expandable) focusRow(rows[i + 1]?.id)
        break
      case 'ArrowLeft':
        if (r.expandable && r.expanded) ontoggle(r.id)
        else {
          // Jump to the parent: the nearest row above that is shallower.
          for (let j = i - 1; j >= 0; j--) {
            if (rows[j].depth < r.depth) {
              focusRow(rows[j].id)
              break
            }
          }
        }
        break
      case 'Enter':
      case ' ':
        activate(r)
        break
      default:
        return
    }
    e.preventDefault()
  }
</script>

<div class="tree" role="tree" aria-label={label} bind:this={list} {onkeydown} tabindex="-1">
  {#each rows as r (r.id)}
    <div
      class="row"
      class:active={r.id === active}
      role="treeitem"
      aria-level={r.depth + 1}
      aria-expanded={r.expandable ? r.expanded : undefined}
      aria-selected={r.id === active}
      tabindex={r.id === cursor ? 0 : -1}
      data-id={r.id}
      style="--depth: {r.depth}"
      onclick={() => activate(r)}
      onfocus={() => (focused = r.id)}
      onkeydown={() => {}}
    >
      <span class="twisty" class:open={r.expanded}>
        {#if r.expandable}<ChevronRight size={14} strokeWidth={1.75} />{/if}
      </span>
      {@render row(r.data, r)}
    </div>
  {/each}
</div>

<style>
  .tree {
    display: flex;
    flex-direction: column;
    padding: var(--s-1) 0 var(--s-4);
    outline: none;
  }
  .row {
    position: relative;
    display: flex;
    align-items: center;
    gap: var(--s-1);
    height: 24px;
    padding: 0 var(--s-2) 0 calc(var(--s-2) + var(--depth) * 12px);
    font-size: var(--fs-md);
    color: var(--fg-1);
    white-space: nowrap;
    cursor: pointer;
    user-select: none;
  }
  /* Indent guides: one faint line per level. */
  .row::before {
    content: '';
    position: absolute;
    inset: 0 auto 0 calc(var(--s-2) + 6px);
    width: calc(var(--depth) * 12px);
    background: repeating-linear-gradient(to right, var(--border) 0 1px, transparent 1px 12px);
    pointer-events: none;
  }
  .row:hover {
    color: var(--fg-0);
    background: var(--bg-2);
  }
  .row.active {
    color: var(--fg-0);
    background: var(--bg-3);
  }
  .row:focus-visible {
    outline: 1px solid var(--accent);
    outline-offset: -1px;
  }
  .twisty {
    flex: 0 0 auto;
    display: grid;
    place-items: center;
    width: 14px;
    color: var(--fg-2);
    transition: transform var(--dur) var(--ease);
  }
  .twisty.open {
    transform: rotate(90deg);
  }
</style>
