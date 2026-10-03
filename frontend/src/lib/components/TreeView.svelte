<script module lang="ts">
  export interface Row<D> {
    id: string
    depth: number
    expandable: boolean
    expanded: boolean
    data: D
  }

  /** Drag data type of a dragged row; drop targets look for it. */
  export const TREE_DRAG_TYPE = 'application/x-codec-tree'
</script>

<script lang="ts" generics="T">
  import { tick, type Snippet } from 'svelte'
  import ChevronRight from '@lucide/svelte/icons/chevron-right'

  // Keyboard-navigable tree (WAI-ARIA tree pattern) over pre-flattened
  // rows: the caller decides which rows are visible, this component
  // renders them and handles focus, arrows and clicks. Long trees (a
  // 10k-file repo, a 9,000-document outline) render only the rows in
  // view, so opening and scrolling them stays instant.
  //   ↑/↓ move · → expand / first child · ← collapse / parent
  //   Enter/Space open or toggle · Home/End
  // With `selection`, Ctrl/⌘+click or Ctrl+Space adds a selectable row to
  // it (a plain click still opens), and Esc empties it.

  interface Props {
    rows: Row<T>[]
    label: string
    /** Highlighted row, e.g. the file in the active tab. */
    active?: string | null
    row: Snippet<[T, Row<T>]>
    ontoggle: (id: string) => void
    onopen: (data: T) => void
    /** Clicking a parent row toggles it (file tree). When false, every
     *  click opens and only the chevron toggles (outline). */
    toggleOnClick?: boolean
    /** Multi-selection, owned by the caller. */
    selection?: Set<string>
    selectable?: (data: T) => boolean
    /** Text to carry when a row is dragged; null = not draggable. */
    dragText?: (data: T) => string | null
  }

  let {
    rows,
    label,
    active = null,
    row,
    ontoggle,
    onopen,
    toggleOnClick = true,
    selection,
    selectable = () => false,
    dragText = () => null,
  }: Props = $props()

  let focused = $state<string | null>(null)
  let list = $state<HTMLDivElement>()

  // --- windowing ---
  const ROW = 24 // px, fixed by .row's height
  const PAD = 4 // px, .tree's top padding
  const WINDOW_FROM = 300 // rows; shorter trees render whole
  const OVERSCAN = 30 // rows rendered beyond each edge

  // The element that scrolls the tree (the panel around it).
  let scroller: HTMLElement | null = null
  let viewTop = $state(0) // px of the tree scrolled out of view
  let viewHeight = $state(1000)

  const windowed = $derived(rows.length > WINDOW_FROM)
  const first = $derived(windowed ? Math.max(0, Math.floor((viewTop - PAD) / ROW) - OVERSCAN) : 0)
  const last = $derived(windowed ? Math.min(rows.length, Math.ceil((viewTop + viewHeight) / ROW) + OVERSCAN) : rows.length)
  const shown = $derived(windowed ? rows.slice(first, last) : rows)

  function measure() {
    if (!scroller || !list) return
    viewTop = Math.max(0, scroller.getBoundingClientRect().top - list.getBoundingClientRect().top)
    viewHeight = scroller.clientHeight
  }

  $effect(() => {
    if (!list) return
    let el = list.parentElement
    while (el && !/auto|scroll/.test(getComputedStyle(el).overflowY)) el = el.parentElement
    scroller = el
    if (!el) return
    measure()
    el.addEventListener('scroll', measure, { passive: true })
    const ro = new ResizeObserver(measure)
    ro.observe(el)
    return () => {
      el.removeEventListener('scroll', measure)
      ro.disconnect()
    }
  })

  /** Scroll so row i is in view; it may not be rendered yet. */
  function reveal(i: number) {
    if (!scroller || !list || i < 0) return
    const top = list.getBoundingClientRect().top - scroller.getBoundingClientRect().top + scroller.scrollTop + PAD + i * ROW
    if (top < scroller.scrollTop) scroller.scrollTop = top
    else if (top + ROW > scroller.scrollTop + scroller.clientHeight) scroller.scrollTop = top + ROW - scroller.clientHeight
    measure()
  }

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
    queueMicrotask(() => reveal(rows.findIndex((r) => r.id === id)))
  })

  function rowEl(id: string): HTMLElement | null {
    return list?.querySelector<HTMLElement>(`[data-id="${CSS.escape(id)}"]`) ?? null
  }

  async function focusRow(id: string | undefined) {
    if (!id) return
    focused = id
    reveal(rows.findIndex((r) => r.id === id))
    await tick() // renders the row if it was outside the window
    rowEl(id)?.focus({ preventScroll: true })
  }

  // With the cursor row scrolled out (so not rendered), the tree itself
  // takes Tab focus and hands it on.
  const cursorShown = $derived(shown.some((r) => r.id === cursor))

  function activate(r: Row<T>, add = false) {
    focused = r.id
    if (add && selection && selectable(r.data)) {
      if (selection.has(r.id)) selection.delete(r.id)
      else selection.add(r.id)
      return
    }
    if (r.expandable && toggleOnClick) ontoggle(r.id)
    else {
      selection?.clear()
      onopen(r.data)
    }
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
        activate(r, e.key === ' ' && (e.ctrlKey || e.metaKey))
        break
      case 'Escape':
        if (!selection?.size) return
        selection.clear()
        break
      default:
        return
    }
    e.preventDefault()
  }
</script>

<!-- svelte-ignore a11y_no_noninteractive_tabindex -->
<div
  class="tree"
  role="tree"
  aria-label={label}
  aria-multiselectable={selection ? true : undefined}
  bind:this={list}
  {onkeydown}
  tabindex={cursorShown ? -1 : 0}
  onfocus={(e) => e.target === list && focusRow(cursor ?? undefined)}
>
  {#if first > 0}<div class="spacer" style:height="{first * ROW}px" aria-hidden="true"></div>{/if}
  {#each shown as r (r.id)}
    {@const drag = dragText(r.data)}
    <div
      class="row"
      class:active={r.id === active}
      class:selected={selection?.has(r.id)}
      role="treeitem"
      aria-level={r.depth + 1}
      aria-expanded={r.expandable ? r.expanded : undefined}
      aria-selected={selection ? selection.has(r.id) : r.id === active}
      aria-current={selection && r.id === active ? 'true' : undefined}
      tabindex={r.id === cursor ? 0 : -1}
      data-id={r.id}
      style="--depth: {r.depth}"
      draggable={drag !== null}
      ondragstart={(e) => {
        if (drag === null || !e.dataTransfer) return
        e.dataTransfer.setData(TREE_DRAG_TYPE, drag)
        e.dataTransfer.setData('text/plain', drag)
        e.dataTransfer.effectAllowed = 'copy'
      }}
      onclick={(e) => activate(r, e.ctrlKey || e.metaKey)}
      onfocus={() => (focused = r.id)}
      onkeydown={() => {}}
    >
      <!-- svelte-ignore a11y_click_events_have_key_events, a11y_no_static_element_interactions -->
      <span
        class="twisty"
        class:open={r.expanded}
        onclick={(e) => {
          if (!r.expandable) return
          e.stopPropagation()
          focused = r.id
          ontoggle(r.id)
        }}
      >
        {#if r.expandable}<ChevronRight size={14} strokeWidth={1.75} />{/if}
      </span>
      {@render row(r.data, r)}
    </div>
  {/each}
  {#if last < rows.length}<div class="spacer" style:height="{(rows.length - last) * ROW}px" aria-hidden="true"></div>{/if}
</div>

<style>
  .tree {
    display: flex;
    flex-direction: column;
    padding: var(--s-1) 0 var(--s-4);
    outline: none;
  }
  .spacer {
    flex: 0 0 auto;
  }
  .row {
    position: relative;
    flex: 0 0 auto;
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
  .row.selected {
    color: var(--fg-0);
    background: var(--accent-soft);
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
