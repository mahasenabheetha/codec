<script lang="ts">
  import ExternalLink from '@lucide/svelte/icons/external-link'
  import RefreshCw from '@lucide/svelte/icons/refresh-cw'
  import Button from '../../lib/components/Button.svelte'
  import Toggle from '../../lib/components/Toggle.svelte'
  import { openLink } from '../../lib/platform'
  import { updates } from '../../lib/stores/update.svelte'
  import SettingRow from './SettingRow.svelte'

  // Settings → General: the daily release check (off by default) and a
  // "Check now" that asks GitHub once, whichever way the switch is.
  const info = $derived(updates.info)
  const when = (iso?: string) => (iso ? new Date(iso).toLocaleString([], { dateStyle: 'medium', timeStyle: 'short' }) : '')

  const status = $derived.by(() => {
    if (!info) return ''
    if (info.skipped === 'dev') return 'This is a development build; there is no release to compare it with.'
    if (info.error) return `Couldn't check: ${info.error}`
    if (info.newer && info.latest) return `codec ${info.latest.version} is available (you have ${info.current}).`
    if (info.latest) return `You have the latest release, ${info.current}. Checked ${when(info.checked)}.`
    return ''
  })
</script>

<SettingRow
  title="Check for updates"
  description="Once a day, codec asks GitHub for its latest release and tells you in the status bar when there is a newer one. It sends nothing about you or your files. Off by default."
>
  <Toggle checked={info?.enabled ?? false} label={info?.enabled ? 'On' : 'Off'} name="Check for updates" onchange={(on) => updates.setEnabled(on)} />
  <Button size="sm" variant="secondary" icon={RefreshCw} disabled={updates.checking} onclick={() => updates.load(true)}>
    {updates.checking ? 'Checking…' : 'Check now'}
  </Button>
</SettingRow>
{#if status}
  <p class="status" class:newer={info?.newer} role="status">
    {status}
    {#if info?.newer && info.latest}
      <Button size="sm" variant="ghost" icon={ExternalLink} tone="open" onclick={() => openLink(info.latest!.url)}>Release notes and download</Button>
    {/if}
  </p>
{/if}

<style>
  .status {
    display: flex;
    align-items: center;
    flex-wrap: wrap;
    gap: var(--s-2);
    margin: calc(-1 * var(--s-2)) 0 var(--s-2);
    font-size: var(--fs-sm);
    color: var(--fg-1);
  }
  .status.newer {
    color: var(--fg-0);
  }
</style>
