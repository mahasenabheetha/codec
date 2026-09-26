<script lang="ts">
  import Sparkles from '@lucide/svelte/icons/sparkles'
  import Play from '@lucide/svelte/icons/play'
  import type CodeView from '../../lib/components/CodeView.svelte'
  import Badge from '../../lib/components/Badge.svelte'
  import Button from '../../lib/components/Button.svelte'
  import type { Kind, TransformResponse } from '../../lib/api/transform'
  import { toolById } from '../../lib/tools'
  import { copyText } from '../../lib/utils/clipboard'
  import { formatShortcut } from '../../lib/utils/platform'
  import ToolLayout from '../shared/ToolLayout.svelte'
  import InputPane from '../shared/InputPane.svelte'
  import OutputPane from '../shared/OutputPane.svelte'
  import { samples } from '../shared/samples'
  import { TransformState, liveTransform } from '../shared/transform.svelte'
  import { toolShortcuts, toolStatus } from '../shared/tool.svelte'
  import JwtView from '../jwt/JwtView.svelte'
  import AnsibleView from '../ansible/AnsibleView.svelte'

  let { active }: { active: boolean } = $props()

  const tool = toolById('smart')!
  const session = new TransformState()
  let editor = $state<CodeView>()

  // What auto-detect did, in words, for each kind the engine reports.
  const detectedLabel: Record<Kind, string> = {
    json: 'JSON → encoded to base64',
    base64: 'base64 → decoded',
    jwt: 'JWT → decoded',
    ansible: 'Ansible log → analysed',
    unknown: 'unrecognised',
  }

  const options = () => ({ mode: 'auto' as const })
  liveTransform(session, options)

  const run = () => session.run(options())
  const copy = () => copyText(session.output)
  function clear() {
    session.clear()
    editor?.focus()
  }
  function swap() {
    if (!session.output) return
    session.input = session.output
    editor?.focus()
  }

  const sampleOrder = [samples.json, samples.base64Decode, samples.jwt, samples.ansible]
  let sampleIndex = 0
  function sample() {
    session.input = sampleOrder[sampleIndex++ % sampleOrder.length]
  }

  toolShortcuts(() => active, { run, copy, clear, swap })
  toolStatus(() => active, () => {
    if (session.error) return { text: session.error.message, tone: 'err' }
    if (session.result) return { text: `Detected ${session.result.kind}`, tone: 'ok' }
    return { text: 'Paste anything' }
  })
</script>

<ToolLayout {tool}>
  {#snippet actions()}
    <Button variant="primary" icon={Play} onclick={run} title={formatShortcut('Mod+Enter')}>Transform</Button>
  {/snippet}
  {#snippet input()}
    <InputPane
      {session}
      bind:editor
      onclear={clear}
      placeholder="Paste JSON, base64, a JWT or an ansible task log — codec figures out which"
      onpaste={run}
    />
  {/snippet}
  {#snippet output()}
    <OutputPane
      {session}
      emptyIcon={Sparkles}
      emptyTitle="The obvious transformation appears here"
      emptyDescription="JSON is encoded, base64 decoded (JSON inside is pretty-printed), JWTs expanded, Ansible logs analysed."
      onsample={sample}
      onswap={swap}
      view={session.result?.jwt || session.result?.task ? rich : undefined}
    >
      {#snippet badges()}
        {#if session.result}<Badge tone="accent" icon={Sparkles}>{detectedLabel[session.result.kind]}</Badge>{/if}
      {/snippet}
    </OutputPane>
  {/snippet}
</ToolLayout>

{#snippet rich(res: TransformResponse)}
  {#if res.jwt}
    <JwtView jwt={res.jwt} />
  {:else if res.task}
    <AnsibleView task={res.task} />
  {/if}
{/snippet}
