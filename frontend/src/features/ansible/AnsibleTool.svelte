<script lang="ts">
  import ScrollText from '@lucide/svelte/icons/scroll-text'
  import Play from '@lucide/svelte/icons/play'
  import type CodeView from '../../lib/components/CodeView.svelte'
  import Button from '../../lib/components/Button.svelte'
  import { toolById } from '../../lib/tools'
  import { copyText } from '../../lib/utils/clipboard'
  import { formatShortcut } from '../../lib/utils/platform'
  import ToolLayout from '../shared/ToolLayout.svelte'
  import InputPane from '../shared/InputPane.svelte'
  import OutputPane from '../shared/OutputPane.svelte'
  import { samples } from '../shared/samples'
  import { TransformState, liveTransform } from '../shared/transform.svelte'
  import { toolShortcuts, toolStatus } from '../shared/tool.svelte'
  import AnsibleView from './AnsibleView.svelte'

  let { active }: { active: boolean } = $props()

  const tool = toolById('ansible')!
  const session = new TransformState()
  let editor = $state<CodeView>()

  const options = () => ({ mode: 'ansible' as const })
  liveTransform(session, options)

  const run = () => session.run(options())
  const copy = () => copyText(session.output, 'Summary')
  function clear() {
    session.clear()
    editor?.focus()
  }

  toolShortcuts(() => active, { run, copy, clear })
  toolStatus(() => active, () => {
    if (session.error) return { text: 'Not an ansible task log', tone: 'err' }
    const task = session.result?.task
    if (!task) return { text: 'Ready' }
    const bad = task.status === 'FAILED' || task.status === 'UNREACHABLE'
    return { text: `${task.status}${task.name ? ' · ' + task.name : ''}`, tone: bad ? 'err' : 'ok' }
  })
</script>

<ToolLayout {tool}>
  {#snippet actions()}
    <Button variant="primary" icon={Play} onclick={run} title={formatShortcut('Mod+Enter')}>Parse log</Button>
  {/snippet}
  {#snippet input()}
    <InputPane
      {session}
      bind:editor
      onclear={clear}
      placeholder="Paste one task's ansible -vv output, from its TASK [...] line to the result — CI prefixes are fine"
      onpaste={run}
    />
  {/snippet}
  {#snippet output()}
    <OutputPane
      {session}
      emptyIcon={ScrollText}
      emptyTitle="Task analysis appears here"
      emptyDescription="Status, probable cause, key fields and highlighted stderr/stdout."
      onsample={() => (session.input = samples.ansible)}
    >
      {#snippet view(res)}
        {#if res.task}<AnsibleView task={res.task} />{/if}
      {/snippet}
    </OutputPane>
  {/snippet}
</ToolLayout>
