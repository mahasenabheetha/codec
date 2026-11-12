<script lang="ts">
  import type { Snippet } from 'svelte'
  import type { HTMLButtonAttributes } from 'svelte/elements'

  // A small choice in a row of them. Two kinds, one look:
  //  - a suggestion (a preset, an example): click to use it; `selected`
  //    marks the one in use, when that is known;
  //  - a toggle (a flag, a weekday): pass `pressed`, and it is a switch
  //    with aria-pressed, filled in the tool's colour when on.
  // `square` is for one-letter toggles (regex flags), `mono` for code.
  interface Props extends HTMLButtonAttributes {
    pressed?: boolean
    selected?: boolean
    square?: boolean
    mono?: boolean
    children: Snippet
  }

  let { pressed, selected = false, square = false, mono = false, children, type = 'button', ...rest }: Props = $props()
</script>

<button {type} class="chip" class:toggle={pressed !== undefined} class:on={pressed || selected} class:square class:mono aria-pressed={pressed} {...rest}>
  {@render children()}
</button>

<style>
  .chip {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    height: var(--control-h-sm);
    padding: 0 var(--s-3);
    font-size: var(--fs-sm);
    white-space: nowrap;
    color: var(--fg-1);
    background: var(--bg-3);
    border: 1px solid var(--border);
    border-radius: var(--r-full);
    transition:
      background var(--dur) var(--ease),
      border-color var(--dur) var(--ease),
      color var(--dur) var(--ease);
  }
  .chip:hover:not(:disabled) {
    color: var(--fg-0);
    border-color: var(--border-strong);
  }
  /* A toggle is squarer, so a row of them reads as switches, not as
     suggestions. */
  .toggle {
    padding: 0 var(--s-2);
    background: var(--bg-0);
    border-color: var(--border-strong);
    border-radius: var(--r-md);
  }
  .square {
    width: var(--control-h-sm);
    padding: 0;
  }
  .mono {
    font-family: var(--font-mono);
  }
  /* The suggestion in use: outlined in the tool's colour. */
  .on {
    color: var(--fg-0);
    background: var(--tool-accent-soft, var(--accent-soft));
    border-color: var(--tool-accent, var(--accent));
  }
  /* A toggle that is on: filled. */
  .toggle.on {
    color: var(--accent-fg);
    font-weight: var(--fw-semibold);
    background: var(--tool-accent, var(--accent));
  }
  .chip:disabled {
    opacity: 0.4;
    cursor: not-allowed;
  }
</style>
