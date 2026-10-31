<script lang="ts">
  import Braces from '@lucide/svelte/icons/braces'
  import CircleCheck from '@lucide/svelte/icons/circle-check'
  import TriangleAlert from '@lucide/svelte/icons/triangle-alert'
  import Play from '@lucide/svelte/icons/play'
  import type CodeView from '../../lib/components/CodeView.svelte'
  import Badge from '../../lib/components/Badge.svelte'
  import Button from '../../lib/components/Button.svelte'
  import SegmentedControl from '../../lib/components/SegmentedControl.svelte'
  import Select from '../../lib/components/Select.svelte'
  import Toggle from '../../lib/components/Toggle.svelte'
  import { formatJSON, type Dialect } from '../../lib/api/json'
  import { handoff } from '../../lib/stores/handoff.svelte'
  import type { ToolDef } from '../../lib/tools'
  import { copyText } from '../../lib/utils/clipboard'
  import { formatShortcut } from '../../lib/utils/platform'
  import TextOutput from '../encode/TextOutput.svelte'
  import ErrorBanner from '../shared/ErrorBanner.svelte'
  import InputPane from '../shared/InputPane.svelte'
  import Pane from '../shared/Pane.svelte'
  import ToolLayout from '../shared/ToolLayout.svelte'
  import ToolTabs from '../shared/ToolTabs.svelte'
  import { Job } from '../shared/job.svelte'
  import { sizeLabel } from '../shared/detect'
  import { toolCommands, toolShortcuts, toolStatus } from '../shared/tool.svelte'
  import { untrack } from 'svelte'
  import { jsonDoc, jsonPrefs, liveJob, sampleDoc } from './doc.svelte'

  let { tool, active }: { tool: ToolDef; active: boolean } = $props()

  type Mode = 'pretty' | 'min' | 'validate'
  let mode = $state<Mode>('pretty')
  const prefs = $derived(jsonPrefs.value)
  const job = new Job<{ output: string; dialect: Dialect }>()
  let editor = $state<CodeView>()

  const dialectName: Record<Dialect, string> = { json: 'JSON', jsonc: 'JSONC', json5: 'JSON5' }
  const call = () => {
    const input = jsonDoc.input
    if (!input.trim()) return null
    const indent = mode === 'pretty' ? prefs.indent : 'min'
    const sort = prefs.sort
    return () => formatJSON(input, { indent, sort })
  }
  liveJob(() => active, job, call)
  // JSONC or JSON5 handed over by smart paste.
  $effect(() => {
    const v = handoff.take('json', 'format')
    if (v !== null)
      untrack(() => {
        mode = 'pretty'
        jsonDoc.input = v
      })
  })

  const run = () => job.run(call())
  const copy = () => copyText(job.result?.output ?? '')
  function clear() {
    jsonDoc.clear()
    job.reset()
    editor?.focus()
  }
  function swap() {
    if (!job.result || mode === 'validate') return
    jsonDoc.input = job.result.output
    editor?.focus()
  }
  const jump = (line: number, column: number) => editor?.selectPosition(line, column)

  toolShortcuts(() => active, { run, copy, clear, swap })
  toolCommands(() => active, () => [
    { id: 'json.pretty', title: 'JSON: Pretty-print', group: 'JSON & YAML', run: () => (mode = 'pretty') },
    { id: 'json.min', title: 'JSON: Minify', group: 'JSON & YAML', run: () => (mode = 'min') },
    { id: 'json.validate', title: 'JSON: Validate', group: 'JSON & YAML', run: () => (mode = 'validate') },
    { id: 'json.sort', title: 'JSON: Toggle sorting keys', group: 'JSON & YAML', run: () => (prefs.sort = !prefs.sort) },
  ])
  toolStatus(() => active, () => {
    const e = job.error
    if (e) return { text: e.line ? `Not JSON · line ${e.line}, col ${e.column}` : 'Not JSON', tone: 'err' }
    const r = job.result
    if (r && mode === 'validate') return r.dialect === 'json' ? { text: 'Valid JSON', tone: 'ok' } : { text: `Valid ${dialectName[r.dialect]}, not strict JSON`, tone: 'warn' }
    if (r) return { text: `${mode === 'min' ? 'Minified' : 'Formatted'}${r.dialect !== 'json' ? ` · read as ${dialectName[r.dialect]}` : ''}`, tone: 'ok' }
    return { text: 'Ready' }
  })
</script>

<ToolLayout {tool}>
  {#snippet controls()}
    <ToolTabs {tool} />
    <SegmentedControl
      label="Action"
      bind:value={mode}
      options={[
        { value: 'pretty', label: 'Pretty' },
        { value: 'min', label: 'Minify' },
        { value: 'validate', label: 'Validate' },
      ]}
    />
    <Select
      label="Indentation"
      bind:value={prefs.indent}
      disabled={mode !== 'pretty'}
      items={[
        { value: '  ', label: '2 spaces' },
        { value: '    ', label: '4 spaces' },
        { value: '\t', label: 'Tabs' },
      ]}
    />
    <Toggle label="Sort keys" bind:checked={prefs.sort} disabled={mode === 'validate'} />
  {/snippet}
  {#snippet actions()}
    <Button variant="primary" icon={Play} tone="run" onclick={run} title={formatShortcut('Mod+Enter')}>
      {mode === 'validate' ? 'Validate' : mode === 'min' ? 'Minify' : 'Format'}
    </Button>
  {/snippet}
  {#snippet input()}
    <InputPane session={jsonDoc} bind:editor onclear={clear} language="auto" placeholder="Paste JSON, JSONC or JSON5: comments, trailing commas and unquoted keys are read, errors point to the exact line and column" onpaste={run} />
  {/snippet}
  {#snippet output()}
    {#if mode === 'validate'}
      <Pane title="Result">
        {#if job.error}
          <ErrorBanner error={job.error} onjump={jump} />
        {:else if job.result}
          {@const d = job.result.dialect}
          <div class="verdict">
            {#if d === 'json'}
              <CircleCheck size={30} strokeWidth={1.75} class="ok-icon" />
              <p class="headline">Valid JSON</p>
            {:else}
              <TriangleAlert size={30} strokeWidth={1.75} class="warn-icon" />
              <p class="headline">Valid {dialectName[d]}, not strict JSON</p>
              <p class="sub">{d === 'jsonc' ? 'It has comments or trailing commas.' : 'It uses JSON5: unquoted keys, single quotes, hex numbers or the like.'} Pretty writes it as JSON.</p>
            {/if}
            <p class="sub">{sizeLabel(jsonDoc.input)}</p>
          </div>
        {:else}
          <p class="empty">Paste JSON on the left to check it.</p>
        {/if}
      </Pane>
    {:else}
      <TextOutput
        output={job.result?.output ?? null}
        error={job.error}
        busy={job.busy}
        language="json"
        emptyIcon={Braces}
        emptyTitle={mode === 'min' ? 'Minified JSON appears here' : 'Formatted JSON appears here'}
        emptyDescription="Type or paste on the left; the output follows as you type. JSONC and JSON5 come out as JSON."
        onsample={() => (jsonDoc.input = sampleDoc)}
        onswap={swap}
        onjump={jump}
      >
        {#snippet badges()}
          {#if job.result && job.result.dialect !== 'json'}<Badge tone="accent">{dialectName[job.result.dialect]} → JSON</Badge>{/if}
        {/snippet}
      </TextOutput>
    {/if}
  {/snippet}
</ToolLayout>

<style>
  .verdict {
    height: 100%;
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: var(--s-2);
    padding: var(--s-6);
    text-align: center;
  }
  .verdict :global(.ok-icon) {
    color: var(--ok);
  }
  .verdict :global(.warn-icon) {
    color: var(--warn);
  }
  .headline {
    font-size: var(--fs-xl);
    font-weight: var(--fw-semibold);
  }
  .sub,
  .empty {
    max-width: 360px;
    font-size: var(--fs-sm);
    color: var(--fg-1);
  }
  .empty {
    padding: var(--s-6);
    color: var(--fg-2);
  }
</style>
