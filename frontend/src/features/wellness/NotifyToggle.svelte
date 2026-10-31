<script lang="ts">
  import Toggle from '../../lib/components/Toggle.svelte'
  import { notifications } from './notify'

  // Switch system notifications on (asking the browser) or off.
  let on = $state(notifications.on)
  const denied = $derived(notifications.permission === 'denied')
</script>

{#if notifications.supported}
  <div class="notify">
    <Toggle
      bind:checked={on}
      label="Notify me when codec is in the background"
      disabled={denied}
      onchange={async (v) => (on = await notifications.set(v))}
    />
    {#if denied}<span class="note">Notifications are blocked for this page; allow them in the browser's site settings.</span>{/if}
  </div>
{/if}

<style>
  .notify {
    display: flex;
    flex-direction: column;
    gap: var(--s-1);
  }
  .note {
    padding-left: var(--s-6);
    font-size: var(--fs-sm);
    color: var(--fg-2);
  }
</style>
