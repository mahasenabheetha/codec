<script lang="ts">
  import { untrack } from 'svelte'
  import Copy from '@lucide/svelte/icons/copy'
  import IdCard from '@lucide/svelte/icons/id-card'
  import RefreshCw from '@lucide/svelte/icons/refresh-cw'
  import Button from '../../lib/components/Button.svelte'
  import EmptyState from '../../lib/components/EmptyState.svelte'
  import IconButton from '../../lib/components/IconButton.svelte'
  import SegmentedControl from '../../lib/components/SegmentedControl.svelte'
  import Select from '../../lib/components/Select.svelte'
  import { parseUUID, uuids, type UUIDInfo, type UUIDOptions } from '../../lib/api/encode'
  import { handoff } from '../../lib/stores/handoff.svelte'
  import type { ToolDef } from '../../lib/tools'
  import { copyText } from '../../lib/utils/clipboard'
  import { formatShortcut } from '../../lib/utils/platform'
  import ErrorBanner from '../shared/ErrorBanner.svelte'
  import LineInput from '../shared/LineInput.svelte'
  import Pane from '../shared/Pane.svelte'
  import ToolLayout from '../shared/ToolLayout.svelte'
  import ToolTabs from '../shared/ToolTabs.svelte'
  import ValueRows from '../shared/ValueRows.svelte'
  import { Job, TextJob } from '../shared/job.svelte'
  import { toolShortcuts, toolStatus } from '../shared/tool.svelte'

  // UUIDs: made (v4 random, v7 time-ordered, v5 and v3 from a namespace
  // and name), up to 1000 at a time and written the way you need; and
  // one pasted UUID read for its version, variant and time.
  let { tool, active }: { tool: ToolDef; active: boolean } = $props()

  let version = $state<'4' | '7' | '5' | '3'>('4')
  let ns = $state('dns')
  let customNS = $state('')
  let name = $state('')
  let count = $state(1)
  type Format = 'lower' | 'upper' | 'plain' | 'braces' | 'urn'
  let format = $state<Format>('lower')
  const job = new Job<string[]>()
  const parsed = new TextJob<UUIDInfo>()

  const named = $derived(version === '5' || version === '3')
  const call = () => {
    const o: UUIDOptions = { version: Number(version) as UUIDOptions['version'] }
    if (named) {
      o.namespace = ns === 'custom' ? customNS : ns
      o.name = name
    }
    const n = named ? 1 : Math.min(1000, Math.max(1, Math.round(Number(count) || 1)))
    return () => uuids(n, o).then((r) => r.uuids)
  }
  $effect(() => {
    const c = call()
    untrack(() => job.schedule(c, 120))
  })
  const generate = () => job.run(call())

  // A UUID or ULID handed over by smart paste, read below.
  $effect(() => {
    const v = handoff.take('encode', 'uuid')
    if (v !== null) untrack(() => (parsed.input = v))
  })
  $effect(() => {
    const v = parsed.input.trim()
    untrack(() => parsed.schedule(v ? () => parseUUID(v) : null, 120))
  })

  const write = (u: string, f: Format = format) =>
    f === 'upper' ? u.toUpperCase() : f === 'plain' ? u.replaceAll('-', '') : f === 'braces' ? `{${u}}` : f === 'urn' ? `urn:uuid:${u}` : u
  const values = $derived((job.result ?? []).map((u) => write(u)))
  const all = $derived(values.join('\n'))
  const copy = () => copyText(all, values.length > 1 ? 'UUIDs' : 'UUID')

  const info = $derived(parsed.result)
  const rows = $derived(
    info
      ? [
          { label: 'ULID', value: info.ulid },
          { label: info.ulid ? 'As a UUID' : 'UUID', value: info.uuid },
          { label: info.ulid ? 'Kind' : 'Version', value: info.version ? `${info.version} · ${info.kind}` : info.kind },
          { label: 'Variant', value: info.ulid ? undefined : info.variant },
          { label: 'Made', value: info.time ? `${info.time} · ${new Date(info.time).toLocaleString()}` : undefined, copy: info.time },
          { label: 'Upper case', value: write(info.uuid, 'upper') },
          { label: 'No hyphens', value: write(info.uuid, 'plain') },
          { label: 'URN', value: write(info.uuid, 'urn') },
        ]
      : [],
  )

  toolShortcuts(() => active, { run: generate, copy, clear: () => (parsed.input = '') })
  toolStatus(() => active, () => {
    if (job.error) return { text: job.error.message, tone: 'err' }
    if (!values.length) return { text: 'Ready' }
    const what = named ? `version ${version}, the same for this namespace and name` : `version ${version}`
    return { text: `${values.length} UUID${values.length === 1 ? '' : 's'} · ${what}`, tone: 'ok' }
  })
</script>

<ToolLayout {tool}>
  {#snippet controls()}
    <ToolTabs {tool} />
  {/snippet}
  {#snippet actions()}
    <Button variant="primary" icon={RefreshCw} onclick={generate} disabled={named} title={formatShortcut('Mod+Enter')}>Generate</Button>
  {/snippet}
  {#snippet body()}
    <Pane title="Options" grow={false}>
      <div class="form">
        <div class="row">
          <span class="label">Version</span>
          <SegmentedControl
            label="Version"
            bind:value={version}
            options={[
              { value: '4', label: 'v4 · random', title: 'Random: what most systems use' },
              { value: '7', label: 'v7 · time-ordered', title: 'Starts with the time in milliseconds, so new ones sort last: good database keys' },
              { value: '5', label: 'v5 · name', title: 'The same UUID every time for a namespace and name (SHA-1)' },
              { value: '3', label: 'v3 · name (MD5)', title: 'As v5 but with MD5: only to match an existing v3' },
            ]}
          />
        </div>
        {#if named}
          <div class="row">
            <span class="label">Namespace</span>
            <Select
              label="Namespace"
              bind:value={ns}
              items={[
                { value: 'dns', label: 'DNS name' },
                { value: 'url', label: 'URL' },
                { value: 'oid', label: 'ISO OID' },
                { value: 'x500', label: 'X.500 DN' },
                { value: 'custom', label: 'A UUID of your own' },
              ]}
            />
            {#if ns === 'custom'}<input class="text" bind:value={customNS} placeholder="namespace UUID" aria-label="Namespace UUID" spellcheck="false" />{/if}
          </div>
          <div class="row">
            <label class="label" for="uuid-name">Name</label>
            <input id="uuid-name" class="text wide" bind:value={name} placeholder={ns === 'url' ? 'https://example.com/' : 'www.example.com'} spellcheck="false" />
          </div>
        {:else}
          <div class="row">
            <label class="label" for="uuid-count">How many</label>
            <input id="uuid-count" class="num" type="number" min="1" max="1000" bind:value={count} />
          </div>
        {/if}
        <div class="row">
          <span class="label">Written</span>
          <Select
            label="Written as"
            bind:value={format}
            items={[
              { value: 'lower', label: 'Standard (lower case)' },
              { value: 'upper', label: 'UPPER CASE' },
              { value: 'plain', label: 'No hyphens' },
              { value: 'braces', label: '{In braces}' },
              { value: 'urn', label: 'urn:uuid:' },
            ]}
          />
        </div>
      </div>
    </Pane>
    <Pane title={values.length > 1 ? `${values.length} UUIDs` : 'UUID'} grow={values.length > 3}>
      {#snippet actions()}
        <IconButton icon={Copy} tone="copy" label={values.length > 1 ? 'Copy all' : 'Copy'} shortcut="Alt+C" size="sm" disabled={!values.length} onclick={copy} />
      {/snippet}
      {#if job.error}<ErrorBanner error={job.error} />{/if}
      {#if values.length}
        <ul class="values">
          {#each values as v, i (i)}
            <li>
              <code>{v}</code>
              <IconButton icon={Copy} tone="copy" label="Copy" size="sm" onclick={() => copyText(v, 'UUID')} />
            </li>
          {/each}
        </ul>
      {:else if !job.error}
        <EmptyState icon={IdCard} title={job.busy ? 'Generating…' : 'Press Generate'} />
      {/if}
    </Pane>
    <Pane title="Read a UUID" grow={false}>
      <div class="parse">
        <LineInput bind:value={parsed.input} label="UUID to read" placeholder="018f… or {'{'}…{'}'} or urn:uuid:…" invalid={!!parsed.error} />
        {#if parsed.error}<ErrorBanner error={parsed.error} />{:else if info}<ValueRows {rows} />{/if}
      </div>
    </Pane>
  {/snippet}
</ToolLayout>

<style>
  .form {
    padding: var(--s-4);
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: var(--s-4) var(--s-6);
  }
  .row {
    display: flex;
    align-items: center;
    gap: var(--s-3);
  }
  .label {
    font-size: var(--fs-sm);
    color: var(--fg-1);
  }
  .num,
  .text {
    height: var(--control-h);
    padding: 0 var(--s-2);
    font-family: var(--font-mono);
    font-size: var(--fs-sm);
    color: var(--fg-0);
    background: var(--bg-0);
    border: 1px solid var(--border-strong);
    border-radius: var(--r-sm);
  }
  .num {
    width: 80px;
  }
  .text {
    width: 300px;
  }
  .text.wide {
    width: 340px;
  }
  .num:focus,
  .text:focus {
    outline: none;
    border-color: var(--accent);
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
    padding: var(--s-1) var(--s-2) var(--s-1) var(--s-3);
    background: var(--bg-1);
    border: 1px solid var(--border);
    border-radius: var(--r-md);
  }
  code {
    /* As wide as the value, so Copy sits right after it. */
    flex: 0 1 auto;
    min-width: 0;
    font-size: var(--fs-md);
    color: var(--fg-0);
    overflow-wrap: anywhere;
    user-select: all;
  }
  .parse {
    padding: var(--s-3);
    display: flex;
    flex-direction: column;
    gap: var(--s-3);
  }
</style>
