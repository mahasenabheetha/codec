<script lang="ts">
  import Braces from '@lucide/svelte/icons/braces'
  import CircleCheck from '@lucide/svelte/icons/circle-check'
  import Play from '@lucide/svelte/icons/play'
  import type CodeView from '../../lib/components/CodeView.svelte'
  import Badge from '../../lib/components/Badge.svelte'
  import Button from '../../lib/components/Button.svelte'
  import SegmentedControl from '../../lib/components/SegmentedControl.svelte'
  import Select from '../../lib/components/Select.svelte'
  import { toolById } from '../../lib/tools'
  import { copyText } from '../../lib/utils/clipboard'
  import { formatShortcut } from '../../lib/utils/platform'
  import ToolLayout from '../shared/ToolLayout.svelte'
  import InputPane from '../shared/InputPane.svelte'
  import OutputPane from '../shared/OutputPane.svelte'
  import { samples } from '../shared/samples'
  import { sizeLabel } from '../shared/detect'
  import { TransformState, liveTransform } from '../shared/transform.svelte'
  import { toolCommands, toolShortcuts, toolStatus } from '../shared/tool.svelte'

  let { active }: { active: boolean } = $props()

  type JsonMode = 'json-pretty' | 'json-min' | 'validate'

  const tool = toolById('json')!
  const session = new TransformState()
  let mode = $state<JsonMode>('json-pretty')
  let indent = $state('  ')
  let editor = $state<CodeView>()

  const labels: Record<JsonMode, string> = { 'json-pretty': 'Pretty-print', 'json-min': 'Minify', validate: 'Validate' }
  const options = () => ({ mode, indent: mode === 'json-pretty' ? indent : undefined })
  liveTransform(session, options)

  const run = () => session.run(options())
  const copy = () => copyText(session.output)
  function clear() {
    session.clear()
    editor?.focus()
  }
  function swap() {
    if (!session.output || mode === 'validate') return
    session.input = session.output
    editor?.focus()
  }
  const jump = (line: number, column: number) => editor?.selectPosition(line, column)

  toolShortcuts(() => active, { run, copy, clear, swap })
  toolCommands(() => active, () => [
    { id: 'json.pretty', title: 'JSON: Pretty-print', group: 'JSON', run: () => (mode = 'json-pretty') },
    { id: 'json.min', title: 'JSON: Minify', group: 'JSON', run: () => (mode = 'json-min') },
    { id: 'json.validate', title: 'JSON: Validate', group: 'JSON', run: () => (mode = 'validate') },
  ])
  toolStatus(() => active, () => {
    const e = session.error
    if (e) return { text: e.line ? `Invalid JSON · line ${e.line}, col ${e.column}` : 'Invalid JSON', tone: 'err' }
    if (session.result && mode === 'validate')
      return session.result.kind === 'json'
        ? { text: 'Valid JSON', tone: 'ok' }
        : { text: `Not JSON — looks like ${session.result.kind}`, tone: 'warn' }
    if (session.result) return { text: mode === 'json-min' ? 'Minified' : 'Formatted', tone: 'ok' }
    return { text: 'Ready' }
  })
</script>

<ToolLayout {tool}>
  {#snippet controls()}
    <SegmentedControl
      label="Action"
      bind:value={mode}
      options={[
        { value: 'json-pretty', label: 'Pretty' },
        { value: 'json-min', label: 'Minify' },
        { value: 'validate', label: 'Validate' },
      ]}
    />
    <Select
      label="Indentation"
      bind:value={indent}
      disabled={mode !== 'json-pretty'}
      items={[
        { value: '  ', label: '2 spaces' },
        { value: '    ', label: '4 spaces' },
        { value: '\t', label: 'Tabs' },
      ]}
    />
  {/snippet}
  {#snippet actions()}
    <Button variant="primary" icon={Play} onclick={run} title={formatShortcut('Mod+Enter')}>{labels[mode]}</Button>
  {/snippet}
  {#snippet input()}
    <InputPane
      {session}
      bind:editor
      onclear={clear}
      language="json"
      placeholder="Paste JSON — errors point to the exact line and column"
      onpaste={run}
    />
  {/snippet}
  {#snippet output()}
    {#if mode === 'validate'}
      <OutputPane
        {session}
        emptyIcon={Braces}
        emptyTitle="Validation result appears here"
        emptyDescription="Errors link straight to the offending character."
        onsample={() => (session.input = samples.json)}
        onjump={jump}
      >
        {#snippet view(res)}
          <div class="verdict">
            {#if res.kind === 'json'}
              <CircleCheck size={28} strokeWidth={1.75} class="ok-icon" />
              <p class="headline">Valid JSON</p>
            {:else}
              <Badge tone="warn">{res.kind}</Badge>
              <p class="headline">Not JSON, but valid {res.kind}</p>
            {/if}
            <p class="sub">{sizeLabel(session.input)}</p>
          </div>
        {/snippet}
      </OutputPane>
    {:else}
      <OutputPane
        {session}
        emptyIcon={Braces}
        emptyTitle={mode === 'json-min' ? 'Minified JSON appears here' : 'Formatted JSON appears here'}
        emptyDescription="Type or paste on the left — output updates as you type."
        onsample={() => (session.input = samples.json)}
        onswap={swap}
        onjump={jump}
      />
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
  }
  .verdict :global(.ok-icon) {
    color: var(--ok);
  }
  .headline {
    font-size: var(--fs-xl);
    font-weight: var(--fw-semibold);
  }
  .sub {
    color: var(--fg-1);
  }
</style>
