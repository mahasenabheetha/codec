<script lang="ts">
  import ClipboardPaste from '@lucide/svelte/icons/clipboard-paste'
  import Eraser from '@lucide/svelte/icons/eraser'
  import Search from '@lucide/svelte/icons/search'
  import CodeView, { type Language } from '../../lib/components/CodeView.svelte'
  import IconButton from '../../lib/components/IconButton.svelte'
  import Button from '../../lib/components/Button.svelte'
  import { toast } from '../../lib/stores/toast.svelte'
  import FindBar from './FindBar.svelte'
  import { findShortcut } from './find.svelte'
  import Pane from './Pane.svelte'
  import { displayLanguage, sizeLabel } from './detect'

  // Any session with an input: TransformState, or a tool's own state.
  type Session = { input: string; clear(): void }

  interface Props {
    session: Session
    placeholder: string
    /** 'auto' colours the input by what it holds (JSON, else plain text). */
    language?: Language | 'auto'
    onpaste?: () => void
    onclear?: () => void
    editor?: CodeView
  }

  let { session, placeholder, language = 'text', onpaste, onclear, editor = $bindable() }: Props = $props()

  const shown = $derived<Language>(language === 'auto' ? displayLanguage(session.input) : language)

  async function pasteFromClipboard() {
    try {
      session.input = await navigator.clipboard.readText()
      onpaste?.()
    } catch {
      toast('Clipboard access was blocked — paste with Ctrl+V instead', 'warn')
    }
  }

  // Find in the input: the button, or Ctrl+F over or inside the pane.
  // Always on, even while empty, so Ctrl+F in the editor never opens the
  // editor's own search panel instead.
  let finding = $state(false)
  let over = $state(false)
  let pane = $state<HTMLDivElement>()
  findShortcut(
    {
      get pane() {
        return pane
      },
      get over() {
        return over
      },
      enabled: true,
    },
    () => (finding = true),
  )
  function closeFind() {
    finding = false
    editor?.focus()
  }
</script>

<!-- svelte-ignore a11y_no_static_element_interactions -->
<div class="in" bind:this={pane} onpointerenter={() => (over = true)} onpointerleave={() => (over = false)}>
  <Pane title="Input">
    {#snippet meta()}{sizeLabel(session.input)}{/snippet}
    {#snippet actions()}
      <IconButton icon={Search} label="Find in the input" shortcut="Ctrl+F" size="sm" active={finding} onclick={() => (finding = !finding)} />
      <IconButton icon={ClipboardPaste} label="Paste from clipboard" size="sm" onclick={pasteFromClipboard} />
      <Button
        icon={Eraser}
        tone="clear"
        variant="ghost"
        size="sm"
        title="Clear the input (Escape)"
        aria-keyshortcuts="Escape"
        disabled={!session.input}
        onclick={() => (onclear ? onclear() : session.clear())}>Clear</Button
      >
    {/snippet}
    {#if finding}
      <FindBar find={(q, d) => editor?.find(q, d) ?? { count: 0, index: 0 }} text={session.input} what="input" stay onclose={closeFind} />
    {/if}
    <CodeView bind:this={editor} bind:value={session.input} label="Input" language={shown} {placeholder} {onpaste} wrap />
  </Pane>
</div>

<style>
  .in {
    display: contents;
  }
</style>
