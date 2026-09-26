<script lang="ts">
  import { shortcutKeys, formatShortcut } from '../utils/platform'

  // Renders a shortcut like "Mod+K" as platform-correct key caps.
  let { combo }: { combo: string } = $props()
  const keys = $derived(shortcutKeys(combo))
</script>

<span class="kbd" aria-label={formatShortcut(combo)}>
  {#each keys as k (k)}<kbd>{k}</kbd>{/each}
</span>

<style>
  .kbd {
    display: inline-flex;
    gap: 2px;
    vertical-align: middle;
  }
  kbd {
    min-width: 18px;
    height: 18px;
    padding: 0 var(--s-1);
    display: inline-grid;
    place-items: center;
    font-family: var(--font-ui);
    font-size: var(--fs-xs);
    font-weight: var(--fw-medium);
    color: var(--fg-1);
    background: var(--bg-3);
    border: 1px solid var(--border-strong);
    border-bottom-width: 2px;
    border-radius: var(--r-sm);
    line-height: 1;
  }
</style>
