<script lang="ts">
  import ListTree from '@lucide/svelte/icons/list-tree'
  import ChevronsDownUp from '@lucide/svelte/icons/chevrons-down-up'
  import ChevronsUpDown from '@lucide/svelte/icons/chevrons-up-down'
  import type CodeView from '../../lib/components/CodeView.svelte'
  import EmptyState from '../../lib/components/EmptyState.svelte'
  import IconButton from '../../lib/components/IconButton.svelte'
  import SearchInput from '../../lib/components/SearchInput.svelte'
  import Button from '../../lib/components/Button.svelte'
  import { flattenJSON, formatJSON, jsonStats, type Leaf, type Stats } from '../../lib/api/json'
  import type { ToolDef } from '../../lib/tools'
  import { copyText } from '../../lib/utils/clipboard'
  import ErrorBanner from '../shared/ErrorBanner.svelte'
  import InputPane from '../shared/InputPane.svelte'
  import Pane from '../shared/Pane.svelte'
  import ToolLayout from '../shared/ToolLayout.svelte'
  import ToolTabs from '../shared/ToolTabs.svelte'
  import { Job } from '../shared/job.svelte'
  import { toolStatus } from '../shared/tool.svelte'
  import { jsonDoc, liveJob, sampleDoc } from './doc.svelte'
  import JsonTree from './JsonTree.svelte'

  // The document as a tree you can fold, with the path of any value a
  // click away; a search lists every match by path. Input too big to
  // draw as a tree gets its counts instead.
  let { tool, active }: { tool: ToolDef; active: boolean } = $props()

  const BIG = 2 << 20 // above this, counts instead of a tree
  interface TreeData {
    value?: unknown
    leaves?: Leaf[]
    stats?: Stats
  }
  const job = new Job<TreeData>()
  let editor = $state<CodeView>()
  let query = $state('')
  let foldAll = $state(0) // bumped to re-create the tree folded or opened
  let openAll = $state(false)

  liveJob(() => active, job, () => {
    const input = jsonDoc.input
    if (!input.trim()) return null
    if (input.length > BIG) return async () => ({ stats: await jsonStats(input) })
    return async () => {
      const [min, flat] = await Promise.all([formatJSON(input, { indent: 'min' }), flattenJSON(input)])
      return { value: JSON.parse(min.output), leaves: flat.leaves }
    }
  })

  const q = $derived(query.trim().toLowerCase())
  const matches = $derived(q && job.result?.leaves ? job.result.leaves.filter((l) => l.path.toLowerCase().includes(q) || l.value.toLowerCase().includes(q)) : [])
  const jump = (line: number, column: number) => editor?.selectPosition(line, column)

  toolStatus(() => active, () => {
    if (job.error) return { text: job.error.line ? `Not JSON · line ${job.error.line}, col ${job.error.column}` : 'Not JSON', tone: 'err' }
    if (job.result?.stats) return { text: 'Too big for a tree: counts shown', tone: 'warn' }
    if (job.result?.leaves) return { text: `${job.result.leaves.length.toLocaleString()} values${q ? ` · ${matches.length} ${matches.length === 1 ? 'match' : 'matches'}` : ''}`, tone: 'ok' }
    return { text: 'Ready' }
  })
</script>

<ToolLayout {tool}>
  {#snippet controls()}
    <ToolTabs {tool} />
  {/snippet}
  {#snippet input()}
    <InputPane session={jsonDoc} bind:editor onclear={() => (jsonDoc.clear(), job.reset())} language="auto" placeholder="Paste JSON, JSONC or JSON5 to see it as a tree" />
  {/snippet}
  {#snippet output()}
    <Pane title="Tree">
      {#snippet actions()}
        {#if job.result?.value !== undefined}
          <IconButton icon={ChevronsUpDown} label="Open everything" size="sm" onclick={() => ((openAll = true), foldAll++)} />
          <IconButton icon={ChevronsDownUp} label="Fold everything" size="sm" onclick={() => ((openAll = false), foldAll++)} />
        {/if}
      {/snippet}
      {#if job.error}
        <ErrorBanner error={job.error} onjump={jump} />
      {:else if job.result?.stats}
        {@const s = job.result.stats}
        <div class="stats">
          <p>This document is too big to draw as a tree ({(s.bytes / 1048576).toFixed(1)} MB). What it holds:</p>
          <dl>
            {#each [['Values', s.values], ['Objects', s.objects], ['Arrays', s.arrays], ['Keys', s.keys], ['Strings', s.strings], ['Numbers', s.numbers], ['Booleans', s.bools], ['Nulls', s.nulls], ['Deepest level', s.maxDepth]] as [k, v] (k)}
              <dt>{k}</dt>
              <dd>{Number(v).toLocaleString()}</dd>
            {/each}
          </dl>
          <p class="hint">Query, Paths and Format still work on it.</p>
        </div>
      {:else if job.result?.value !== undefined}
        <div class="search"><SearchInput bind:value={query} placeholder="Find a key, path or value" /></div>
        <div class="tree">
          {#if q}
            {#each matches.slice(0, 500) as m (m.path)}
              <button type="button" class="match" title="Copy the path" onclick={() => copyText(m.path, 'Path')}>
                <span class="mpath">{m.path}</span>
                <span class="mval">{m.value}</span>
              </button>
            {:else}
              <p class="hint pad">Nothing matches “{query}”.</p>
            {/each}
            {#if matches.length > 500}<p class="hint pad">Showing 500 of {matches.length.toLocaleString()} matches.</p>{/if}
          {:else}
            {#key foldAll}
              <JsonTree value={job.result.value} all={openAll} />
            {/key}
          {/if}
        </div>
      {:else}
        <EmptyState icon={ListTree} title="The tree appears here" description="Fold and open objects and arrays, copy any value's path (.spec.containers[0].image), and search keys and values.">
          <Button size="sm" onclick={() => (jsonDoc.input = sampleDoc)}>Try a sample</Button>
        </EmptyState>
      {/if}
    </Pane>
  {/snippet}
</ToolLayout>

<style>
  .search {
    padding: var(--s-2) var(--s-3);
    border-bottom: 1px solid var(--border);
  }
  .tree {
    flex: 1;
    min-height: 0;
    overflow: auto;
    padding: var(--s-2);
  }
  .match {
    display: flex;
    align-items: baseline;
    gap: var(--s-3);
    width: 100%;
    padding: 3px var(--s-2);
    font-family: var(--font-mono);
    font-size: var(--fs-sm);
    text-align: left;
    background: none;
    border: none;
    border-radius: var(--r-sm);
  }
  .match:hover {
    background: var(--bg-3);
  }
  .mpath {
    color: var(--syn-key);
  }
  .mval {
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    color: var(--fg-1);
  }
  .stats {
    padding: var(--s-4);
    font-size: var(--fs-md);
    color: var(--fg-1);
  }
  dl {
    display: grid;
    grid-template-columns: max-content max-content;
    gap: var(--s-1) var(--s-6);
    margin: var(--s-3) 0;
  }
  dd {
    margin: 0;
    font-family: var(--font-mono);
    color: var(--fg-0);
  }
  .hint {
    font-size: var(--fs-sm);
    color: var(--fg-2);
  }
  .pad {
    padding: var(--s-2);
  }
</style>
