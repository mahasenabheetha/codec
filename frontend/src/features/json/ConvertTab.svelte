<script lang="ts">
  import ArrowRightLeft from '@lucide/svelte/icons/arrow-right-left'
  import type CodeView from '../../lib/components/CodeView.svelte'
  import SegmentedControl from '../../lib/components/SegmentedControl.svelte'
  import { fromYAML, toYAML } from '../../lib/api/json'
  import type { ToolDef } from '../../lib/tools'
  import { copyText } from '../../lib/utils/clipboard'
  import TextOutput from '../encode/TextOutput.svelte'
  import InputPane from '../shared/InputPane.svelte'
  import ToolLayout from '../shared/ToolLayout.svelte'
  import ToolTabs from '../shared/ToolTabs.svelte'
  import { Job, TextJob } from '../shared/job.svelte'
  import { toolCommands, toolShortcuts, toolStatus } from '../shared/tool.svelte'
  import { jsonDoc, jsonPrefs, liveJob, sampleDoc } from './doc.svelte'

  // JSON ⇄ YAML, keeping key order. JSON → YAML reads the shared
  // document; YAML → JSON has its own input. Swap moves the result over
  // and turns the direction round.
  let { tool, active }: { tool: ToolDef; active: boolean } = $props()

  type Dir = 'to-yaml' | 'to-json'
  let dir = $state<Dir>('to-yaml')
  const yamlDoc = new TextJob<never>() // only its input is used
  const job = new Job<{ output: string }>()
  let editor = $state<CodeView>()
  const src = $derived(dir === 'to-yaml' ? jsonDoc : yamlDoc)

  const call = () => {
    const input = src.input
    if (!input.trim()) return null
    const indent = jsonPrefs.value.indent
    return dir === 'to-yaml' ? () => toYAML(input) : () => fromYAML(input, { indent })
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
    if (dir === 'to-yaml') {
      yamlDoc.input = out
      dir = 'to-json'
    } else {
      jsonDoc.input = out
      dir = 'to-yaml'
    }
    editor?.focus()
  }
  const jump = (line: number, column: number) => editor?.selectPosition(line, column)

  toolShortcuts(() => active, { run, copy, clear, swap })
  toolCommands(() => active, () => [
    { id: 'json.toYaml', title: 'JSON: Convert to YAML', group: 'JSON & YAML', run: () => (dir = 'to-yaml') },
    { id: 'json.fromYaml', title: 'JSON: Convert YAML to JSON', group: 'JSON & YAML', run: () => (dir = 'to-json') },
  ])
  toolStatus(() => active, () => {
    if (job.error) return { text: job.error.line ? `Not ${dir === 'to-yaml' ? 'JSON' : 'YAML'} · line ${job.error.line}, col ${job.error.column}` : job.error.message, tone: 'err' }
    if (job.result) return { text: dir === 'to-yaml' ? 'Converted to YAML' : 'Converted to JSON', tone: 'ok' }
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
        { value: 'to-yaml', label: 'JSON → YAML' },
        { value: 'to-json', label: 'YAML → JSON' },
      ]}
    />
  {/snippet}
  {#snippet input()}
    <InputPane
      session={src}
      bind:editor
      onclear={clear}
      language={dir === 'to-yaml' ? 'auto' : 'yaml'}
      placeholder={dir === 'to-yaml' ? 'Paste JSON, JSONC or JSON5' : 'Paste YAML: anchors are resolved; several documents become a JSON array'}
    />
  {/snippet}
  {#snippet output()}
    <TextOutput
      output={job.result?.output ?? null}
      error={job.error}
      busy={job.busy}
      language={dir === 'to-yaml' ? 'yaml' : 'json'}
      emptyIcon={ArrowRightLeft}
      emptyTitle={dir === 'to-yaml' ? 'YAML appears here' : 'JSON appears here'}
      emptyDescription="Key order is kept. Swap sends the result back the other way."
      onsample={() => (dir === 'to-yaml' ? (jsonDoc.input = sampleDoc) : (yamlDoc.input = 'name: api\nreplicas: 3\nports:\n  - 80\n  - 443\n'))}
      onswap={swap}
      onjump={jump}
    />
  {/snippet}
</ToolLayout>
