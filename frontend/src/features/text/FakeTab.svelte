<script lang="ts">
  import { onMount, untrack } from 'svelte'
  import Dices from '@lucide/svelte/icons/dices'
  import Pin from '@lucide/svelte/icons/pin'
  import Plus from '@lucide/svelte/icons/plus'
  import RefreshCw from '@lucide/svelte/icons/refresh-cw'
  import TableProperties from '@lucide/svelte/icons/table-properties'
  import X from '@lucide/svelte/icons/x'
  import Badge from '../../lib/components/Badge.svelte'
  import Button from '../../lib/components/Button.svelte'
  import IconButton from '../../lib/components/IconButton.svelte'
  import SegmentedControl from '../../lib/components/SegmentedControl.svelte'
  import Select from '../../lib/components/Select.svelte'
  import Toggle from '../../lib/components/Toggle.svelte'
  import { fakeKinds, fakeRows, lorem, type FakeColumn, type FakeFormat } from '../../lib/api/fake'
  import type { ToolDef } from '../../lib/tools'
  import { copyText } from '../../lib/utils/clipboard'
  import { formatShortcut } from '../../lib/utils/platform'
  import TextOutput from '../encode/TextOutput.svelte'
  import Pane from '../shared/Pane.svelte'
  import ToolLayout from '../shared/ToolLayout.svelte'
  import ToolTabs from '../shared/ToolTabs.svelte'
  import { Job } from '../shared/job.svelte'
  import { toolShortcuts, toolStatus } from '../shared/tool.svelte'

  // Made-up data for tests and demos: rows from columns you pick, or
  // lorem ipsum. A seed gives the same output again; leave it empty for
  // new values each time. Names, emails and usernames in a row belong to
  // one made-up person; emails, IPs and phones use reserved ranges.
  let { tool, active }: { tool: ToolDef; active: boolean } = $props()

  type Mode = 'rows' | 'lorem'
  let mode = $state<Mode>('rows')
  let kinds = $state<{ Kind: string; About: string }[]>([])
  let presets = $state<Record<string, FakeColumn[]>>({})
  // Choices are edited as one line, split by commas.
  type Col = FakeColumn & { key: number; choiceText: string }
  let nextKey = 0
  const toCol = (c: FakeColumn): Col => ({ ...c, key: ++nextKey, choiceText: (c.choices ?? []).join(', ') })
  let cols = $state<Col[]>([])
  let count = $state(10)
  let format = $state<FakeFormat>('json')
  let tableName = $state('items')
  let seedText = $state('')
  let unit = $state<'words' | 'sentences' | 'paragraphs'>('paragraphs')
  let loremCount = $state(3)
  let classic = $state(true)
  const job = new Job<{ output: string; seed: number }>()

  onMount(async () => {
    const k = await fakeKinds()
    kinds = k.kinds
    presets = k.presets
    if (!cols.length) cols = k.presets.users.map(toCol)
  })

  const ranged = (k: string) => k === 'integer' || k === 'number' || k === 'date' || k === 'datetime'
  const placeholder = (k: string, end: 'min' | 'max') =>
    k === 'integer' ? (end === 'min' ? '0' : '1000') : k === 'number' ? (end === 'min' ? '0' : '1000') : end === 'min' ? '2024-01-01' : '2026-01-01'

  const call = () => {
    const seed = /^\d+$/.test(seedText.trim()) ? Number(seedText.trim()) : 0
    if (mode === 'lorem') {
      const u = unit
      const n = Math.min(1000, Math.max(1, Math.round(Number(loremCount) || 1)))
      const c = classic
      return () => lorem(u, n, c, seed)
    }
    if (!cols.length) return null
    const columns: FakeColumn[] = cols.map((c) => ({
      name: c.name.trim(),
      kind: c.kind,
      min: ranged(c.kind) ? c.min?.trim() || undefined : undefined,
      max: ranged(c.kind) ? c.max?.trim() || undefined : undefined,
      decimals: c.kind === 'number' && c.decimals ? Number(c.decimals) : undefined,
      choices: c.kind === 'choice' ? c.choiceText.split(/[,|]/).map((s) => s.trim()).filter(Boolean) : undefined,
      blank: Number(c.blank) || undefined,
    }))
    const n = Math.min(10000, Math.max(1, Math.round(Number(count) || 1)))
    const f = format
    const t = tableName.trim() || 'items'
    return () => fakeRows(columns, n, f, t, seed)
  }
  $effect(() => {
    const c = call()
    untrack(() => job.schedule(c, 250))
  })
  const generate = () => job.run(call())

  function addColumn() {
    let n = cols.length + 1
    while (cols.some((c) => c.name === `column_${n}`)) n++
    cols.push(toCol({ name: `column_${n}`, kind: 'word' }))
  }
  const usePreset = (name: string) => ((cols = presets[name].map(toCol)), (mode = 'rows'))
  const keepSeed = () => job.result && (seedText = String(job.result.seed))
  const language = $derived(mode === 'lorem' ? 'text' : format === 'json' || format === 'ndjson' ? 'json' : format === 'yaml' ? 'yaml' : 'text')
  const kindItems = $derived(kinds.map((k) => ({ value: k.Kind, label: k.Kind })))
  const about = (k: string) => kinds.find((x) => x.Kind === k)?.About ?? ''

  toolShortcuts(() => active, { run: generate, copy: () => copyText(job.result?.output ?? ''), clear: () => job.reset() })
  toolStatus(() => active, () => {
    if (job.error) return { text: job.error.message, tone: 'err' }
    if (!job.result) return { text: 'Ready' }
    const what = mode === 'lorem' ? `${loremCount} ${unit}` : `${count} rows · ${cols.length} columns · ${format.toUpperCase()}`
    return { text: `${what} · seed ${job.result.seed}`, tone: 'ok' }
  })
</script>

<ToolLayout {tool}>
  {#snippet controls()}
    <ToolTabs {tool} />
    <SegmentedControl
      label="Make"
      bind:value={mode}
      options={[
        { value: 'rows', label: 'Rows' },
        { value: 'lorem', label: 'Lorem ipsum' },
      ]}
    />
  {/snippet}
  {#snippet actions()}
    <Button variant="primary" icon={RefreshCw} onclick={generate} title={formatShortcut('Mod+Enter')}>Generate</Button>
  {/snippet}
  {#snippet body()}
    <Pane title={mode === 'rows' ? 'Columns' : 'Lorem ipsum'} grow={false}>
      {#snippet meta()}
        {#if mode === 'rows'}<span>{cols.length} {cols.length === 1 ? 'column' : 'columns'}</span>{/if}
      {/snippet}
      <div class="opts">
        {#if mode === 'rows'}
          <div class="presets">
            <span class="label">Start from</span>
            {#each Object.keys(presets) as p (p)}
              <button type="button" class="preset" onclick={() => usePreset(p)}>{p}</button>
            {/each}
          </div>
          <div class="cols">
            {#each cols as c, i (c.key)}
              <div class="col">
                <input class="text name" bind:value={c.name} aria-label="Column {i + 1} name" spellcheck="false" />
                <Select label="Kind of column {i + 1}" bind:value={c.kind} items={kindItems} />
                {#if ranged(c.kind)}
                  <input class="text small" bind:value={c.min} placeholder={placeholder(c.kind, 'min')} aria-label="Minimum" spellcheck="false" />
                  <span class="dim">to</span>
                  <input class="text small" bind:value={c.max} placeholder={placeholder(c.kind, 'max')} aria-label="Maximum" spellcheck="false" />
                  {#if c.kind === 'number'}<input class="text tiny" type="number" min="0" max="10" bind:value={c.decimals} placeholder="2" aria-label="Decimals" title="Decimals" />{/if}
                {:else if c.kind === 'choice'}
                  <input class="text wide" bind:value={c.choiceText} placeholder="free, pro, team" aria-label="Choices, separated by commas" spellcheck="false" />
                {:else}
                  <span class="about">{about(c.kind)}</span>
                {/if}
                <span class="grow"></span>
                <label class="blank" title="Share of rows left empty: null, NULL or an empty cell">
                  <input class="text tiny" type="number" min="0" max="100" bind:value={c.blank} placeholder="0" aria-label="Percent blank" />% blank
                </label>
                <IconButton icon={X} label="Remove {c.name}" size="sm" onclick={() => cols.splice(i, 1)} />
              </div>
            {/each}
            <div><Button size="sm" variant="ghost" icon={Plus} onclick={addColumn} disabled={cols.length >= 50}>Add column</Button></div>
          </div>
          <div class="row">
            <label class="label" for="fake-count">Rows</label>
            <input id="fake-count" class="text tiny wide-num" type="number" min="1" max="10000" bind:value={count} />
            <Select
              label="Format"
              bind:value={format}
              items={[
                { value: 'json', label: 'JSON' },
                { value: 'ndjson', label: 'NDJSON' },
                { value: 'csv', label: 'CSV' },
                { value: 'yaml', label: 'YAML' },
                { value: 'sql', label: 'SQL INSERT' },
              ]}
            />
            {#if format === 'sql'}<input class="text small" bind:value={tableName} aria-label="Table name" placeholder="table" spellcheck="false" />{/if}
          </div>
        {:else}
          <div class="row">
            <SegmentedControl
              label="Unit"
              bind:value={unit}
              options={[
                { value: 'words', label: 'Words' },
                { value: 'sentences', label: 'Sentences' },
                { value: 'paragraphs', label: 'Paragraphs' },
              ]}
            />
            <input class="text tiny wide-num" type="number" min="1" max="1000" bind:value={loremCount} aria-label="How many" />
            <Toggle label='Start with "Lorem ipsum"' bind:checked={classic} />
          </div>
        {/if}
        <div class="row">
          <label class="label" for="fake-seed">Seed</label>
          <input id="fake-seed" class="text small" bind:value={seedText} placeholder="new each time" inputmode="numeric" spellcheck="false" />
          {#if seedText}<IconButton icon={Dices} label="New values each time" size="sm" onclick={() => (seedText = '')} />{:else}<IconButton icon={Pin} label="Keep this seed" size="sm" disabled={!job.result} onclick={keepSeed} />{/if}
          <span class="dim">The same seed and options give the same output, for tests that must not change.</span>
        </div>
      </div>
    </Pane>
    <TextOutput
      output={job.result?.output ?? null}
      error={job.error}
      busy={job.busy}
      {language}
      emptyIcon={TableProperties}
      emptyTitle="Made-up data appears here"
      emptyDescription="Emails on example.com, IPs from the documentation ranges, phones in 555-01xx: nothing reaches a real person or host."
    >
      {#snippet badges()}
        {#if job.result}<Badge>seed {job.result.seed}</Badge>{/if}
      {/snippet}
    </TextOutput>
  {/snippet}
</ToolLayout>

<style>
  .opts {
    padding: var(--s-3) var(--s-4);
    display: flex;
    flex-direction: column;
    gap: var(--s-3);
  }
  .presets,
  .row {
    display: flex;
    flex-wrap: wrap;
    align-items: center;
    gap: var(--s-2) var(--s-3);
  }
  .label {
    font-size: var(--fs-sm);
    color: var(--fg-1);
  }
  .preset {
    padding: 2px var(--s-3);
    font-size: var(--fs-sm);
    color: var(--fg-1);
    background: var(--bg-3);
    border: 1px solid var(--border);
    border-radius: var(--r-full);
    text-transform: capitalize;
  }
  .preset:hover {
    color: var(--fg-0);
    border-color: var(--border-strong);
  }
  .cols {
    display: flex;
    flex-direction: column;
    gap: var(--s-1);
    max-height: 300px;
    overflow: auto;
  }
  .col {
    display: flex;
    align-items: center;
    gap: var(--s-2);
    padding: 2px 0;
  }
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
  .text:focus {
    outline: none;
    border-color: var(--accent);
  }
  .name {
    width: 150px;
  }
  .small {
    width: 110px;
  }
  .tiny {
    width: 56px;
  }
  .wide-num {
    width: 80px;
  }
  .wide {
    width: 260px;
  }
  .about,
  .dim {
    font-size: var(--fs-sm);
    color: var(--fg-2);
  }
  .grow {
    flex: 1;
  }
  .blank {
    display: flex;
    align-items: center;
    gap: var(--s-1);
    font-size: var(--fs-sm);
    color: var(--fg-2);
  }
</style>
