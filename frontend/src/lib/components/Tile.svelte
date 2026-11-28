<script lang="ts">
  import type { Component } from 'svelte'

  // An icon on a rounded square in the tool's colour: soft (a tint of it)
  // or solid (filled, the icon in the text colour for fills). A status
  // takes the place of the tool's colour and keeps it with tool colours
  // off; a soft tile in the tool's colour turns grey then.
  interface Props {
    icon: Component
    /** sm 20, md 28, lg 40, xl 56 at Comfortable; they grow with the controls. */
    size?: 'sm' | 'md' | 'lg' | 'xl'
    solid?: boolean
    status?: 'ok' | 'warn' | 'err' | 'info'
  }

  let { icon: Icon, size = 'md', solid = false, status }: Props = $props()
  const iconSize = { sm: 12, md: 16, lg: 20, xl: 28 }
</script>

<span class="tile {size} {status ?? ''}" class:solid class:status={!!status} aria-hidden="true">
  <Icon size={iconSize[size]} />
</span>

<style>
  .tile {
    flex: none;
    display: grid;
    place-items: center;
    color: color-mix(in oklab, var(--tool-accent, var(--accent)) var(--tint-tool), var(--fg-1));
    background: color-mix(in oklab, var(--tool-accent-soft, var(--accent-soft)) var(--tint-tool), var(--bg-3));
  }
  .sm {
    width: calc(var(--control-h-sm) - 8px);
    height: calc(var(--control-h-sm) - 8px);
    border-radius: var(--r-sm);
  }
  .md {
    width: calc(var(--control-h) - 4px);
    height: calc(var(--control-h) - 4px);
    border-radius: var(--r-md);
  }
  .lg {
    width: calc(var(--control-h) + 8px);
    height: calc(var(--control-h) + 8px);
    border-radius: var(--r-lg);
  }
  .xl {
    width: calc(var(--control-h) + 24px);
    height: calc(var(--control-h) + 24px);
    border-radius: var(--r-xl);
  }
  .solid {
    color: var(--accent-fg);
    background: var(--tool-accent, var(--accent));
  }

  .ok {
    --st: var(--ok);
    --st-soft: var(--ok-soft);
  }
  .warn {
    --st: var(--warn);
    --st-soft: var(--warn-soft);
  }
  .err {
    --st: var(--err);
    --st-soft: var(--err-soft);
  }
  .info {
    --st: var(--info);
    --st-soft: var(--info-soft);
  }
  .status {
    color: var(--st);
    background: var(--st-soft);
  }
  .status.solid {
    color: var(--accent-fg);
    background: var(--st);
  }
</style>
