<script lang="ts">
  import { untrack } from 'svelte'
  import Copy from '@lucide/svelte/icons/copy'
  import Dices from '@lucide/svelte/icons/dices'
  import RefreshCw from '@lucide/svelte/icons/refresh-cw'
  import Button from '../../lib/components/Button.svelte'
  import EmptyState from '../../lib/components/EmptyState.svelte'
  import IconButton from '../../lib/components/IconButton.svelte'
  import SegmentedControl from '../../lib/components/SegmentedControl.svelte'
  import Select from '../../lib/components/Select.svelte'
  import Toggle from '../../lib/components/Toggle.svelte'
  import { secrets, uuids, type SecretOptions } from '../../lib/api/encode'
  import type { ToolDef } from '../../lib/tools'
  import { copyText } from '../../lib/utils/clipboard'
  import { formatShortcut } from '../../lib/utils/platform'
  import ToolLayout from '../shared/ToolLayout.svelte'
  import ToolTabs from '../shared/ToolTabs.svelte'
  import Pane from '../shared/Pane.svelte'
  import ErrorBanner from '../shared/ErrorBanner.svelte'
  import { Job } from '../shared/job.svelte'
  import { toolShortcuts, toolStatus } from '../shared/tool.svelte'

  let { tool, active }: { tool: ToolDef; active: boolean } = $props()

  type Item = { value: string; bits?: number }
  const job = new Job<Item[]>()
  let kind = $state<'secret' | 'uuid'>('secret')
  let count = $state(1)
  const opts = $state<SecretOptions>({ length: 32, format: 'text', lower: true, upper: true, digits: true, symbols: false, noAmbiguous: false })

  const text = $derived(opts.format === 'text')
  const noSet = $derived(text && !opts.lower && !opts.upper && !opts.digits && !opts.symbols)
  const call = () => {
    const n = clamp(count, 1, 100)
    if (kind === 'secret' && noSet) return null
    if (kind === 'uuid') return () => uuids(n).then((r) => r.uuids.map((value) => ({ value })))
    const o = { ...opts, length: clamp(opts.length, 1, 1024) }
    return () => secrets(o, n).then((r) => r.secrets)
  }
  // New values whenever an option changes; Generate makes more.
  $effect(() => {
    const c = call()
    untrack(() => job.schedule(c, 120))
  })
  const generate = () => job.run(call())

  const values = $derived(job.result ?? [])
  const all = $derived(values.map((v) => v.value).join('\n'))
  const copy = () => copyText(all, values.length > 1 ? 'Values' : kind === 'uuid' ? 'UUID' : 'Secret')

  function clamp(n: number, lo: number, hi: number) {
    return Math.min(hi, Math.max(lo, Math.round(Number(n) || lo)))
  }
  // Bits of entropy in words. 128 bits is the usual bar for keys and tokens.
  function strength(bits: number) {
    if (bits >= 128) return { label: 'strong', tone: 'ok' }
    if (bits >= 72) return { label: 'good', tone: 'ok' }
    if (bits >= 48) return { label: 'fair', tone: 'warn' }
    return { label: 'weak', tone: 'err' }
  }
  const bits = $derived(values[0]?.bits)

  toolShortcuts(() => active, { run: generate, copy, clear: () => job.reset() })
  toolStatus(() => active, () => {
    if (job.error) return { text: job.error.message, tone: 'err' }
    if (!values.length) return { text: 'Ready' }
    if (bits === undefined) return { text: `${values.length} UUID${values.length === 1 ? '' : 's'} · version 4`, tone: 'ok' }
    const s = strength(bits)
    return { text: `${values.length === 1 ? 'Secret' : values.length + ' secrets'} · ${bits} bits · ${s.label}`, tone: s.tone as 'ok' | 'warn' | 'err' }
  })
</script>

<ToolLayout {tool}>
  {#snippet controls()}
    <ToolTabs {tool} />
  {/snippet}
  {#snippet actions()}
    <Button variant="primary" icon={RefreshCw} onclick={generate} title={formatShortcut('Mod+Enter')}>Generate</Button>
  {/snippet}
  {#snippet input()}
    <Pane title="Options">
      <div class="form">
        <div class="row">
          <span class="label">Kind</span>
          <SegmentedControl
            label="Kind"
            bind:value={kind}
            options={[
              { value: 'secret', label: 'Secret' },
              { value: 'uuid', label: 'UUID' },
            ]}
          />
        </div>
        {#if kind === 'secret'}
          <div class="row">
            <span class="label">Format</span>
            <Select
              label="Format"
              bind:value={opts.format}
              items={[
                { value: 'text', label: 'Text' },
                { value: 'hex', label: 'Hex' },
                { value: 'base64', label: 'Base64' },
                { value: 'base64url', label: 'Base64 URL-safe' },
              ]}
            />
          </div>
          <div class="row">
            <label class="label" for="secret-length">Length</label>
            <div class="length">
              <input type="range" min="4" max="128" bind:value={opts.length} aria-label="Length" />
              <input id="secret-length" class="num" type="number" min="1" max="1024" bind:value={opts.length} />
              <span class="unit">{text ? 'characters' : 'bytes'}</span>
            </div>
          </div>
          {#if text}
            <div class="row top">
              <span class="label">Characters</span>
              <div class="sets">
                <Toggle label="a–z" bind:checked={opts.lower} />
                <Toggle label="A–Z" bind:checked={opts.upper} />
                <Toggle label="0–9" bind:checked={opts.digits} />
                <Toggle label="Symbols" bind:checked={opts.symbols} title="!#$%&()*+,-./:;<=>?@[]^_{'{'}|{'}'}~ — no quotes, backslash, backtick or space" />
                <Toggle label="No look-alikes" bind:checked={opts.noAmbiguous} title="Leave out I l 1 O 0 o" />
              </div>
            </div>
          {/if}
        {/if}
        <div class="row">
          <label class="label" for="secret-count">How many</label>
          <input id="secret-count" class="num" type="number" min="1" max="100" bind:value={count} />
        </div>
        <p class="hint">
          Made with the operating system's secure random source, on this machine. Nothing is stored.
          {#if kind === 'secret' && text}A text secret has at least one character of each chosen set.{/if}
        </p>
      </div>
    </Pane>
  {/snippet}
  {#snippet output()}
    <Pane title={kind === 'uuid' ? 'UUIDs' : 'Secrets'}>
      {#snippet meta()}
        {#if bits !== undefined}
          {@const s = strength(bits)}
          <span class="strength {s.tone}">{bits} bits · {s.label}</span>
        {/if}
      {/snippet}
      {#snippet actions()}
        <IconButton icon={Copy} label={values.length > 1 ? 'Copy all' : 'Copy'} shortcut="Alt+C" size="sm" disabled={!values.length} onclick={copy} />
      {/snippet}
      {#if job.error}
        <ErrorBanner error={job.error} />
      {/if}
      {#if values.length}
        <ul class="values">
          {#each values as v, i (i)}
            <li>
              <code>{v.value}</code>
              <IconButton icon={Copy} label="Copy" size="sm" onclick={() => copyText(v.value, kind === 'uuid' ? 'UUID' : 'Secret')} />
            </li>
          {/each}
        </ul>
      {:else if !job.error}
        <EmptyState icon={Dices} title={kind === 'secret' && noSet ? 'Choose at least one character set' : 'Generating…'} />
      {/if}
    </Pane>
  {/snippet}
</ToolLayout>

<style>
  .form {
    flex: 1;
    min-height: 0;
    overflow: auto;
    padding: var(--s-4);
    display: flex;
    flex-direction: column;
    gap: var(--s-3);
  }
  .row {
    display: grid;
    grid-template-columns: 96px 1fr;
    align-items: center;
    gap: var(--s-3);
  }
  .row.top {
    align-items: start;
  }
  .row.top .label {
    padding-top: 4px;
  }
  .label {
    font-size: var(--fs-sm);
    color: var(--fg-1);
  }
  .length {
    display: flex;
    align-items: center;
    gap: var(--s-2);
  }
  input[type='range'] {
    flex: 1;
    min-width: 80px;
    accent-color: var(--accent);
  }
  .num {
    width: 72px;
    height: var(--control-h);
    padding: 0 var(--s-2);
    font-family: var(--font-mono);
    font-size: var(--fs-sm);
    color: var(--fg-0);
    background: var(--bg-0);
    border: 1px solid var(--border-strong);
    border-radius: var(--r-sm);
  }
  .num:focus {
    outline: none;
    border-color: var(--accent);
  }
  .unit {
    font-size: var(--fs-sm);
    color: var(--fg-2);
  }
  .sets {
    display: flex;
    flex-wrap: wrap;
    gap: var(--s-2) var(--s-4);
  }
  .hint {
    font-size: var(--fs-sm);
    color: var(--fg-2);
  }
  .strength.ok {
    color: var(--ok);
  }
  .strength.warn {
    color: var(--warn);
  }
  .strength.err {
    color: var(--err);
  }
  .values {
    flex: 1;
    min-height: 0;
    overflow: auto;
    margin: 0;
    padding: var(--s-3);
    list-style: none;
    display: flex;
    flex-direction: column;
    gap: var(--s-2);
  }
  li {
    display: flex;
    align-items: center;
    gap: var(--s-2);
    padding: var(--s-2) var(--s-2) var(--s-2) var(--s-3);
    background: var(--bg-1);
    border: 1px solid var(--border);
    border-radius: var(--r-md);
  }
  code {
    flex: 1;
    min-width: 0;
    font-size: var(--fs-md);
    color: var(--fg-0);
    overflow-wrap: anywhere;
    user-select: all;
  }
</style>
