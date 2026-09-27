<script lang="ts">
  import { Popover } from 'bits-ui'
  import type { Snippet } from 'svelte'

  // Click-to-open floating panel. `trigger` receives props that must be
  // spread onto the trigger element.
  interface Props {
    trigger: Snippet<[Record<string, unknown>]>
    children: Snippet
    side?: 'top' | 'right' | 'bottom' | 'left'
    align?: 'start' | 'center' | 'end'
  }

  let { trigger, children, side = 'top', align = 'end' }: Props = $props()
</script>

<Popover.Root>
  <Popover.Trigger>
    {#snippet child({ props })}
      {@render trigger(props)}
    {/snippet}
  </Popover.Trigger>
  <Popover.Portal>
    <Popover.Content {side} {align} sideOffset={6} class="codec-popover">
      {@render children()}
    </Popover.Content>
  </Popover.Portal>
</Popover.Root>

<style>
  :global(.codec-popover) {
    z-index: var(--z-popover);
    min-width: 220px;
    padding: var(--s-3);
    background: var(--bg-2);
    border-radius: var(--r-lg);
    box-shadow: var(--shadow-pop);
    animation: popover-in var(--dur) var(--ease);
  }
  @keyframes popover-in {
    from {
      opacity: 0;
      transform: translateY(2px);
    }
  }
</style>
