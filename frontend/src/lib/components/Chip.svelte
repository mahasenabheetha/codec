<script lang="ts">
  import Check from '@lucide/svelte/icons/check'
  import type { Snippet } from 'svelte'
  import type { HTMLButtonAttributes } from 'svelte/elements'

  // A small choice in a row of them. Two kinds, one look:
  //  - a suggestion (a preset, an example): click to use it; `selected`
  //    marks the one in use, when that is known;
  //  - a toggle (a flag, a weekday): pass `pressed`, and it is a switch
  //    with aria-pressed.
  // On, it takes a soft fill and an edge in the tool's colour, and a
  // tick. `square` is for one-letter toggles (regex flags), which have
  // no room for the tick; `mono` for code.
  interface Props extends HTMLButtonAttributes {
    pressed?: boolean
    selected?: boolean
    square?: boolean
    mono?: boolean
    children: Snippet
  }

  let { pressed, selected = false, square = false, mono = false, children, type = 'button', ...rest }: Props = $props()
  const on = $derived(pressed || selected)
</script>

<button {type} class="chip" class:on class:square class:mono aria-pressed={pressed} {...rest}>
  {#if on && !square}<Check size={12} />{/if}
  {@render children()}
</button>

<style>
  .chip {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    gap: var(--s-1);
    height: var(--control-h-sm);
    padding: 0 var(--s-3);
    font-size: var(--fs-sm);
    white-space: nowrap;
    color: var(--fg-1);
    background: var(--bg-2);
    border: 1px solid var(--border);
    border-radius: var(--r-full);
    transition:
      background var(--dur-fast) var(--ease),
      border-color var(--dur-fast) var(--ease),
      color var(--dur-fast) var(--ease);
  }
  .chip:hover:not(:disabled) {
    color: var(--fg-0);
    background: var(--bg-3);
  }
  .square {
    width: var(--control-h-sm);
    padding: 0;
  }
  .mono {
    font-family: var(--font-mono);
  }
  .on,
  .on:hover:not(:disabled) {
    color: var(--fg-0);
    background: var(--tool-accent-soft, var(--accent-soft));
    border-color: var(--tool-line, color-mix(in srgb, var(--accent) 45%, var(--border)));
  }
  .on :global(svg) {
    color: var(--tool-accent, var(--accent));
  }
  .chip:disabled {
    opacity: 0.45;
    cursor: not-allowed;
  }
</style>
