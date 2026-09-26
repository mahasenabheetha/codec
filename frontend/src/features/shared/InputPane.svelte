<script lang="ts">
  import ClipboardPaste from '@lucide/svelte/icons/clipboard-paste'
  import Eraser from '@lucide/svelte/icons/eraser'
  import CodeView, { type Language } from '../../lib/components/CodeView.svelte'
  import IconButton from '../../lib/components/IconButton.svelte'
  import { toast } from '../../lib/stores/toast.svelte'
  import Pane from './Pane.svelte'
  import { sizeLabel } from './detect'
  import type { TransformState } from './transform.svelte'

  interface Props {
    session: TransformState
    placeholder: string
    language?: Language
    onpaste?: () => void
    onclear?: () => void
    editor?: CodeView
  }

  let { session, placeholder, language = 'text', onpaste, onclear, editor = $bindable() }: Props = $props()

  async function pasteFromClipboard() {
    try {
      session.input = await navigator.clipboard.readText()
      onpaste?.()
    } catch {
      toast('Clipboard access was blocked — paste with Ctrl+V instead', 'warn')
    }
  }
</script>

<Pane title="Input">
  {#snippet meta()}{sizeLabel(session.input)}{/snippet}
  {#snippet actions()}
    <IconButton icon={ClipboardPaste} label="Paste from clipboard" size="sm" onclick={pasteFromClipboard} />
    <IconButton
      icon={Eraser}
      label="Clear"
      shortcut="Escape"
      size="sm"
      disabled={!session.input}
      onclick={() => (onclear ? onclear() : session.clear())}
    />
  {/snippet}
  <CodeView bind:this={editor} bind:value={session.input} label="Input" {language} {placeholder} {onpaste} wrap />
</Pane>
