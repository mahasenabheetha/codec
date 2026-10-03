<script lang="ts">
  import RotateCw from '@lucide/svelte/icons/rotate-cw'
  import Button from '../components/Button.svelte'
  import { connection } from '../stores/connection.svelte'

  // Shown when the server rejects this page's token: codec was restarted
  // since the page loaded. Open tabs are saved, so a reload brings them back.
</script>

{#if connection.restarted}
  <div class="banner" role="alert">
    <span><b>codec restarted.</b> Reload to reconnect; your tabs come back.</span>
    <Button size="sm" variant="primary" icon={RotateCw} onclick={() => location.reload()}>Reload</Button>
  </div>
{/if}

<style>
  .banner {
    display: flex;
    align-items: center;
    justify-content: center;
    gap: var(--s-3);
    padding: var(--s-1) var(--s-4);
    font-size: var(--fs-sm);
    color: var(--fg-0);
    background: var(--warn-soft);
    border-bottom: 1px solid var(--border);
  }
  b {
    font-weight: var(--fw-semibold);
    color: var(--warn);
  }
</style>
