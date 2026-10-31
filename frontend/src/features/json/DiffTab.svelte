<script lang="ts">
  import ArrowLeftRight from '@lucide/svelte/icons/arrow-left-right'
  import CircleCheck from '@lucide/svelte/icons/circle-check'
  import Copy from '@lucide/svelte/icons/copy'
  import GitCompare from '@lucide/svelte/icons/git-compare'
  import Button from '../../lib/components/Button.svelte'
  import EmptyState from '../../lib/components/EmptyState.svelte'
  import IconButton from '../../lib/components/IconButton.svelte'
  import { diffJSON, type Change } from '../../lib/api/json'
  import type { ToolDef } from '../../lib/tools'
  import { copyText } from '../../lib/utils/clipboard'
  import ErrorBanner from '../shared/ErrorBanner.svelte'
  import ToolLayout from '../shared/ToolLayout.svelte'
  import ToolTabs from '../shared/ToolTabs.svelte'
  import { Job } from '../shared/job.svelte'
  import { toolStatus } from '../shared/tool.svelte'
  import DiffSide from '../text/DiffSide.svelte'
  import { liveJob } from './doc.svelte'

  // Two JSON documents compared by path: what was added, removed,
  // changed, or changed type. Key order and formatting don't count;
  // numbers are compared by value.
  let { tool, active }: { tool: ToolDef; active: boolean } = $props()

  let left = $state('')
  let right = $state('')
  let leftName = $state('')
  let rightName = $state('')
  const job = new Job<{ changes: Change[] }>()

  liveJob(() => active, job, () => {
    const a = left
    const b = right
    if (!a.trim() || !b.trim()) return null
    return () => diffJSON(a, b)
  })

  function swap() {
    ;[left, right] = [right, left]
    ;[leftName, rightName] = [rightName, leftName]
  }
  function sample() {
    left = '{\n  "name": "api",\n  "replicas": 2,\n  "ports": [80, 443],\n  "env": { "LOG": "info" }\n}'
    right = '{\n  "name": "api",\n  "replicas": 3,\n  "ports": [80],\n  "env": { "LOG": "debug", "TRACE": true },\n  "port": "8080"\n}'
  }

  const changes = $derived(job.result?.changes ?? [])
  const counts = $derived.by(() => {
    const n = { added: 0, removed: 0, changed: 0, type: 0 }
    for (const c of changes) n[c.kind]++
    return n
  })
  const symbol: Record<Change['kind'], string> = { added: '+', removed: '−', changed: '~', type: '±' }
  const asText = () => changes.map((c) => `${symbol[c.kind]} ${c.path}: ${c.kind === 'added' ? c.new : c.kind === 'removed' ? c.old : `${c.old} → ${c.new}`}`).join('\n')

  toolStatus(() => active, () => {
    if (job.error) return { text: job.error.message, tone: 'err' }
    if (job.result) return { text: changes.length ? `${changes.length} ${changes.length === 1 ? 'difference' : 'differences'}` : 'The same', tone: 'ok' }
    return { text: 'Paste a document on each side' }
  })
</script>

<ToolLayout {tool}>
  {#snippet controls()}
    <ToolTabs {tool} />
  {/snippet}
  {#snippet actions()}
    <IconButton icon={ArrowLeftRight} tone="swap" label="Swap the sides" disabled={!left && !right} onclick={swap} />
  {/snippet}
  {#snippet body()}
    <div class="frame">
      <div class="sides">
        <DiffSide title="Old" bind:value={left} bind:name={leftName} placeholder="Paste JSON, JSONC or JSON5, or drop a file" />
        <DiffSide title="New" bind:value={right} bind:name={rightName} placeholder="…and the document to compare it with" />
      </div>
      <section class="result" aria-label="Differences">
        {#if job.error}
          <ErrorBanner error={job.error} />
        {:else if job.result && !changes.length}
          <p class="same"><CircleCheck size={16} strokeWidth={1.75} /> No differences: the same data, whatever the key order and formatting.</p>
        {:else if job.result}
          <header>
            <span class="add">{counts.added} added</span> · <span class="del">{counts.removed} removed</span> · <span class="chg">{counts.changed} changed</span>{#if counts.type}
              · <span class="typ">{counts.type} type changed</span>{/if}
            <span class="grow"></span>
            <Button size="sm" variant="ghost" icon={Copy} tone="copy" onclick={() => copyText(asText(), 'Differences')}>Copy</Button>
          </header>
          <ul>
            {#each changes as c (c.path + c.kind)}
              <li class={c.kind}>
                <span class="sym" aria-label={c.kind}>{symbol[c.kind]}</span>
                <button type="button" class="path" title="Copy the path" onclick={() => copyText(c.path, 'Path')}>{c.path}</button>
                <span class="vals">
                  {#if c.kind === 'added'}<span class="new">{c.new}</span>
                  {:else if c.kind === 'removed'}<span class="old">{c.old}</span>
                  {:else}<span class="old">{c.old}</span> → <span class="new">{c.new}</span>{/if}
                </span>
              </li>
            {/each}
          </ul>
        {:else}
          <EmptyState icon={GitCompare} title="The differences appear here" description="Each change by its path: added, removed, changed, or a type that changed. Key order and formatting don't count; 1.0 and 1 are the same.">
            <Button size="sm" onclick={sample}>Try an example</Button>
          </EmptyState>
        {/if}
      </section>
    </div>
  {/snippet}
</ToolLayout>

<style>
  .frame {
    flex: 1;
    min-height: 0;
    display: flex;
    flex-direction: column;
    gap: var(--s-3);
  }
  .sides {
    flex: 0 0 40%;
    min-height: 140px;
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: var(--s-3);
  }
  .result {
    flex: 1;
    min-height: 0;
    display: flex;
    flex-direction: column;
    background: var(--bg-2);
    border: 1px solid var(--border);
    border-radius: var(--r-lg);
    overflow: hidden;
  }
  header {
    display: flex;
    align-items: center;
    gap: var(--s-1);
    padding: var(--s-2) var(--s-3);
    font-size: var(--fs-sm);
    border-bottom: 1px solid var(--border);
    background: var(--bg-1);
  }
  .grow {
    flex: 1;
  }
  .add {
    color: var(--ok);
  }
  .del {
    color: var(--err);
  }
  .chg,
  .typ {
    color: var(--warn);
  }
  ul {
    flex: 1;
    overflow: auto;
    margin: 0;
    padding: var(--s-2);
    list-style: none;
  }
  li {
    display: grid;
    grid-template-columns: 16px minmax(120px, max-content) 1fr;
    align-items: baseline;
    gap: var(--s-2);
    padding: 3px var(--s-2);
    font-family: var(--font-mono);
    font-size: var(--fs-sm);
    border-radius: var(--r-sm);
  }
  li:hover {
    background: var(--bg-3);
  }
  .sym {
    font-weight: var(--fw-semibold);
  }
  li.added .sym {
    color: var(--ok);
  }
  li.removed .sym {
    color: var(--err);
  }
  li.changed .sym,
  li.type .sym {
    color: var(--warn);
  }
  .path {
    padding: 0;
    font: inherit;
    text-align: left;
    color: var(--syn-key);
    background: none;
    border: none;
  }
  .path:hover {
    text-decoration: underline;
  }
  .vals {
    min-width: 0;
    overflow-wrap: anywhere;
    color: var(--fg-1);
  }
  .old {
    color: var(--err);
  }
  .new {
    color: var(--ok);
  }
  .same {
    display: flex;
    align-items: center;
    gap: var(--s-2);
    padding: var(--s-4);
    color: var(--ok);
  }
</style>
