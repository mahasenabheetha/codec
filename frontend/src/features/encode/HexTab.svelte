<script lang="ts">
  import { untrack } from 'svelte'
  import FileDigit from '@lucide/svelte/icons/file-digit'
  import type CodeView from '../../lib/components/CodeView.svelte'
  import Badge from '../../lib/components/Badge.svelte'
  import SegmentedControl from '../../lib/components/SegmentedControl.svelte'
  import Select from '../../lib/components/Select.svelte'
  import Toggle from '../../lib/components/Toggle.svelte'
  import { hexDecode, hexEncode } from '../../lib/api/encode'
  import type { ToolDef } from '../../lib/tools'
  import { copyText } from '../../lib/utils/clipboard'
  import ToolLayout from '../shared/ToolLayout.svelte'
  import ToolTabs from '../shared/ToolTabs.svelte'
  import InputPane from '../shared/InputPane.svelte'
  import { TextJob } from '../shared/job.svelte'
  import { samples } from '../shared/samples'
  import { toolCommands, toolShortcuts, toolStatus } from '../shared/tool.svelte'
  import TextOutput from './TextOutput.svelte'

  let { tool, active }: { tool: ToolDef; active: boolean } = $props()

  const job = new TextJob<{ output: string; text?: boolean; bytes?: number }>()
  let mode = $state<'encode' | 'decode'>('decode')
  let upper = $state(false)
  let sep = $state<'none' | 'space' | 'colon'>('none')
  let editor = $state<CodeView>()

  const encoding = $derived(mode === 'encode')
  const separators = { none: '', space: ' ', colon: ':' }
  const call = () => {
    const input = job.input
    if (!input) return null
    return encoding ? () => hexEncode(input, upper, separators[sep]) : () => hexDecode(input)
  }
  $effect(() => {
    const c = call()
    untrack(() => job.schedule(c))
  })

  const run = () => job.run(call())
  const copy = () => copyText(job.result?.output ?? '')
  function clear() {
    job.clear()
    editor?.focus()
  }
  // Binary output shows \xNN escapes, which wouldn't encode back to the
  // same bytes; swapping is for text only.
  const canSwap = $derived(!!job.result && job.result.text !== false)
  function swap() {
    if (!canSwap) return
    job.input = job.result!.output
    mode = encoding ? 'decode' : 'encode'
    editor?.focus()
  }
  const sample = () => (job.input = encoding ? samples.hexEncode : samples.hexDecode)

  toolShortcuts(() => active, { run, copy, clear, swap })
  toolCommands(() => active, () => [
    { id: 'hex.encode', title: 'Hex: Encode', group: 'Hex', run: () => (mode = 'encode') },
    { id: 'hex.decode', title: 'Hex: Decode', group: 'Hex', run: () => (mode = 'decode') },
    { id: 'hex.swap', title: 'Hex: Use output as input', group: 'Hex', shortcut: 'Alt+S', run: swap },
  ])
  toolStatus(() => active, () => {
    if (job.error) return { text: encoding ? 'Encoding failed' : 'Not valid hex', tone: 'err' }
    const r = job.result
    if (!r) return { text: encoding ? 'Ready to encode' : 'Ready to decode' }
    if (encoding) return { text: 'Encoded', tone: 'ok' }
    return r.text ? { text: `Decoded ${r.bytes} bytes`, tone: 'ok' } : { text: `Decoded ${r.bytes} bytes · not text`, tone: 'warn' }
  })
</script>

<ToolLayout {tool}>
  {#snippet controls()}
    <ToolTabs {tool} />
    <span class="divider" aria-hidden="true"></span>
    <SegmentedControl
      label="Direction"
      bind:value={mode}
      options={[
        { value: 'decode', label: 'Decode' },
        { value: 'encode', label: 'Encode' },
      ]}
    />
    {#if encoding}
      <Select
        label="Between bytes"
        bind:value={sep}
        items={[
          { value: 'none', label: 'No separator' },
          { value: 'space', label: 'Space' },
          { value: 'colon', label: 'Colon' },
        ]}
      />
      <Toggle label="Upper case" bind:checked={upper} />
    {/if}
  {/snippet}
  {#snippet input()}
    <InputPane
      session={job}
      bind:editor
      onclear={clear}
      placeholder={encoding ? 'Text to write as hex…' : 'Hex to decode — spaces, colons, 0x and \\x are fine'}
      onpaste={run}
    />
  {/snippet}
  {#snippet output()}
    <TextOutput
      output={job.result?.output ?? null}
      error={job.error}
      busy={job.busy}
      emptyIcon={FileDigit}
      emptyTitle={encoding ? 'Hex appears here' : 'Decoded text appears here'}
      emptyDescription={encoding ? 'Each byte of the UTF-8 text as two hex digits.' : 'Bytes that are not UTF-8 text are shown as \\xNN.'}
      onsample={sample}
      onswap={canSwap ? swap : undefined}
    >
      {#snippet badges()}
        {#if job.result && job.result.text === false}<Badge tone="warn">not text · bytes as \xNN</Badge>{/if}
      {/snippet}
    </TextOutput>
  {/snippet}
</ToolLayout>

<style>
  .divider {
    width: 1px;
    height: 20px;
    background: var(--border);
  }
</style>
