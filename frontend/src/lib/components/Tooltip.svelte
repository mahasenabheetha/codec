<script lang="ts">
  import { Tooltip } from 'bits-ui'
  import type { Snippet } from 'svelte'
  import Kbd from './Kbd.svelte'

  // Wraps any element with an accessible tooltip. The children snippet
  // receives props that MUST be spread onto the trigger element. If the
  // element has its own handler for an event those props also handle
  // (e.g. onclick), combine them with chain() from utils/events.
  interface Props {
    text: string
    shortcut?: string
    side?: 'top' | 'right' | 'bottom' | 'left'
    /** Turn the tooltip off (e.g. rail labels are already visible). */
    off?: boolean
    children: Snippet<[Record<string, unknown>]>
  }

  let { text, shortcut, side = 'bottom', off = false, children }: Props = $props()
</script>

<Tooltip.Root delayDuration={350} disabled={off}>
  <Tooltip.Trigger>
    {#snippet child({ props })}
      {@render children(props)}
    {/snippet}
  </Tooltip.Trigger>
  <Tooltip.Portal>
    <Tooltip.Content {side} sideOffset={6} class="codec-tooltip">
      <span>{text}</span>
      {#if shortcut}<Kbd combo={shortcut} />{/if}
    </Tooltip.Content>
  </Tooltip.Portal>
</Tooltip.Root>

<style>
  /* Content is portalled to <body>, outside this component's scope. */
  :global(.codec-tooltip) {
    z-index: var(--z-popover);
    display: flex;
    align-items: center;
    gap: var(--s-2);
    padding: var(--s-1) var(--s-2);
    font-size: var(--fs-sm);
    color: var(--fg-0);
    background: var(--bg-3);
    border-radius: var(--r-md);
    box-shadow: var(--shadow-pop);
    pointer-events: none;
    animation: tooltip-in var(--dur) var(--ease);
  }
  @keyframes tooltip-in {
    from {
      opacity: 0;
      transform: translateY(2px);
    }
  }
</style>
