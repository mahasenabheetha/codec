<script lang="ts">
  import { untrack } from 'svelte'
  import Copy from '@lucide/svelte/icons/copy'
  import Waypoints from '@lucide/svelte/icons/waypoints'
  import Badge from '../../lib/components/Badge.svelte'
  import Button from '../../lib/components/Button.svelte'
  import EmptyState from '../../lib/components/EmptyState.svelte'
  import IconButton from '../../lib/components/IconButton.svelte'
  import SegmentedControl from '../../lib/components/SegmentedControl.svelte'
  import { cidrContains, cidrDetails, cidrSplit, groupDigits, overlaps, type Listed, type Overlap, type Range, type Subnets } from '../../lib/api/net'
  import type { ToolDef } from '../../lib/tools'
  import { copyText } from '../../lib/utils/clipboard'
  import ErrorBanner from '../shared/ErrorBanner.svelte'
  import LineInput from '../shared/LineInput.svelte'
  import Pane from '../shared/Pane.svelte'
  import ToolLayout from '../shared/ToolLayout.svelte'
  import ToolTabs from '../shared/ToolTabs.svelte'
  import { Job, TextJob } from '../shared/job.svelte'
  import { toolCommands, toolShortcuts, toolStatus } from '../shared/tool.svelte'
  import Rows from './Rows.svelte'

  let { tool, active }: { tool: ToolDef; active: boolean } = $props()

  let mode = $state<'range' | 'list'>('range')
  let field = $state<HTMLInputElement>()

  // One range: details, then subnets and an address check on demand.
  const job = new TextJob<{ range: Range }>()
  const split = new Job<Subnets>()
  const check = new Job<{ contains: boolean }>()
  let splitTo = $state<number | null>(null)
  let address = $state('')

  $effect(() => {
    const input = job.input.trim()
    untrack(() => job.schedule(input ? () => cidrDetails(input) : null))
  })
  const range = $derived(job.result?.range)
  const maxBits = $derived(range?.version === 6 ? 128 : 32)
  // A new range starts the splitter two bits down (four subnets).
  $effect(() => {
    const r = range
    untrack(() => {
      if (r && (splitTo === null || splitTo < r.prefix || splitTo > maxBits)) splitTo = Math.min(r.prefix + 2, maxBits)
    })
  })
  $effect(() => {
    const r = range
    const to = splitTo
    untrack(() => split.schedule(r && to !== null && to >= r.prefix && to <= maxBits ? () => cidrSplit(r.cidr, to) : null, 120))
  })
  $effect(() => {
    const r = range
    const a = address.trim()
    untrack(() => check.schedule(r && a ? () => cidrContains(r.cidr, a) : null))
  })

  const kindTone = (k: string) => (k === 'public' ? 'neutral' : k === 'mixed' ? 'warn' : 'accent')
  const rows = $derived(
    range
      ? [
          { label: 'Range', value: range.cidr },
          { label: 'Address given', value: range.host },
          { label: 'Network', value: range.network },
          { label: 'Broadcast', value: range.broadcast },
          { label: 'First host', value: range.first },
          { label: 'Last host', value: range.last },
          { label: 'Netmask', value: range.mask },
          { label: 'Wildcard', value: range.wildcard },
          { label: 'Addresses', value: groupDigits(range.addresses), copy: range.addresses },
          { label: 'Usable hosts', value: groupDigits(range.hosts), copy: range.hosts },
        ]
      : [],
  )

  // A list: which ranges overlap which.
  let listText = $state('')
  const list = new Job<{ ranges: Listed[]; overlaps: Overlap[] }>()
  $effect(() => {
    const lines = listText.split('\n')
    const any = lines.some((l) => l.replace(/#.*/, '').trim())
    untrack(() => list.schedule(any ? () => overlaps(lines) : null, 200))
  })
  const ranges = $derived(list.result?.ranges ?? [])
  const pairs = $derived(list.result?.overlaps ?? [])
  const clashes = $derived.by(() => {
    const m = new Map<number, string[]>()
    const note = (i: number, s: string) => m.set(i, [...(m.get(i) ?? []), s])
    for (const p of pairs) {
      const a = ranges[p.a].cidr!
      const b = ranges[p.b].cidr!
      // Numbers are the list's own (blank and comment lines aren't counted).
      if (p.relation === 'same') {
        note(p.a, `same as #${p.b + 1}`)
        note(p.b, `same as #${p.a + 1}`)
      } else {
        note(p.a, `contains #${p.b + 1} ${b}`)
        note(p.b, `inside #${p.a + 1} ${a}`)
      }
    }
    return m
  })

  function sample() {
    if (mode === 'range') job.input = '10.20.0.0/16'
    else listText = '10.0.0.0/16      # hub VNet\n10.1.0.0/16      # spoke: shop\n10.1.8.0/21      # spoke: shop AKS nodes\n10.2.0.0/16      # spoke: data\n172.16.0.0/16    # AKS pod CIDR\n10.0.0.0/16      # spoke: legacy\n'
  }
  function clear() {
    if (mode === 'range') {
      job.clear()
      address = ''
      field?.focus()
    } else {
      listText = ''
      list.reset()
    }
  }
  const copy = () => (mode === 'range' ? range && copyText(range.cidr, 'Range') : split.result && copyText(split.result.subnets.join('\n'), 'Subnets'))

  function run() {
    const input = job.input.trim()
    if (mode === 'range') job.run(input ? () => cidrDetails(input) : null)
    else {
      const lines = listText.split('\n')
      list.run(() => overlaps(lines))
    }
  }

  toolShortcuts(() => active, { run, copy, clear })
  toolCommands(() => active, () => [
    { id: 'cidr.range', title: 'CIDR: One range in detail', group: 'Network', run: () => (mode = 'range') },
    { id: 'cidr.list', title: 'CIDR: Check a list for overlaps', group: 'Network', run: () => (mode = 'list') },
  ])
  toolStatus(() => active, () => {
    if (mode === 'list') {
      if (list.error) return { text: list.error.message, tone: 'err' }
      if (!list.result) return { text: 'Ready' }
      const bad = ranges.filter((r) => r.error).length
      const text = `${ranges.length} ranges · ${pairs.length ? `${pairs.length} overlap${pairs.length === 1 ? '' : 's'}` : 'no overlaps'}${bad ? ` · ${bad} skipped` : ''}`
      return { text, tone: pairs.length ? 'warn' : 'ok' }
    }
    if (job.error) return { text: 'Not a CIDR range', tone: 'err' }
    if (range) return { text: `${range.cidr} · IPv${range.version} · ${groupDigits(range.hosts)} hosts · ${range.kind}`, tone: 'ok' }
    return { text: 'Ready' }
  })
</script>

<ToolLayout {tool}>
  {#snippet controls()}
    <ToolTabs {tool} />
    <span class="divider" aria-hidden="true"></span>
    <SegmentedControl
      label="CIDR mode"
      bind:value={mode}
      options={[
        { value: 'range', label: 'Range', title: 'One range in detail, split into subnets' },
        { value: 'list', label: 'Overlaps', title: 'A list of ranges checked for overlaps' },
      ]}
    />
  {/snippet}
  {#snippet body()}
    {#if mode === 'range'}
      <LineInput bind:value={job.input} bind:field label="CIDR range" invalid={!!job.error} onclear={clear} placeholder="10.0.0.0/16, 2001:db8::/48, 10.0.0.0 255.255.0.0 or one address">
        <p class="hint">IPv4 or IPv6, as address/length, an address and netmask, or one address. An address inside a range (10.0.0.5/24) shows the range it belongs to.</p>
      </LineInput>
      {#if job.error}
        <ErrorBanner error={job.error} />
      {/if}
      {#if range}
        <Pane title="Range" grow={false}>
          {#snippet meta()}
            <Badge tone={kindTone(range.kind)}>{range.kind}</Badge>
            <Badge>IPv{range.version}</Badge>
          {/snippet}
          <Rows {rows} />
        </Pane>
        <div class="pair">
          <Pane title="Subnets" grow={false}>
            {#snippet meta()}
              {#if split.result}<span class="meta">{groupDigits(split.result.count)} × /{splitTo}, {groupDigits(split.result.each.hosts)} hosts each</span>{/if}
            {/snippet}
            {#snippet actions()}
              <IconButton icon={Copy} label="Copy the subnets" size="sm" disabled={!split.result} onclick={() => split.result && copyText(split.result.subnets.join('\n'), 'Subnets')} />
            {/snippet}
            <div class="split">
              <label>
                Split into /
                <input class="num" type="number" min={range.prefix} max={maxBits} bind:value={splitTo} aria-label="Subnet prefix length" />
              </label>
              {#if split.error}<ErrorBanner error={split.error} />{/if}
              {#if split.result}
                <ol class="subnets">
                  {#each split.result.subnets as s (s)}<li><code>{s}</code></li>{/each}
                </ol>
                {#if split.result.truncated}<p class="hint">The first {split.result.subnets.length} of {groupDigits(split.result.count)}.</p>{/if}
              {/if}
            </div>
          </Pane>
          <Pane title="Is an address inside?" grow={false}>
            <div class="split">
              <input class="text" bind:value={address} placeholder="e.g. {range.first}" aria-label="Address to check" spellcheck="false" autocomplete="off" />
              {#if check.error}
                <ErrorBanner error={check.error} />
              {:else if check.result}
                <p class="answer" class:yes={check.result.contains}>
                  {address.trim()} is {check.result.contains ? '' : 'not '}in {range.cidr}
                </p>
              {/if}
            </div>
          </Pane>
        </div>
      {:else if !job.error}
        <Pane title="Range" grow={false}>
          <EmptyState icon={Waypoints} title="The range in detail appears here" description="Network, broadcast, first and last host, netmask, wildcard and how many hosts it holds; then split it into subnets.">
            <Button size="sm" onclick={sample}>Try an example</Button>
          </EmptyState>
        </Pane>
      {/if}
    {:else}
      <Pane title="Ranges" grow={false}>
        {#snippet meta()}<span class="meta">one per line · # starts a comment</span>{/snippet}
        <textarea bind:value={listText} rows="8" placeholder={'10.0.0.0/16   # hub\n10.1.0.0/16   # spoke\n172.16.0.0/16 # pod CIDR'} aria-label="Ranges, one per line" spellcheck="false"></textarea>
      </Pane>
      {#if list.error}
        <ErrorBanner error={list.error} />
      {/if}
      {#if ranges.length}
        <Pane title="Result" grow={false}>
          {#snippet meta()}
            {#if pairs.length}<Badge tone="warn">{pairs.length} overlap{pairs.length === 1 ? '' : 's'}</Badge>{:else}<Badge tone="ok">no overlaps</Badge>{/if}
          {/snippet}
          <ol class="list">
            {#each ranges as r, i (i)}
              {@const notes = clashes.get(i)}
              <li class:bad={!!r.error} class:clash={!!notes}>
                <span class="no">{i + 1}</span>
                <span class="what"><code>{r.cidr ?? r.input}</code>{#if r.note}<span class="name">{r.note}</span>{/if}</span>
                {#if r.error}
                  <span class="note err">{r.error}</span>
                {:else if notes}
                  <span class="note warn">{notes.join('; ')}</span>
                {:else}
                  <span class="note ok">no overlap</span>
                {/if}
              </li>
            {/each}
          </ol>
        </Pane>
      {:else if !list.error}
        <Pane title="Result" grow={false}>
          <EmptyState icon={Waypoints} title="Overlaps appear here" description="Paste your VNets, VPCs, subnets, pod and service ranges; ranges that share addresses are marked.">
            <Button size="sm" onclick={sample}>Try an example</Button>
          </EmptyState>
        </Pane>
      {/if}
    {/if}
  {/snippet}
</ToolLayout>

<style>
  .divider {
    width: 1px;
    height: 20px;
    background: var(--border);
  }
  .hint,
  .meta {
    font-size: var(--fs-sm);
    color: var(--fg-2);
  }
  .pair {
    display: grid;
    grid-template-columns: repeat(auto-fit, minmax(300px, 1fr));
    gap: var(--s-3);
    align-items: start;
  }
  .split {
    display: flex;
    flex-direction: column;
    gap: var(--s-2);
    padding: var(--s-3);
  }
  label {
    display: flex;
    align-items: center;
    gap: var(--s-1);
    font-size: var(--fs-sm);
    color: var(--fg-1);
  }
  .num,
  .text,
  textarea {
    font-family: var(--font-mono);
    font-size: var(--fs-md);
    color: var(--fg-0);
    background: var(--bg-0);
    border: 1px solid var(--border-strong);
    border-radius: var(--r-sm);
  }
  .num {
    width: 72px;
    height: var(--control-h);
    padding: 0 var(--s-2);
  }
  .text {
    height: var(--control-h);
    padding: 0 var(--s-2);
  }
  textarea {
    display: block;
    width: calc(100% - 2 * var(--s-3));
    margin: var(--s-3);
    padding: var(--s-2);
    resize: vertical;
    line-height: var(--lh-code);
  }
  .num:focus,
  .text:focus,
  textarea:focus {
    outline: none;
    border-color: var(--accent);
  }
  .subnets {
    max-height: 260px;
    overflow: auto;
    margin: 0;
    padding: 0 0 0 3.2em;
    font-size: var(--fs-md);
    color: var(--fg-2);
  }
  .subnets code {
    color: var(--fg-0);
    user-select: all;
  }
  .answer {
    margin: 0;
    font-size: var(--fs-md);
    color: var(--warn);
  }
  .answer.yes {
    color: var(--ok);
  }
  .list {
    margin: 0;
    padding: var(--s-2) var(--s-3);
    list-style: none;
  }
  .list li {
    display: grid;
    grid-template-columns: 2.5em minmax(160px, auto) 1fr;
    align-items: baseline;
    gap: var(--s-3);
    padding: var(--s-1) 0;
    border-bottom: 1px solid var(--border);
  }
  .list li:last-child {
    border-bottom: none;
  }
  .no {
    font-size: var(--fs-sm);
    color: var(--fg-2);
    text-align: right;
    font-variant-numeric: tabular-nums;
  }
  .what {
    display: flex;
    flex-direction: column;
  }
  .list code {
    font-size: var(--fs-md);
    color: var(--fg-0);
  }
  .name {
    font-size: var(--fs-sm);
    color: var(--fg-2);
  }
  .note {
    font-size: var(--fs-sm);
  }
  .note.ok {
    color: var(--fg-2);
  }
  .note.warn {
    color: var(--warn);
  }
  .note.err {
    color: var(--err);
  }
  .list li.bad code {
    color: var(--fg-2);
    text-decoration: line-through;
  }
</style>
