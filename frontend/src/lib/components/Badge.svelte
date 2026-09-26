<script lang="ts">
  import type { Component, Snippet } from 'svelte'

  interface Props {
    tone?: 'neutral' | 'accent' | 'ok' | 'warn' | 'err'
    icon?: Component
    solid?: boolean
    children: Snippet
  }

  let { tone = 'neutral', icon: Icon, solid = false, children }: Props = $props()
</script>

<span class="badge {tone}" class:solid>
  {#if Icon}<Icon size={12} strokeWidth={2} />{/if}
  {@render children()}
</span>

<style>
  .badge {
    display: inline-flex;
    align-items: center;
    gap: var(--s-1);
    height: 20px;
    padding: 0 var(--s-2);
    font-size: var(--fs-xs);
    font-weight: var(--fw-semibold);
    letter-spacing: 0.02em;
    white-space: nowrap;
    border-radius: var(--r-full);
  }
  .neutral {
    color: var(--fg-1);
    background: var(--bg-3);
  }
  .accent {
    color: var(--accent);
    background: var(--accent-soft);
  }
  .ok {
    color: var(--ok);
    background: var(--ok-soft);
  }
  .warn {
    color: var(--warn);
    background: var(--warn-soft);
  }
  .err {
    color: var(--err);
    background: var(--err-soft);
  }
  .solid.ok {
    color: var(--bg-0);
    background: var(--ok);
  }
  .solid.err {
    color: var(--bg-0);
    background: var(--err);
  }
  .solid.warn {
    color: var(--bg-0);
    background: var(--warn);
  }
</style>
