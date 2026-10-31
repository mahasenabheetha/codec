<script lang="ts">
  import TextSearch from '@lucide/svelte/icons/text-search'
  import type CodeView from '../../lib/components/CodeView.svelte'
  import Badge from '../../lib/components/Badge.svelte'
  import Toggle from '../../lib/components/Toggle.svelte'
  import { queryJSON } from '../../lib/api/json'
  import type { ToolDef } from '../../lib/tools'
  import { copyText } from '../../lib/utils/clipboard'
  import TextOutput from '../encode/TextOutput.svelte'
  import InputPane from '../shared/InputPane.svelte'
  import LineInput from '../shared/LineInput.svelte'
  import ToolLayout from '../shared/ToolLayout.svelte'
  import ToolTabs from '../shared/ToolTabs.svelte'
  import { Job } from '../shared/job.svelte'
  import { toolShortcuts, toolStatus } from '../shared/tool.svelte'
  import { useJsonDoc, jsonPrefs, liveJob, sampleDoc } from './doc.svelte'

  // A jq filter over the document (gojq: no environment, 1000 results,
  // 5 seconds). Each result shows as JSON.
  let { tool, active }: { tool: ToolDef; active: boolean } = $props()
  const jsonDoc = useJsonDoc()

  let expr = $state('.')
  let compact = $state(false)
  const job = new Job<{ results: string[]; truncated?: boolean }>()
  let editor = $state<CodeView>()
  const examples = ['.metadata.name', '.spec.template.spec.containers[].image', '[.. | .image? // empty]', 'paths(scalars) | join(".")', 'keys']

  const call = () => {
    const input = jsonDoc.input
    const filter = expr.trim()
    if (!input.trim() || !filter) return null
    const indent = compact ? 'min' : jsonPrefs.value.indent
    return () => queryJSON(input, filter, { indent })
  }
  liveJob(() => active, job, call)

  const resultText = $derived(job.result ? job.result.results.join('\n') : null)
  const run = () => job.run(call())
  const copy = () => copyText(resultText ?? '')
  const clear = () => (jsonDoc.clear(), job.reset(), editor?.focus())
  const jump = (line: number, column: number) => editor?.selectPosition(line, column)

  toolShortcuts(() => active, { run, copy, clear })
  toolStatus(() => active, () => {
    if (job.error) return { text: job.error.message, tone: 'err' }
    const r = job.result
    if (r) return { text: `${r.results.length}${r.truncated ? '+' : ''} ${r.results.length === 1 ? 'result' : 'results'}`, tone: 'ok' }
    return { text: 'Ready' }
  })
</script>

<ToolLayout {tool}>
  {#snippet controls()}
    <ToolTabs {tool} />
    <Toggle label="One line each" bind:checked={compact} />
  {/snippet}
  {#snippet input()}
    <div class="col">
      <LineInput bind:value={expr} label="jq filter" placeholder={'.items[] | select(.kind == "Deployment") | .metadata.name'}>
        <div class="examples">
          {#each examples as ex (ex)}
            <button type="button" class="ex" onclick={() => (expr = ex)}>{ex}</button>
          {/each}
        </div>
      </LineInput>
      <InputPane session={jsonDoc} bind:editor onclear={clear} language="auto" placeholder="Paste JSON, JSONC or JSON5 to query" />
    </div>
  {/snippet}
  {#snippet output()}
    <TextOutput
      output={resultText}
      error={job.error}
      busy={job.busy}
      language="json"
      emptyIcon={TextSearch}
      emptyTitle="Results appear here"
      emptyDescription="jq filters, as in the jq tool: select, map, paths, keys, length… No environment, at most 1000 results, 5 seconds."
      onsample={() => ((jsonDoc.input = sampleDoc), (expr = '.spec.template.spec.containers[] | {name, image}'))}
      onjump={jump}
    >
      {#snippet badges()}
        {#if job.result}<Badge>{job.result.results.length}{job.result.truncated ? '+' : ''} {job.result.results.length === 1 ? 'result' : 'results'}</Badge>{/if}
      {/snippet}
    </TextOutput>
  {/snippet}
</ToolLayout>

<style>
  .col {
    height: 100%;
    display: flex;
    flex-direction: column;
    gap: var(--s-3);
    min-height: 0;
  }
  .examples {
    display: flex;
    flex-wrap: wrap;
    gap: var(--s-1);
  }
  .ex {
    padding: 1px var(--s-2);
    font-family: var(--font-mono);
    font-size: var(--fs-xs);
    color: var(--fg-1);
    background: var(--bg-3);
    border: 1px solid var(--border);
    border-radius: var(--r-full);
  }
  .ex:hover {
    color: var(--fg-0);
    border-color: var(--border-strong);
  }
</style>
