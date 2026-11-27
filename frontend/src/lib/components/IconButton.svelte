<script lang="ts">
  import type { Component } from 'svelte'
  import type { HTMLButtonAttributes } from 'svelte/elements'
  import Tooltip from './Tooltip.svelte'
  import { chain } from '../utils/events'
  import type { Tone } from './tone'

  // Square icon-only button. `label` is both the accessible name and
  // the tooltip, so an icon button can never ship unlabeled.
  interface Props extends HTMLButtonAttributes {
    icon: Component
    label: string
    shortcut?: string
    size?: 'sm' | 'md'
    /** On, for a button that toggles (aria-pressed is the caller's). */
    active?: boolean
    /** A frame and a ground, for a row of framed buttons (a tool's
     *  actions in its toolbar). */
    box?: boolean
    side?: 'top' | 'right' | 'bottom' | 'left'
    /** What kind of action this is; themes that show colour tint the icon. */
    tone?: Tone
  }

  let { icon: Icon, label, shortcut, size = 'md', active = false, box = false, side = 'bottom', tone, disabled, onclick, ...rest }: Props = $props()
</script>

<Tooltip text={label} {shortcut} {side}>
  {#snippet children(props)}
    <button
      {...props}
      {...rest}
      type="button"
      class="icon-btn {size}"
      class:active
      class:box
      class:toned={!!tone}
      style:--act={tone && `var(--act-${tone})`}
      aria-label={label}
      {disabled}
      onclick={chain(props.onclick, onclick)}
    >
      <Icon size={size === 'sm' ? 14 : 16} />
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
    border: 1px solid transparent;
    border-radius: var(--r-md);
    transition:
      background var(--dur-fast) var(--ease),
      border-color var(--dur-fast) var(--ease),
      color var(--dur-fast) var(--ease);
  }
  .icon-btn.sm {
    width: var(--control-h-sm);
    height: var(--control-h-sm);
  }
  .icon-btn.box {
    background: var(--bg-3);
    border-color: var(--border);
  }
  .icon-btn:hover:not(:disabled) {
    background: var(--bg-4);
    color: var(--fg-0);
  }
  /* Pressed: framed, as a box button is at rest. */
  .icon-btn:active:not(:disabled) {
    background: var(--bg-3);
    border-color: var(--border);
  }
  .icon-btn.toned:not(.active, :disabled) :global(svg) {
    color: color-mix(in oklab, var(--act) var(--tint-action), currentColor);
  }
  .icon-btn.active {
    color: var(--tool-accent, var(--accent));
    background: var(--tool-accent-soft, var(--accent-soft));
  }
  .icon-btn:disabled {
    opacity: 0.45;
    cursor: not-allowed;
  }
</style>
