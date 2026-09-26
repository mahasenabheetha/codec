<script lang="ts">
  import KeyRound from '@lucide/svelte/icons/key-round'
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
  import JwtView from './JwtView.svelte'
  import { validity } from './time'

  let { active }: { active: boolean } = $props()

  const tool = toolById('jwt')!
  const session = new TransformState()
  let editor = $state<CodeView>()

  const options = () => ({ mode: 'jwt' as const })
  liveTransform(session, options)

  const run = () => session.run(options())
  const copy = () => copyText(session.output)
  function clear() {
    session.clear()
    editor?.focus()
  }

  toolShortcuts(() => active, { run, copy, clear })
  toolStatus(() => active, () => {
    if (session.error) return { text: 'Not a valid JWT', tone: 'err' }
    const jwt = session.result?.jwt
    if (!jwt) return { text: 'Ready' }
    try {
      const p = JSON.parse(jwt.payload)
      const v = validity(p.exp, p.nbf)
      if (v === 'expired') return { text: 'Token expired', tone: 'err' }
      if (v === 'not-yet') return { text: 'Token not yet valid', tone: 'warn' }
    } catch {
      /* payload is always JSON when decoding succeeded */
    }
    return { text: 'Decoded · signature not verified', tone: 'ok' }
  })
</script>

<ToolLayout {tool}>
  {#snippet actions()}
    <Button variant="primary" icon={Play} onclick={run} title={formatShortcut('Mod+Enter')}>Decode</Button>
  {/snippet}
  {#snippet input()}
    <InputPane
      {session}
      bind:editor
      onclear={clear}
      placeholder="Paste a JWT (header.payload.signature), without the 'Bearer ' prefix"
      onpaste={run}
    />
  {/snippet}
  {#snippet output()}
    <OutputPane
      {session}
      emptyIcon={KeyRound}
      emptyTitle="Decoded claims appear here"
      emptyDescription="Expiry is checked against your clock. Signatures are never verified."
      onsample={() => (session.input = samples.jwt)}
    >
      {#snippet view(res)}
        {#if res.jwt}<JwtView jwt={res.jwt} />{/if}
      {/snippet}
    </OutputPane>
  {/snippet}
</ToolLayout>
