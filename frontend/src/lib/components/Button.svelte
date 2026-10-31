<script lang="ts">
  import type { Component, Snippet } from 'svelte'
  import type { HTMLButtonAttributes } from 'svelte/elements'
  import type { Tone } from './tone'

  interface Props extends HTMLButtonAttributes {
    variant?: 'primary' | 'secondary' | 'ghost'
    size?: 'sm' | 'md'
    icon?: Component
    /** What kind of action this is; themes that show colour tint the icon
     *  (not on a primary button, whose fill already stands out). */
    tone?: Tone
    children?: Snippet
  }

  let { variant = 'secondary', size = 'md', icon: Icon, tone, children, type = 'button', ...rest }: Props = $props()
</script>

<button {type} class="btn {variant} {size}" class:toned={!!tone} style:--act={tone && `var(--act-${tone})`} {...rest}>
  {#if Icon}<Icon size={size === 'sm' ? 14 : 15} strokeWidth={1.75} />{/if}
  {#if children}<span>{@render children()}</span>{/if}
</button>

<style>
  .btn {
    display: inline-flex;
    align-items: center;
    justify-content: center;
    gap: var(--s-2);
    height: var(--control-h);
    padding: 0 var(--s-3);
    font-size: var(--fs-md);
    font-weight: var(--fw-medium);
    white-space: nowrap;
    border: 1px solid transparent;
    border-radius: var(--r-md);
    transition:
      background var(--dur) var(--ease),
      border-color var(--dur) var(--ease),
      color var(--dur) var(--ease);
  }
  .btn.sm {
    height: var(--control-h-sm);
    padding: 0 var(--s-2);
    font-size: var(--fs-sm);
    gap: var(--s-1);
  }
  .btn:disabled {
    opacity: 0.45;
    cursor: default;
  }

  .toned:not(.primary, :disabled) :global(svg) {
    color: color-mix(in oklab, var(--act) var(--tint-action), currentColor);
  }

  /* Inside a tool, the tool's accent (the plain accent unless the theme
     shows tool colours). */
  .primary {
    background: var(--tool-accent, var(--accent));
    color: var(--accent-fg);
  }
  .primary:hover:not(:disabled) {
    background: var(--tool-accent-hover, var(--accent-hover));
  }

  .secondary {
    background: var(--bg-3);
    border-color: var(--border-strong);
    color: var(--fg-0);
  }
  .secondary:hover:not(:disabled) {
    background: var(--bg-4);
  }

  .ghost {
    background: transparent;
    color: var(--fg-1);
  }
  .ghost:hover:not(:disabled) {
    background: var(--bg-3);
    color: var(--fg-0);
  }
</style>
