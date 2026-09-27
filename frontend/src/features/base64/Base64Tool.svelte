<script lang="ts">
  import Binary from '@lucide/svelte/icons/binary'
  import Play from '@lucide/svelte/icons/play'
  import type CodeView from '../../lib/components/CodeView.svelte'
  import Button from '../../lib/components/Button.svelte'
  import SegmentedControl from '../../lib/components/SegmentedControl.svelte'
  import Toggle from '../../lib/components/Toggle.svelte'
  import { toolById } from '../../lib/tools'
  import { copyText } from '../../lib/utils/clipboard'
  import { formatShortcut } from '../../lib/utils/platform'
  import ToolLayout from '../shared/ToolLayout.svelte'
  import InputPane from '../shared/InputPane.svelte'
  import OutputPane from '../shared/OutputPane.svelte'
  import { samples } from '../shared/samples'
  import { TransformState, liveTransform } from '../shared/transform.svelte'
  import { toolCommands, toolShortcuts, toolStatus } from '../shared/tool.svelte'

  let { active }: { active: boolean } = $props()

  const tool = toolById('base64')!
  const session = new TransformState()
  let mode = $state<'b64-encode' | 'b64-decode'>('b64-encode')
  let urlSafe = $state(false)
  let editor = $state<CodeView>()

  const encoding = $derived(mode === 'b64-encode')
  const options = () => ({ mode, urlSafe })
  liveTransform(session, options)

  const run = () => session.run(options())
  const copy = () => copyText(session.output)
  function clear() {
    session.clear()
    editor?.focus()
  }
  // Swapping also flips direction: encode → swap → decode round-trips.
  function swap() {
    if (!session.output) return
    session.input = session.output
    mode = encoding ? 'b64-decode' : 'b64-encode'
    editor?.focus()
  }
  function sample() {
    session.input = encoding ? samples.base64Encode : samples.base64Decode
  }

  toolShortcuts(() => active, { run, copy, clear, swap })
  toolCommands(() => active, () => [
    { id: 'base64.encode', title: 'Base64: Encode', group: 'Base64', run: () => (mode = 'b64-encode') },
    { id: 'base64.decode', title: 'Base64: Decode', group: 'Base64', run: () => (mode = 'b64-decode') },
    { id: 'base64.urlsafe', title: 'Base64: Toggle URL-safe alphabet', group: 'Base64', run: () => (urlSafe = !urlSafe) },
    { id: 'base64.swap', title: 'Base64: Use output as input', group: 'Base64', shortcut: 'Alt+S', run: swap },
  ])
  toolStatus(() => active, () => {
    if (session.error) return { text: encoding ? 'Encoding failed' : 'Not valid base64', tone: 'err' }
    if (session.result) return { text: `${encoding ? 'Encoded' : 'Decoded'}${urlSafe ? ' · URL-safe' : ''}`, tone: 'ok' }
    return { text: encoding ? 'Ready to encode' : 'Ready to decode' }
  })
</script>

<ToolLayout {tool}>
  {#snippet controls()}
    <SegmentedControl
      label="Direction"
      bind:value={mode}
      options={[
        { value: 'b64-encode', label: 'Encode' },
        { value: 'b64-decode', label: 'Decode' },
      ]}
    />
    <Toggle label="URL-safe" bind:checked={urlSafe} title="Use - and _ instead of + and / (JWTs, URLs)" />
  {/snippet}
  {#snippet actions()}
    <Button variant="primary" icon={Play} onclick={run} title={formatShortcut('Mod+Enter')}>
      {encoding ? 'Encode' : 'Decode'}
    </Button>
  {/snippet}
  {#snippet input()}
    <InputPane
      {session}
      bind:editor
      onclear={clear}
      placeholder={encoding ? 'Text to encode…' : 'Base64 to decode — whitespace and missing padding are fine'}
      onpaste={run}
    />
  {/snippet}
  {#snippet output()}
    <OutputPane
      {session}
      emptyIcon={Binary}
      emptyTitle={encoding ? 'Encoded text appears here' : 'Decoded text appears here'}
      emptyDescription="Type or paste on the left. Decoded JSON is pretty-printed automatically."
      onsample={sample}
      onswap={swap}
    />
  {/snippet}
</ToolLayout>
