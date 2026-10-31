<script lang="ts">
  import ClipboardPaste from '@lucide/svelte/icons/clipboard-paste'
  import Eraser from '@lucide/svelte/icons/eraser'
  import FolderOpen from '@lucide/svelte/icons/folder-open'
  import CodeView from '../../lib/components/CodeView.svelte'
  import IconButton from '../../lib/components/IconButton.svelte'
  import { toast } from '../../lib/stores/toast.svelte'
  import Pane from '../shared/Pane.svelte'
  import { sizeLabel } from '../shared/detect'

  // One side of the text diff: type, paste, open or drop a file. The
  // file is read here in the page; it goes no further than the diff.
  interface Props {
    title: string
    value: string
    placeholder: string
    /** The dropped or opened file's name, shown beside the title. */
    name?: string
  }

  let { title, value = $bindable(), placeholder, name = $bindable('') }: Props = $props()

  const maxSize = 5 << 20
  let picker = $state<HTMLInputElement>()
  let over = $state(false)

  async function load(f: File | undefined) {
    if (!f) return
    if (f.size > maxSize) {
      toast(`${f.name} is over 5 MB; compare a smaller part`, 'warn')
      return
    }
    const text = await f.text()
    if (text.includes('\u0000')) {
      toast(`${f.name} isn't text`, 'warn')
      return
    }
    value = text
    name = f.name
  }
  async function paste() {
    try {
      value = await navigator.clipboard.readText()
      name = ''
    } catch {
      toast('Clipboard access was blocked — paste with Ctrl+V instead', 'warn')
    }
  }
  function ondrop(e: DragEvent) {
    e.preventDefault()
    over = false
    load(e.dataTransfer?.files[0])
  }
</script>

<!-- svelte-ignore a11y_no_static_element_interactions -->
<div class="side" class:over ondragover={(e) => (e.preventDefault(), (over = true))} ondragleave={() => (over = false)} {ondrop}>
  <Pane {title}>
    {#snippet meta()}{name ? `${name} · ` : ''}{sizeLabel(value)}{/snippet}
    {#snippet actions()}
      <IconButton icon={FolderOpen} label="Open a file" size="sm" onclick={() => picker?.click()} />
      <IconButton icon={ClipboardPaste} label="Paste from clipboard" size="sm" onclick={paste} />
      <IconButton icon={Eraser} label="Clear" size="sm" disabled={!value} onclick={() => ((value = ''), (name = ''))} />
    {/snippet}
    <CodeView bind:value label={title} {placeholder} wrap />
  </Pane>
  <input bind:this={picker} type="file" hidden onchange={(e) => load(e.currentTarget.files?.[0])} />
</div>

<style>
  .side {
    min-width: 0;
    min-height: 0;
    display: flex;
    flex-direction: column;
    border-radius: var(--r-lg);
  }
  .side.over {
    outline: 2px dashed var(--accent);
    outline-offset: -2px;
  }
</style>
