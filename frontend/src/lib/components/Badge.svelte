<script lang="ts">
  import type { Component, Snippet } from 'svelte'

  // A short label: a count, a state, a rule's code. `accent` is in the
  // tool's colour (the accent outside a tool), a status in its own;
  // `neutral` is grey. `solid` fills it, for the one that must stand out
  // (What-if · 2, FAILED); `mono` is for code.
  interface Props {
    tone?: 'neutral' | 'accent' | 'ok' | 'warn' | 'err' | 'info'
    icon?: Component
    solid?: boolean
    mono?: boolean
    children: Snippet
  }

  let { tone = 'neutral', icon: Icon, solid = false, mono = false, children }: Props = $props()
</script>

<span class="badge {tone}" class:solid class:mono>
  {#if Icon}<Icon size={12} />{/if}
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
    border-radius: var(--r-sm);
  }
  .mono {
    font-family: var(--font-mono);
    font-weight: var(--fw-medium);
    letter-spacing: 0;
  }
  .neutral {
    color: var(--fg-1);
    background: var(--bg-3);
  }
  .accent {
    color: var(--tool-ink, var(--accent));
    background: var(--tool-accent-soft, var(--accent-soft));
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
  .info {
    color: var(--info);
    background: var(--info-soft);
  }
  .solid {
    color: var(--accent-fg);
  }
  .solid.accent {
    background: var(--tool-accent, var(--accent));
  }
  .solid.ok {
    background: var(--ok);
  }
  .solid.warn {
    background: var(--warn);
  }
  .solid.err {
    background: var(--err);
  }
  .solid.info {
    background: var(--info);
  }
</style>
