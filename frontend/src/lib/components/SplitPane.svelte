<script lang="ts">
  import { untrack, type Snippet } from 'svelte'

  // Two resizable panes. Side by side when there is room, stacked on
  // narrow windows. The split ratio persists per `id`.
  interface Props {
    id: string
    initial?: number // share of the first pane, 0..1
    min?: number // minimum pane size in px
    first: Snippet
    second: Snippet
  }

  let { id, initial = 0.5, min = 180, first, second }: Props = $props()

  // A pane's identity is fixed for its lifetime, so the key is read once.
  const key = untrack(() => `codec:split:${id}`)

  function loadRatio(): number {
    try {
      const v = Number(localStorage.getItem(key))
      return v > 0 && v < 1 ? v : initial
    } catch {
      return initial
    }
  }

  let ratio = $state(loadRatio())
  let container = $state<HTMLDivElement>()
  let size = $state({ w: 0, h: 0 })
  let dragging = $state(false)

  const stacked = $derived(size.w > 0 && size.w < 760)

  $effect(() => {
    try {
      localStorage.setItem(key, String(ratio))
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
    ratio = clamp(r)
  }

  function onkeydown(e: KeyboardEvent) {
    const step = e.shiftKey ? 0.1 : 0.02
    const back = stacked ? 'ArrowUp' : 'ArrowLeft'
    const fwd = stacked ? 'ArrowDown' : 'ArrowRight'
    if (e.key === back) ratio = clamp(ratio - step)
    else if (e.key === fwd) ratio = clamp(ratio + step)
    else return
    e.preventDefault()
  }
</script>

<div class="split" class:stacked class:dragging bind:this={container}>
  <div class="pane" style="flex-basis: {ratio * 100}%">{@render first()}</div>
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
    tabindex="0"
    {onpointerdown}
    {onpointermove}
    onpointerup={() => (dragging = false)}
    ondblclick={() => (ratio = initial)}
    {onkeydown}
  ></div>
  <div class="pane" style="flex-basis: {(1 - ratio) * 100}%">{@render second()}</div>
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
  .divider {
    position: relative;
    flex: 0 0 var(--s-2);
    cursor: col-resize;
    border-radius: var(--r-full);
  }
  .stacked .divider {
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
  .stacked .divider::after {
    inset: 3px var(--s-3);
  }
  .divider:hover::after,
  .dragging .divider::after,
  .divider:focus-visible::after {
    background: var(--accent);
  }
</style>
