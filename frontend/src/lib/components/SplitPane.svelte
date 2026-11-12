<script lang="ts">
  import { untrack, type Snippet } from 'svelte'

  // Two resizable panes: drag the divider, or focus it and use the
  // arrow keys (Shift for bigger steps, Home and End for the limits);
  // a double click or Enter puts it back. Side by side when there is
  // room, stacked when narrow, or as `direction` says. The share of the
  // first pane persists per `id`, side by side and stacked apart (a
  // display preference).
  interface Props {
    id: string
    initial?: number // share of the first pane, 0..1
    min?: number // minimum pane size in px
    /** auto: side by side unless narrower than `stackBelow`. */
    direction?: 'auto' | 'row' | 'column'
    stackBelow?: number
    /** Panes that touch: the divider is a line, not a gap. */
    flush?: boolean
    /** The first pane's width in px while side by side: a side panel
     *  that keeps its width when the window changes, and gives the rest
     *  to the second pane. Stacked, the share (`initial`) applies. */
    px?: number
    first: Snippet
    second: Snippet
  }

  let { id, initial = 0.5, min = 180, direction = 'auto', stackBelow = 760, flush = false, px, first, second }: Props = $props()

  // A pane's identity is fixed for its lifetime, so the key is read once.
  const key = untrack(() => `codec:split:${id}`)

  function loadRatio(k: string): number {
    try {
      const v = Number(localStorage.getItem(k))
      return v > 0 && v < 1 ? v : initial
    } catch {
      return initial
    }
  }

  // One share for each way of laying the panes out: what suits a wide
  // row rarely suits a column.
  let ratios = $state({ row: loadRatio(key), column: loadRatio(key + ':stacked') })
  let width = $state(
    untrack(() => {
      try {
        const v = Number(localStorage.getItem(key + ':px'))
        return v > 0 ? v : (px ?? 0)
      } catch {
        return px ?? 0
      }
    }),
  )
  let container = $state<HTMLDivElement>()
  let size = $state({ w: 0, h: 0 })
  let dragging = $state(false)

  const stacked = $derived(direction === 'column' || (direction === 'auto' && size.w > 0 && size.w < stackBelow))
  // In px mode, side by side, the first pane has a width, not a share.
  const fixed = $derived(px !== undefined && !stacked)
  const clampPx = (w: number) => Math.max(min, Math.min(w, Math.max(min, size.w - min)))
  const ratio = $derived(fixed && size.w > 0 ? clampPx(width) / size.w : stacked ? ratios.column : ratios.row)
  function set(r: number) {
    if (fixed) width = clampPx(r * size.w)
    else if (stacked) ratios.column = r
    else ratios.row = r
  }

  $effect(() => {
    try {
      localStorage.setItem(key, String(ratios.row))
      localStorage.setItem(key + ':stacked', String(ratios.column))
      if (px !== undefined) localStorage.setItem(key + ':px', String(Math.round(width)))
    } catch {
      /* persistence is optional */
    }
  })

  $effect(() => {
    if (!container) return
    const ro = new ResizeObserver(([entry]) => {
      size = { w: entry.contentRect.width, h: entry.contentRect.height }
    })
    ro.observe(container)
    return () => ro.disconnect()
  })

  function clamp(r: number): number {
    const total = stacked ? size.h : size.w
    if (total <= 0) return r
    const lo = Math.min(0.5, min / total)
    return Math.min(1 - lo, Math.max(lo, r))
  }

  function onpointerdown(e: PointerEvent) {
    e.preventDefault()
    dragging = true
    ;(e.currentTarget as HTMLElement).setPointerCapture(e.pointerId)
  }

  function onpointermove(e: PointerEvent) {
    if (!dragging || !container) return
    const rect = container.getBoundingClientRect()
    const r = stacked ? (e.clientY - rect.top) / rect.height : (e.clientX - rect.left) / rect.width
    set(clamp(r))
  }

  function reset() {
    if (fixed) width = px!
    else set(initial)
  }

  function onkeydown(e: KeyboardEvent) {
    const step = e.shiftKey ? 0.1 : 0.02
    const back = stacked ? 'ArrowUp' : 'ArrowLeft'
    const fwd = stacked ? 'ArrowDown' : 'ArrowRight'
    if (e.key === back) set(clamp(ratio - step))
    else if (e.key === fwd) set(clamp(ratio + step))
    else if (e.key === 'Home') set(clamp(0))
    else if (e.key === 'End') set(clamp(1))
    else if (e.key === 'Enter') reset()
    else return
    e.preventDefault()
  }
</script>

<div class="split" class:stacked class:dragging class:flush bind:this={container}>
  <div class="pane" class:fixed style="flex-basis: {fixed ? clampPx(width) + 'px' : ratio * 100 + '%'}">{@render first()}</div>
  <!-- A focusable separator is the WAI-ARIA "window splitter" pattern;
       Svelte's a11y lint doesn't recognise it as interactive. -->
  <!-- svelte-ignore a11y_no_noninteractive_tabindex, a11y_no_noninteractive_element_interactions -->
  <div
    class="divider"
    role="separator"
    aria-orientation={stacked ? 'horizontal' : 'vertical'}
    aria-valuenow={Math.round(ratio * 100)}
    aria-valuemin={0}
    aria-valuemax={100}
    aria-label="Resize panes"
    title="Drag to resize; double-click to reset"
    tabindex="0"
    {onpointerdown}
    {onpointermove}
    onpointerup={() => (dragging = false)}
    onpointercancel={() => (dragging = false)}
    ondblclick={reset}
    {onkeydown}
  ><span class="grip" aria-hidden="true"></span></div>
  <div class="pane" style="flex-basis: {fixed ? '0' : (1 - ratio) * 100 + '%'}">{@render second()}</div>
</div>

<style>
  .split {
    display: flex;
    width: 100%;
    height: 100%;
    min-height: 0;
  }
  .split.stacked {
    flex-direction: column;
  }
  .split.dragging {
    user-select: none;
    cursor: col-resize;
  }
  .split.stacked.dragging {
    cursor: row-resize;
  }
  .pane {
    min-width: 0;
    min-height: 0;
    flex-grow: 1;
    flex-shrink: 1;
    display: flex;
    flex-direction: column;
  }
  .pane.fixed {
    flex-grow: 0;
    flex-shrink: 0;
  }
  .divider {
    position: relative;
    flex: 0 0 var(--s-2);
    cursor: col-resize;
    border-radius: var(--r-full);
  }
  .stacked > .divider {
    cursor: row-resize;
  }
  /* Thin visual line inside a wider hit area. */
  .divider::after {
    content: '';
    position: absolute;
    inset: var(--s-3) 3px;
    border-radius: var(--r-full);
    transition: background var(--dur) var(--ease);
  }
  .stacked > .divider::after {
    inset: 3px var(--s-3);
  }
  .divider:hover::after,
  .dragging > .divider::after,
  .divider:focus-visible::after {
    background: var(--tool-accent, var(--accent));
  }
  .divider:focus-visible {
    outline: none;
  }
  /* A short grip in the middle, always there, so the divider is found
     without hovering. */
  .grip {
    position: absolute;
    left: 50%;
    top: 50%;
    width: 4px;
    height: 28px;
    transform: translate(-50%, -50%);
    background: var(--border-strong);
    border-radius: var(--r-full);
    transition: background var(--dur) var(--ease);
  }
  .stacked > .divider .grip {
    width: 28px;
    height: 4px;
  }
  .divider:hover .grip,
  .dragging > .divider .grip,
  .divider:focus-visible .grip {
    background: var(--tool-accent, var(--accent));
  }
  /* Flush: panes that touch. The divider is the line between them, and
     its hit area reaches a few pixels into each. */
  .flush > .divider {
    flex-basis: 1px;
    background: var(--border);
    border-radius: 0;
    z-index: 1;
  }
  .flush > .divider::after {
    inset: 0 -4px;
    border-radius: 0;
    background: transparent;
  }
  .flush.stacked > .divider::after {
    inset: -4px 0;
  }
  .flush > .divider:hover,
  .flush.dragging > .divider,
  .flush > .divider:focus-visible {
    background: var(--tool-accent, var(--accent));
    box-shadow: 0 0 0 1px var(--tool-accent, var(--accent));
  }
  .flush > .divider .grip {
    width: 3px;
    height: 24px;
    opacity: 0;
  }
  .flush.stacked > .divider .grip {
    width: 24px;
    height: 3px;
  }
</style>
