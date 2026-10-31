<script lang="ts">
  import Copy from '@lucide/svelte/icons/copy'
  import FileUp from '@lucide/svelte/icons/file-up'
  import ScanQrCode from '@lucide/svelte/icons/scan-qr-code'
  import Badge from '../../lib/components/Badge.svelte'
  import Button from '../../lib/components/Button.svelte'
  import EmptyState from '../../lib/components/EmptyState.svelte'
  import IconButton from '../../lib/components/IconButton.svelte'
  import { readQR, type QRMeaning } from '../../lib/api/qr'
  import type { ToolDef } from '../../lib/tools'
  import { copyText } from '../../lib/utils/clipboard'
  import ErrorBanner from '../shared/ErrorBanner.svelte'
  import Pane from '../shared/Pane.svelte'
  import ToolLayout from '../shared/ToolLayout.svelte'
  import ToolTabs from '../shared/ToolTabs.svelte'
  import ValueRows from '../shared/ValueRows.svelte'
  import { Job } from '../shared/job.svelte'
  import { toolShortcuts, toolStatus } from '../shared/tool.svelte'

  // A QR code read from an image: dropped, pasted (Ctrl+V) or chosen; a
  // screenshot works. The image is redrawn as PNG here, so any format the
  // browser shows can be read, then decoded on this machine. No camera;
  // nothing is stored.
  let { tool, active }: { tool: ToolDef; active: boolean } = $props()

  const job = new Job<{ text: string; meaning: QRMeaning }>()
  let preview = $state('')
  let picker = $state<HTMLInputElement>()
  let over = $state(false)

  // An SVG (which createImageBitmap refuses, and which may have no size
  // of its own) goes through an <img> and is drawn at least 1024 px wide.
  async function source(file: Blob): Promise<{ img: CanvasImageSource; w: number; h: number; done: () => void }> {
    if (file.type !== 'image/svg+xml') {
      try {
        const bmp = await createImageBitmap(file)
        return { img: bmp, w: bmp.width, h: bmp.height, done: () => bmp.close() }
      } catch {
        // fall through to an <img>
      }
    }
    const url = URL.createObjectURL(file)
    const img = new Image()
    img.src = url
    try {
      await img.decode()
    } catch {
      URL.revokeObjectURL(url)
      throw new Error('this image could not be opened')
    }
    const w0 = img.naturalWidth || 1024
    const h0 = img.naturalHeight || w0
    const k = Math.max(1, 1024 / w0)
    return { img, w: Math.round(w0 * k), h: Math.round(h0 * k), done: () => URL.revokeObjectURL(url) }
  }

  async function asPNG(file: Blob): Promise<Blob> {
    const s = await source(file)
    const canvas = document.createElement('canvas')
    canvas.width = s.w
    canvas.height = s.h
    const ctx = canvas.getContext('2d')!
    ctx.fillStyle = '#fff' // transparent areas read as light
    ctx.fillRect(0, 0, s.w, s.h)
    ctx.imageSmoothingEnabled = false
    ctx.drawImage(s.img, 0, 0, s.w, s.h)
    s.done()
    return new Promise((ok, fail) => canvas.toBlob((b) => (b ? ok(b) : fail(new Error('could not redraw the image'))), 'image/png'))
  }

  function load(file: Blob | null | undefined) {
    if (!file || !file.type.startsWith('image/')) return
    if (preview) URL.revokeObjectURL(preview)
    preview = URL.createObjectURL(file)
    job.run(async () => readQR(await asPNG(file)))
  }

  // Ctrl+V anywhere while this tab is shown.
  $effect(() => {
    if (!active) return
    const onpaste = (e: ClipboardEvent) => {
      const item = [...(e.clipboardData?.items ?? [])].find((i) => i.type.startsWith('image/'))
      if (!item) return
      e.preventDefault()
      load(item.getAsFile())
    }
    window.addEventListener('paste', onpaste)
    return () => window.removeEventListener('paste', onpaste)
  })

  const hasFiles = (e: DragEvent) => !!e.dataTransfer?.types.includes('Files')
  function ondragover(e: DragEvent) {
    if (!hasFiles(e)) return
    e.preventDefault()
    e.stopPropagation()
    over = true
  }
  function ondrop(e: DragEvent) {
    over = false
    if (!hasFiles(e)) return
    e.preventDefault()
    e.stopPropagation()
    load(e.dataTransfer?.files[0])
  }

  const kindName: Record<QRMeaning['kind'], string> = {
    url: 'Link',
    wifi: 'Wi-Fi network',
    otpauth: 'Authenticator setup',
    email: 'Email',
    phone: 'Phone number',
    sms: 'Text message',
    geo: 'Place',
    text: 'Text',
  }
  const result = $derived(job.result)
  const rows = $derived((result?.meaning.fields ?? []).map((f) => ({ label: f.name, value: f.value })))

  toolShortcuts(() => active, { run: () => picker?.click(), copy: () => result && copyText(result.text, 'Text'), clear: () => (job.reset(), (preview = '')) })
  toolStatus(() => active, () => {
    if (job.error) return { text: job.error.message, tone: 'err' }
    if (job.busy) return { text: 'Reading…' }
    if (result) return { text: `${kindName[result.meaning.kind]} · ${result.text.length} characters`, tone: 'ok' }
    return { text: 'Drop, paste or choose an image' }
  })
</script>

<input bind:this={picker} type="file" accept="image/*" hidden onchange={(e) => (load((e.currentTarget as HTMLInputElement).files?.[0]), ((e.currentTarget as HTMLInputElement).value = ''))} />

<ToolLayout {tool}>
  {#snippet controls()}
    <ToolTabs {tool} />
  {/snippet}
  {#snippet actions()}
    <Button variant="primary" icon={FileUp} tone="open" onclick={() => picker?.click()}>Choose image…</Button>
  {/snippet}
  {#snippet input()}
    <Pane title="Image">
      <!-- svelte-ignore a11y_no_static_element_interactions -->
      <div class="drop" class:over {ondragover} ondragleave={() => (over = false)} {ondrop}>
        {#if preview}
          <img src={preview} alt="What was dropped or pasted" />
        {:else}
          <EmptyState icon={ScanQrCode} title="Drop, paste or choose an image" description="A photo or a screenshot with a QR code in it: PNG, JPEG, WebP, GIF… Ctrl+V pastes one you copied.">
            <Button icon={FileUp} tone="open" onclick={() => picker?.click()}>Choose image…</Button>
          </EmptyState>
        {/if}
      </div>
    </Pane>
  {/snippet}
  {#snippet output()}
    <Pane title="What it says">
      {#snippet meta()}
        {#if result}<Badge tone="accent">{kindName[result.meaning.kind]}</Badge>{/if}
      {/snippet}
      {#snippet actions()}
        <IconButton icon={Copy} tone="copy" label="Copy the text" shortcut="Alt+C" size="sm" disabled={!result} onclick={() => result && copyText(result.text, 'Text')} />
      {/snippet}
      {#if job.error}
        <ErrorBanner error={job.error} />
      {:else if result}
        <div class="result">
          <code class="text">{result.text}</code>
          {#if rows.length}<ValueRows {rows} />{/if}
          {#if result.meaning.kind === 'otpauth'}<p class="hint">This sets up an authenticator: anyone with it can make your codes.</p>{/if}
        </div>
      {:else}
        <EmptyState icon={ScanQrCode} title={job.busy ? 'Reading…' : 'The text appears here'} description="Links, Wi-Fi logins and authenticator setups are split into their parts." />
      {/if}
    </Pane>
  {/snippet}
</ToolLayout>

<style>
  .drop {
    flex: 1;
    min-height: 0;
    display: flex;
    align-items: center;
    justify-content: center;
    padding: var(--s-3);
    border-radius: var(--r-md);
  }
  .drop.over {
    outline: 2px dashed var(--accent);
    outline-offset: -6px;
  }
  img {
    max-width: 100%;
    max-height: 100%;
    object-fit: contain;
    border-radius: var(--r-sm);
  }
  .result {
    flex: 1;
    min-height: 0;
    overflow: auto;
    padding: var(--s-3) var(--s-4);
    display: flex;
    flex-direction: column;
    gap: var(--s-3);
  }
  .text {
    font-size: var(--fs-md);
    color: var(--fg-0);
    overflow-wrap: anywhere;
    user-select: all;
  }
  .hint {
    margin: 0;
    font-size: var(--fs-sm);
    color: var(--warn);
  }
</style>
