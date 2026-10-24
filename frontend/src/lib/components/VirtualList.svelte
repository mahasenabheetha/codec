<script lang="ts">
  import type { Snippet } from 'svelte'
  import { tick } from 'svelte'

  // A list that keeps only the rows in view in the DOM, for lists that
  // can run to thousands of rows (decision 70; TreeView does the same
  // for fixed-height trees). Rows may differ in height: each is
  // measured once drawn, and rows not drawn yet count as `estimate`.
  // The nearest scrolling ancestor scrolls the list, so it can sit
  // below a header inside the same panel.
  interface Props {
    count: number
    row: Snippet<[number]>
    /** A stable key per row, so measured heights survive filtering. */
    key?: (i: number) => string | number
    estimate?: number // px for a row not measured yet
    overscan?: number // px drawn beyond each edge
    windowFrom?: number // rows; shorter lists draw whole
    /** Called with the rows drawn, for loading them page by page. */
    onrange?: (first: number, last: number) => void
  }

  let { count, row, key = (i) => i, estimate = 28, overscan = 600, windowFrom = 150, onrange }: Props = $props()

  let list = $state<HTMLDivElement>()
  let scroller: HTMLElement | null = null
  let viewTop = $state(0) // px of the list scrolled out of view above
  let viewHeight = $state(1000)

  // Measured heights by key; version changes when one does.
  const sizes = new Map<string | number, number>()
  let version = $state(0)

  const windowed = $derived(count > windowFrom)
  const offsets = $derived.by(() => {
    void version
    const o = new Float64Array(count + 1)
    for (let i = 0; i < count; i++) o[i + 1] = o[i] + (sizes.get(key(i)) ?? estimate)
    return o
  })

  /** The last index whose top is at or above y. */
  function at(y: number): number {
    let lo = 0
    let hi = count
    while (lo < hi) {
      const mid = (lo + hi + 1) >> 1
      if (offsets[mid] <= y) lo = mid
      else hi = mid - 1
    }
    return Math.min(lo, Math.max(count - 1, 0))
  }

  const first = $derived(windowed ? at(viewTop - overscan) : 0)
  const last = $derived(windowed ? Math.min(count, at(viewTop + viewHeight + overscan) + 1) : count)
  const shown = $derived(Array.from({ length: Math.max(0, last - first) }, (_, j) => first + j))

  $effect(() => {
    onrange?.(first, last)
  })

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

  // One observer measures every drawn row.
  const keys = new WeakMap<Element, string | number>()
  const rowObserver = new ResizeObserver((entries) => {
    let changed = false
    for (const e of entries) {
      const k = keys.get(e.target)
      if (k === undefined) continue
      const h = e.borderBoxSize?.[0]?.blockSize ?? (e.target as HTMLElement).offsetHeight
      if (h > 0 && sizes.get(k) !== h) {
        sizes.set(k, h)
        changed = true
      }
    }
    if (changed) version++
  })
  $effect(() => () => rowObserver.disconnect())

  function measured(el: HTMLElement, k: string | number) {
    keys.set(el, k)
    rowObserver.observe(el)
    return {
      update(next: string | number) {
        keys.set(el, next)
      },
      destroy() {
        rowObserver.unobserve(el)
      },
    }
  }

  /** Scroll so row i is in view: at the top, centred, or as little as needed. */
  export async function scrollToIndex(i: number, align: 'start' | 'center' | 'nearest' = 'nearest') {
    if (!scroller || !list || i < 0 || i >= count) return
    // Twice: the first scroll draws the row, whose real height may then move it.
    for (let pass = 0; pass < 2; pass++) {
      const base = list.getBoundingClientRect().top - scroller.getBoundingClientRect().top + scroller.scrollTop
      const top = base + offsets[i]
      const h = offsets[i + 1] - offsets[i]
      const view = scroller.clientHeight
      if (align === 'start') scroller.scrollTop = top
      else if (align === 'center') scroller.scrollTop = top - (view - h) / 2
      else if (top < scroller.scrollTop) scroller.scrollTop = top
      else if (top + h > scroller.scrollTop + view) scroller.scrollTop = top + h - view
      measure()
      await tick()
    }
    // Rows measured on the way (those around the target, drawn for the
    // first time after a fresh list) can still move it: correct by where
    // the row really is, until it stays put.
    for (let pass = 0; pass < 4; pass++) {
      if (i < first || i >= last) break
      const el = list.children[i - first + (first > 0 ? 1 : 0)] as HTMLElement | undefined // after the spacer, if any
      if (!el) break
      const r = el.getBoundingClientRect()
      const s = scroller.getBoundingClientRect()
      const want = align === 'start' ? s.top : align === 'center' ? s.top + (s.height - r.height) / 2 : r.top < s.top ? s.top : r.bottom > s.bottom ? s.bottom - r.height : r.top
      if (Math.abs(r.top - want) < 2) break
      scroller.scrollTop += r.top - want
      measure()
      await tick()
    }
  }
</script>

<div class="vlist" bind:this={list}>
  {#if first > 0}<div aria-hidden="true" style:height="{offsets[first]}px"></div>{/if}
  {#each shown as i (key(i))}
    <div class="vrow" use:measured={key(i)}>{@render row(i)}</div>
  {/each}
  {#if last < count}<div aria-hidden="true" style:height="{offsets[count] - offsets[last]}px"></div>{/if}
</div>

<style>
  .vlist {
    /* Rows are measured here; scroll anchoring keeps the view still
       when a row above it changes height once measured. */
    overflow-anchor: auto;
  }
  .vrow {
    /* Contains the row's margins, so its measured height is its whole height. */
    display: flow-root;
  }
</style>
