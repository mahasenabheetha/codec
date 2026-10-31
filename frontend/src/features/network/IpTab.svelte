<script lang="ts">
  import { untrack } from 'svelte'
  import Binary from '@lucide/svelte/icons/binary'
  import Badge from '../../lib/components/Badge.svelte'
  import Button from '../../lib/components/Button.svelte'
  import EmptyState from '../../lib/components/EmptyState.svelte'
  import { ipInfo, type IPInfo } from '../../lib/api/net'
  import { handoff } from '../../lib/stores/handoff.svelte'
  import type { ToolDef } from '../../lib/tools'
  import { copyText } from '../../lib/utils/clipboard'
  import ErrorBanner from '../shared/ErrorBanner.svelte'
  import LineInput from '../shared/LineInput.svelte'
  import Pane from '../shared/Pane.svelte'
  import ToolLayout from '../shared/ToolLayout.svelte'
  import ToolTabs from '../shared/ToolTabs.svelte'
  import { TextJob } from '../shared/job.svelte'
  import { toolShortcuts, toolStatus } from '../shared/tool.svelte'
  import Rows from '../shared/ValueRows.svelte'

  let { tool, active }: { tool: ToolDef; active: boolean } = $props()

  const job = new TextJob<{ ip: IPInfo }>()
  let field = $state<HTMLInputElement>()

  $effect(() => {
    const input = job.input.trim()
    untrack(() => job.schedule(input ? () => ipInfo(input) : null))
  })
  // An address handed over by smart paste.
  $effect(() => {
    const v = handoff.take('network', 'ip')
    if (v !== null) untrack(() => (job.input = v))
  })
  const ip = $derived(job.result?.ip)
  const rows = $derived(
    ip
      ? [
          { label: 'Address', value: ip.address },
          { label: 'Expanded', value: ip.expanded },
          { label: 'Integer', value: ip.integer },
          { label: 'Hex', value: ip.hex },
          { label: 'Binary', value: ip.binary },
          { label: ip.version === 4 ? 'IPv4-mapped IPv6' : 'IPv4 inside', value: ip.mapped },
          { label: 'Reverse DNS', value: ip.reverse },
        ]
      : [],
  )

  function clear() {
    job.clear()
    field?.focus()
  }

  const run = () => {
    const input = job.input.trim()
    job.run(input ? () => ipInfo(input) : null)
  }

  toolShortcuts(() => active, { run, copy: () => ip && copyText(ip.address, 'Address'), clear })
  toolStatus(() => active, () => {
    if (job.error) return { text: 'Not an IP address', tone: 'err' }
    if (ip) return { text: `IPv${ip.version} · ${ip.kind}`, tone: 'ok' }
    return { text: 'Ready' }
  })
</script>

<ToolLayout {tool}>
  {#snippet controls()}
    <ToolTabs {tool} />
  {/snippet}
  {#snippet body()}
    <LineInput bind:value={job.input} bind:field label="IP address" invalid={!!job.error} onclear={clear} placeholder="10.0.0.1, 2001:db8::1, 167772161 or 0x0a000001">
      <p class="hint">An IPv4 or IPv6 address, or the address as a number: decimal, 0x hex or 0b binary (more than 32 bits, or more than 8 hex digits, is IPv6).</p>
    </LineInput>
    {#if job.error}
      <ErrorBanner error={job.error} />
    {/if}
    <Pane title="Address" grow={false}>
      {#snippet meta()}
        {#if ip}
          <Badge tone={ip.kind === 'public' ? 'neutral' : 'accent'}>{ip.kind}</Badge>
          <Badge>IPv{ip.version}</Badge>
        {/if}
      {/snippet}
      {#if ip}
        <Rows {rows} />
      {:else if !job.error}
        <EmptyState icon={Binary} title="The address in every notation appears here" description="As a number in decimal, hex and binary, IPv6 written out in full, and its reverse-DNS name.">
          <Button size="sm" onclick={() => (job.input = '192.168.10.25')}>Try an example</Button>
        </EmptyState>
      {/if}
    </Pane>
  {/snippet}
</ToolLayout>

<style>
  .hint {
    font-size: var(--fs-sm);
    color: var(--fg-2);
  }
</style>
