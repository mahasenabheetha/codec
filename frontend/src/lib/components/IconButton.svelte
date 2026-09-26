<script lang="ts">
  import type { Component } from 'svelte'
  import type { HTMLButtonAttributes } from 'svelte/elements'
  import Tooltip from './Tooltip.svelte'
  import { chain } from '../utils/events'

  // Square icon-only button. `label` is both the accessible name and
  // the tooltip, so an icon button can never ship unlabeled.
  interface Props extends HTMLButtonAttributes {
    icon: Component
    label: string
    shortcut?: string
    size?: 'sm' | 'md'
    active?: boolean
    side?: 'top' | 'right' | 'bottom' | 'left'
  }

  let { icon: Icon, label, shortcut, size = 'md', active = false, side = 'bottom', disabled, onclick, ...rest }: Props = $props()
</script>

<Tooltip text={label} {shortcut} {side}>
  {#snippet children(props)}
    <button
      {...props}
      {...rest}
      type="button"
      class="icon-btn {size}"
      class:active
      aria-label={label}
      {disabled}
      onclick={chain(props.onclick, onclick)}
    >
      <Icon size={size === 'sm' ? 14 : 16} strokeWidth={1.75} />
    </button>
  {/snippet}
</Tooltip>

<style>
  .icon-btn {
    display: inline-grid;
    place-items: center;
    width: var(--control-h);
    height: var(--control-h);
    padding: 0;
    color: var(--fg-1);
    background: transparent;
    border: none;
    border-radius: var(--r-md);
    transition:
      background var(--dur) var(--ease),
      color var(--dur) var(--ease);
  }
  .icon-btn.sm {
    width: var(--control-h-sm);
    height: var(--control-h-sm);
  }
  .icon-btn:hover:not(:disabled) {
    background: var(--bg-3);
    color: var(--fg-0);
  }
  .icon-btn.active {
    color: var(--accent);
    background: var(--accent-soft);
  }
  .icon-btn:disabled {
    opacity: 0.4;
    cursor: default;
  }
</style>
