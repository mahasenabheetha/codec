<script lang="ts">
  import { untrack } from 'svelte'
  import Copy from '@lucide/svelte/icons/copy'
  import Eye from '@lucide/svelte/icons/eye'
  import EyeOff from '@lucide/svelte/icons/eye-off'
  import CircleCheck from '@lucide/svelte/icons/circle-check'
  import FingerprintPattern from '@lucide/svelte/icons/fingerprint-pattern'
  import type CodeView from '../../lib/components/CodeView.svelte'
  import Badge from '../../lib/components/Badge.svelte'
  import Button from '../../lib/components/Button.svelte'
  import EmptyState from '../../lib/components/EmptyState.svelte'
  import IconButton from '../../lib/components/IconButton.svelte'
  import SegmentedControl from '../../lib/components/SegmentedControl.svelte'
  import Toggle from '../../lib/components/Toggle.svelte'
  import { hash, type Digest } from '../../lib/api/encode'
  import type { ToolDef } from '../../lib/tools'
  import { copyText } from '../../lib/utils/clipboard'
  import ToolLayout from '../shared/ToolLayout.svelte'
  import ToolTabs from '../shared/ToolTabs.svelte'
  import InputPane from '../shared/InputPane.svelte'
  import Pane from '../shared/Pane.svelte'
  import ErrorBanner from '../shared/ErrorBanner.svelte'
  import { TextJob } from '../shared/job.svelte'
  import { samples } from '../shared/samples'
  import { toolCommands, toolShortcuts, toolStatus } from '../shared/tool.svelte'

  let { tool, active }: { tool: ToolDef; active: boolean } = $props()

  const job = new TextJob<{ digests: Digest[]; hmac: boolean }>()
  let hmac = $state(false)
  let key = $state('')
  let showKey = $state(false)
  let format = $state<'hex' | 'base64'>('hex')
  let expected = $state('')
  let editor = $state<CodeView>()

  // The exact text is hashed: spaces and line breaks count.
  const call = () => {
    const input = job.input
    const k = hmac ? key : null
    if (!input) return null
    return () => hash(input, k)
  }
  $effect(() => {
    const c = call()
    untrack(() => job.schedule(c))
  })

  const digests = $derived(job.result?.digests ?? [])
  const value = (d: Digest) => (format === 'hex' ? d.hex : d.base64)
  // A pasted digest is matched in either form, ignoring case and spaces in hex.
  const want = $derived(expected.trim().replace(/\s+/g, ''))
  const matchOf = (d: Digest) => !!want && (d.hex === want.toLowerCase() || d.base64 === want)
  const matched = $derived(digests.find(matchOf))
  const prefix = $derived(job.result?.hmac ? 'HMAC-' : '')

  const run = () => job.run(call())
  const copy = () => digests[0] && copyText(value(digests[0]), prefix + digests[0].algorithm)
  function clear() {
    job.clear()
    editor?.focus()
  }
  const sample = () => (job.input = samples.hash)

  toolShortcuts(() => active, { run, copy, clear })
  toolCommands(() => active, () => [
    { id: 'hash.hmac', title: 'Hash: Toggle HMAC', group: 'Hash', run: () => (hmac = !hmac) },
    { id: 'hash.format', title: 'Hash: Switch hex / base64', group: 'Hash', run: () => (format = format === 'hex' ? 'base64' : 'hex') },
  ])
  toolStatus(() => active, () => {
    if (job.error) return { text: 'Hashing failed', tone: 'err' }
    if (!job.result) return { text: hmac ? 'Ready · HMAC' : 'Ready to hash' }
    if (want) return matched ? { text: `Matches ${prefix}${matched.algorithm}`, tone: 'ok' } : { text: 'No digest matches the expected value', tone: 'warn' }
    return { text: job.result.hmac ? 'HMACs computed' : 'Hashed', tone: 'ok' }
  })
</script>

<ToolLayout {tool}>
  {#snippet controls()}
    <ToolTabs {tool} />
    <span class="divider" aria-hidden="true"></span>
    <Toggle label="HMAC" bind:checked={hmac} title="Keyed hashes (HMAC) instead of plain digests" />
    {#if hmac}
      <span class="key">
        <input
          type={showKey ? 'text' : 'password'}
          bind:value={key}
          placeholder="Key"
          aria-label="HMAC key"
          autocomplete="off"
          spellcheck="false"
        />
        <IconButton icon={showKey ? EyeOff : Eye} label={showKey ? 'Hide key' : 'Show key'} size="sm" onclick={() => (showKey = !showKey)} />
      </span>
    {/if}
  {/snippet}
  {#snippet input()}
    <InputPane session={job} bind:editor onclear={clear} placeholder="Text to hash — exactly as typed, spaces and line breaks included" onpaste={run} />
  {/snippet}
  {#snippet output()}
    <Pane title={job.result?.hmac ? 'HMAC' : 'Digests'}>
      {#snippet actions()}
        <SegmentedControl
          label="Format"
          bind:value={format}
          options={[
            { value: 'hex', label: 'Hex' },
            { value: 'base64', label: 'Base64' },
          ]}
        />
      {/snippet}
      {#if job.error}
        <ErrorBanner error={job.error} />
      {/if}
      {#if job.result}
        <div class="list">
          <label class="expected">
            <span>Expected</span>
            <input bind:value={expected} placeholder="Paste a digest to compare (hex or base64)" spellcheck="false" autocomplete="off" />
          </label>
          {#each digests as d (d.algorithm)}
            {@const hit = matchOf(d)}
            <div class="row" class:hit>
              <div class="name">
                {prefix}{d.algorithm}
                {#if d.weak}<Badge tone="warn">checksums only</Badge>{/if}
                {#if hit}<span class="match"><CircleCheck size={14} strokeWidth={2} /> matches</span>{/if}
              </div>
              <code>{value(d)}</code>
              <IconButton icon={Copy} label="Copy {prefix}{d.algorithm}" size="sm" onclick={() => copyText(value(d), prefix + d.algorithm)} />
            </div>
          {/each}
          {#if want && !matched}
            <p class="nomatch">No digest above matches the expected value.</p>
          {/if}
        </div>
      {:else if !job.error}
        <EmptyState
          icon={FingerprintPattern}
          title="Digests appear here"
          description="SHA-256, SHA-384, SHA-512, SHA-1 and MD5 of the text. MD5 and SHA-1 are for checksums, not security."
        >
          <Button size="sm" onclick={sample}>Try a sample</Button>
        </EmptyState>
      {/if}
    </Pane>
  {/snippet}
</ToolLayout>

<style>
  .divider {
    width: 1px;
    height: 20px;
    background: var(--border);
  }
  .key {
    display: inline-flex;
    align-items: center;
    gap: 2px;
  }
  input {
    height: var(--control-h);
    padding: 0 var(--s-2);
    font-family: var(--font-mono);
    font-size: var(--fs-sm);
    color: var(--fg-0);
    background: var(--bg-0);
    border: 1px solid var(--border-strong);
    border-radius: var(--r-sm);
  }
  input:focus {
    outline: none;
    border-color: var(--accent);
  }
  .key input {
    width: 200px;
  }
  .list {
    flex: 1;
    min-height: 0;
    overflow: auto;
    padding: var(--s-3);
    display: flex;
    flex-direction: column;
    gap: var(--s-2);
  }
  .expected {
    display: flex;
    align-items: center;
    gap: var(--s-2);
    margin-bottom: var(--s-1);
    font-size: var(--fs-sm);
    color: var(--fg-1);
  }
  .expected input {
    flex: 1;
    min-width: 0;
  }
  .row {
    display: grid;
    grid-template-columns: 1fr auto;
    align-items: center;
    gap: var(--s-1) var(--s-2);
    padding: var(--s-2) var(--s-2) var(--s-2) var(--s-3);
    background: var(--bg-1);
    border: 1px solid var(--border);
    border-radius: var(--r-md);
  }
  .row.hit {
    border-color: var(--ok);
    background: var(--ok-soft);
  }
  .name {
    display: flex;
    align-items: center;
    gap: var(--s-2);
    font-size: var(--fs-sm);
    font-weight: var(--fw-semibold);
    color: var(--fg-1);
  }
  code {
    grid-column: 1;
    font-size: var(--fs-sm);
    color: var(--fg-0);
    overflow-wrap: anywhere;
    user-select: all;
  }
  .row :global(button) {
    grid-column: 2;
    grid-row: 1 / span 2;
  }
  .match {
    display: inline-flex;
    align-items: center;
    gap: 4px;
    color: var(--ok);
    font-weight: var(--fw-medium);
  }
  .nomatch {
    font-size: var(--fs-sm);
    color: var(--warn);
  }
</style>
