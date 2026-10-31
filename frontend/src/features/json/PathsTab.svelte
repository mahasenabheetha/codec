<script lang="ts">
  import Route from '@lucide/svelte/icons/route'
  import type CodeView from '../../lib/components/CodeView.svelte'
  import Badge from '../../lib/components/Badge.svelte'
  import SegmentedControl from '../../lib/components/SegmentedControl.svelte'
  import { flattenJSON, unflattenJSON } from '../../lib/api/json'
  import type { ToolDef } from '../../lib/tools'
  import { copyText } from '../../lib/utils/clipboard'
  import TextOutput from '../encode/TextOutput.svelte'
  import InputPane from '../shared/InputPane.svelte'
  import ToolLayout from '../shared/ToolLayout.svelte'
  import ToolTabs from '../shared/ToolTabs.svelte'
  import { Job } from '../shared/job.svelte'
  import { toolShortcuts, toolStatus } from '../shared/tool.svelte'
  import { useJsonDoc, jsonPrefs, liveJob, sampleDoc } from './doc.svelte'

  // Every value as a path and value (.spec.replicas = 3), and back.
  let { tool, active }: { tool: ToolDef; active: boolean } = $props()
  const jsonDoc = useJsonDoc()

  type Dir = 'flatten' | 'unflatten'
  let dir = $state<Dir>('flatten')
  const lines = jsonDoc.lines
  const job = new Job<{ output: string; count?: number }>()
  let editor = $state<CodeView>()
  const src = $derived(dir === 'flatten' ? jsonDoc : lines)

  const call = () => {
    const input = src.input
    if (!input.trim()) return null
    const indent = jsonPrefs.value.indent
    return dir === 'flatten'
      ? async () => {
          const r = await flattenJSON(input)
          return { output: r.output, count: r.leaves.length }
        }
      : () => unflattenJSON(input, { indent })
  }
  liveJob(() => active, job, call)

  const run = () => job.run(call())
  const copy = () => copyText(job.result?.output ?? '')
  function clear() {
    src.clear()
    job.reset()
    editor?.focus()
  }
  function swap() {
    const out = job.result?.output
    if (!out) return
    if (dir === 'flatten') {
      lines.input = out
      dir = 'unflatten'
    } else {
      jsonDoc.input = out
      dir = 'flatten'
    }
  }
  const jump = (line: number, column: number) => editor?.selectPosition(line, column)

  toolShortcuts(() => active, { run, copy, clear, swap })
  toolStatus(() => active, () => {
    if (job.error) return { text: job.error.message, tone: 'err' }
    if (job.result) return { text: dir === 'flatten' ? `${job.result.count?.toLocaleString()} paths` : 'Rebuilt as JSON', tone: 'ok' }
    return { text: 'Ready' }
  })
</script>

<ToolLayout {tool}>
  {#snippet controls()}
    <ToolTabs {tool} />
    <SegmentedControl
      label="Direction"
      bind:value={dir}
      options={[
        { value: 'flatten', label: 'JSON → paths' },
        { value: 'unflatten', label: 'Paths → JSON' },
      ]}
    />
  {/snippet}
  {#snippet input()}
    <InputPane
      session={src}
      bind:editor
      onclear={clear}
      language={dir === 'flatten' ? 'auto' : 'text'}
      placeholder={dir === 'flatten' ? 'Paste JSON, JSONC or JSON5' : 'One path = value per line, e.g. .spec.replicas = 3 or .labels["app.kubernetes.io/name"] = "api"'}
    />
  {/snippet}
  {#snippet output()}
    <TextOutput
      output={job.result?.output ?? null}
      error={job.error}
      busy={job.busy}
      language={dir === 'flatten' ? 'text' : 'json'}
      emptyIcon={Route}
      emptyTitle={dir === 'flatten' ? 'Paths appear here' : 'JSON appears here'}
      emptyDescription="Each value with its jq-style path; empty objects and arrays are kept, so the way back loses nothing."
      onsample={() => (dir === 'flatten' ? (jsonDoc.input = sampleDoc) : (lines.input = '.name = "api"\n.ports[0] = 80\n.ports[1] = 443\n.labels["app.kubernetes.io/name"] = "api"'))}
      onswap={swap}
      onjump={jump}
    >
      {#snippet badges()}
        {#if dir === 'flatten' && job.result?.count !== undefined}<Badge>{job.result.count.toLocaleString()} paths</Badge>{/if}
      {/snippet}
    </TextOutput>
  {/snippet}
</ToolLayout>
