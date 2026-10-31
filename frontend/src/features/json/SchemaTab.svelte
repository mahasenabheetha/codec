<script lang="ts">
  import FileJson from '@lucide/svelte/icons/file-braces'
  import type CodeView from '../../lib/components/CodeView.svelte'
  import Badge from '../../lib/components/Badge.svelte'
  import { inferSchema } from '../../lib/api/json'
  import type { ToolDef } from '../../lib/tools'
  import { copyText } from '../../lib/utils/clipboard'
  import TextOutput from '../encode/TextOutput.svelte'
  import InputPane from '../shared/InputPane.svelte'
  import ToolLayout from '../shared/ToolLayout.svelte'
  import ToolTabs from '../shared/ToolTabs.svelte'
  import { Job } from '../shared/job.svelte'
  import { toolShortcuts, toolStatus } from '../shared/tool.svelte'
  import { useJsonDoc, jsonPrefs, liveJob, sampleDoc } from './doc.svelte'

  // A JSON Schema every sample fits: one document, several pasted one
  // after another, or JSON Lines.
  let { tool, active }: { tool: ToolDef; active: boolean } = $props()
  const jsonDoc = useJsonDoc()

  const job = new Job<{ output: string; samples: number }>()
  let editor = $state<CodeView>()
  const call = () => {
    const input = jsonDoc.input
    if (!input.trim()) return null
    const indent = jsonPrefs.value.indent
    return () => inferSchema(input, { indent })
  }
  liveJob(() => active, job, call)

  const run = () => job.run(call())
  const copy = () => copyText(job.result?.output ?? '')
  const clear = () => (jsonDoc.clear(), job.reset(), editor?.focus())
  const jump = (line: number, column: number) => editor?.selectPosition(line, column)

  toolShortcuts(() => active, { run, copy, clear })
  toolStatus(() => active, () => {
    if (job.error) return { text: job.error.message, tone: 'err' }
    if (job.result) return { text: `Schema from ${job.result.samples} ${job.result.samples === 1 ? 'sample' : 'samples'}`, tone: 'ok' }
    return { text: 'Ready' }
  })
</script>

<ToolLayout {tool}>
  {#snippet controls()}
    <ToolTabs {tool} />
  {/snippet}
  {#snippet input()}
    <InputPane session={jsonDoc} bind:editor onclear={clear} language="auto" placeholder="Paste one or more JSON samples (JSON Lines works too): the schema fits them all" />
  {/snippet}
  {#snippet output()}
    <TextOutput
      output={job.result?.output ?? null}
      error={job.error}
      busy={job.busy}
      language="json"
      emptyIcon={FileJson}
      emptyTitle="The schema appears here"
      emptyDescription="JSON Schema 2020-12: the types at each place, properties in the order first seen, required when every sample has them, and formats such as date-time, email and uuid."
      onsample={() => (jsonDoc.input = sampleDoc)}
      onjump={jump}
    >
      {#snippet badges()}
        {#if job.result}<Badge>{job.result.samples} {job.result.samples === 1 ? 'sample' : 'samples'}</Badge>{/if}
      {/snippet}
    </TextOutput>
  {/snippet}
</ToolLayout>
