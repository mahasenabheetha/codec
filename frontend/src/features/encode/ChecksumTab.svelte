<script lang="ts">
  import { untrack } from 'svelte'
  import CircleCheck from '@lucide/svelte/icons/circle-check'
  import CircleX from '@lucide/svelte/icons/circle-x'
  import Copy from '@lucide/svelte/icons/copy'
  import FileCheck from '@lucide/svelte/icons/file-check'
  import FileUp from '@lucide/svelte/icons/file-up'
  import FolderOpen from '@lucide/svelte/icons/folder-open'
  import X from '@lucide/svelte/icons/x'
  import Badge from '../../lib/components/Badge.svelte'
  import Button from '../../lib/components/Button.svelte'
  import EmptyState from '../../lib/components/EmptyState.svelte'
  import IconButton from '../../lib/components/IconButton.svelte'
  import Toggle from '../../lib/components/Toggle.svelte'
  import { checksumFile, checksumPath, parseExpected, type Digest, type Expected } from '../../lib/api/encode'
  import type { ToolDef } from '../../lib/tools'
  import { copyText } from '../../lib/utils/clipboard'
  import ErrorBanner from '../shared/ErrorBanner.svelte'
  import Pane from '../shared/Pane.svelte'
  import ToolLayout from '../shared/ToolLayout.svelte'
  import ToolTabs from '../shared/ToolTabs.svelte'
  import { Job } from '../shared/job.svelte'
  import { toolShortcuts, toolStatus } from '../shared/tool.svelte'
  import { workspace } from '../workspace/workspace.svelte'

  // Checksums of files: dropped or chosen (sent as they are, hashed as a
  // stream on this machine, never kept) or from the open folder; every
  // algorithm at once. Paste an expected value, or a checksums file, to
  // see which files match.
  let { tool, active }: { tool: ToolDef; active: boolean } = $props()

  interface Row {
    key: number
    name: string
    folder: boolean // a path in the open folder, not a file sent
    size?: number
    digests?: Digest[]
    error?: string
    busy: boolean
  }
  let rows = $state<Row[]>([])
  let nextKey = 0
  let picker = $state<HTMLInputElement>()
  let expected = $state('')
  let everyAlgo = $state(false)
  const exp = new Job<{ expected: Expected[] }>()

  $effect(() => {
    const text = expected
    untrack(() => exp.schedule(text.trim() ? () => parseExpected(text) : null, 200))
  })
  const entries = $derived(exp.result?.expected ?? [])

  async function add(name: string, folder: boolean, run: () => Promise<{ digests: Digest[]; size: number }>) {
    rows.push({ key: ++nextKey, name, folder, busy: true })
    const row = rows[rows.length - 1]
    try {
      const r = await run()
      row.digests = r.digests
      row.size = r.size
    } catch (e) {
      row.error = e instanceof Error ? e.message : String(e)
    } finally {
      row.busy = false
    }
  }
  function addFiles(files: FileList | File[] | null | undefined) {
    for (const f of files ?? []) add(f.name, false, () => checksumFile(f))
  }
  const addPath = (p: string) => add(p, true, () => checksumPath(p))

  // Files a checksums file names that aren't here yet, checked in the
  // open folder on request.
  const base = (p: string) => p.split(/[\\/]/).pop() ?? p
  const named = $derived(entries.filter((e) => e.name))
  const unchecked = $derived(named.filter((e) => !rows.some((r) => r.name === e.name || base(r.name) === base(e.name!))))
  const checkInFolder = () => unchecked.forEach((e) => addPath(e.name!))

  // What a row is compared with: the one value pasted, or its line in a
  // checksums file (by path, else by file name).
  function expectedFor(r: Row): Expected | undefined {
    if (entries.length === 1 && !entries[0].name) return entries[0]
    return entries.find((e) => e.name === r.name) ?? entries.find((e) => e.name && base(e.name) === base(r.name))
  }
  function verdict(r: Row): 'ok' | 'bad' | 'none' {
    const e = expectedFor(r)
    if (!e || !r.digests) return 'none'
    return r.digests.find((d) => d.algorithm === e.algorithm)?.hex === e.hex ? 'ok' : 'bad'
  }
  function shown(r: Row): Digest[] {
    if (!r.digests) return []
    if (everyAlgo) return r.digests
    const e = expectedFor(r)
    return r.digests.filter((d) => d.algorithm === 'SHA-256' || d.algorithm === e?.algorithm)
  }

  const done = $derived(rows.filter((r) => r.digests))
  const counts = $derived({ ok: rows.filter((r) => verdict(r) === 'ok').length, bad: rows.filter((r) => verdict(r) === 'bad').length })
  // sha256sum lines, for a checksums file of your own.
  const asLines = () => done.map((r) => `${r.digests!.find((d) => d.algorithm === 'SHA-256')!.hex}  ${r.name}`).join('\n') + '\n'

  function clear() {
    rows = []
    expected = ''
  }
  const size = (n?: number) => (n === undefined ? '' : n < 1024 ? `${n} B` : n < 1 << 20 ? `${(n / 1024).toFixed(1)} KB` : n < 1 << 30 ? `${(n / (1 << 20)).toFixed(1)} MB` : `${(n / (1 << 30)).toFixed(2)} GB`)

  let over = $state(false)
  const hasFiles = (e: DragEvent) => !!e.dataTransfer?.types.includes('Files')
  function ondragover(e: DragEvent) {
    if (!hasFiles(e)) return
    e.preventDefault()
    e.stopPropagation() // keep the drop from the desktop runtime at the page root
    over = true
  }
  function ondrop(e: DragEvent) {
    over = false
    if (!hasFiles(e)) return
    e.preventDefault()
    e.stopPropagation()
    addFiles(e.dataTransfer?.files)
  }

  toolShortcuts(() => active, { run: () => picker?.click(), copy: () => done.length && copyText(asLines(), 'Checksums'), clear })
  toolStatus(() => active, () => {
    if (exp.error) return { text: exp.error.message, tone: 'err' }
    if (rows.some((r) => r.busy)) return { text: 'Hashing…' }
    if (!rows.length) return { text: entries.length ? `${entries.length} expected ${entries.length === 1 ? 'checksum' : 'checksums'} · add the files` : 'Ready' }
    if (counts.bad) return { text: `${counts.bad} of ${rows.length} do not match`, tone: 'err' }
    if (counts.ok) return { text: `${counts.ok} of ${rows.length} match${unchecked.length ? ` · ${unchecked.length} not checked` : ''}`, tone: 'ok' }
    return { text: `${done.length} ${done.length === 1 ? 'file' : 'files'} hashed`, tone: 'ok' }
  })
</script>

<input bind:this={picker} type="file" multiple hidden onchange={(e) => (addFiles((e.currentTarget as HTMLInputElement).files), ((e.currentTarget as HTMLInputElement).value = ''))} />

<ToolLayout {tool}>
  {#snippet controls()}
    <ToolTabs {tool} />
    <Toggle label="Every algorithm" bind:checked={everyAlgo} title="SHA-256, SHA-384, SHA-512, SHA-1 and MD5; otherwise SHA-256 and the one expected" />
  {/snippet}
  {#snippet actions()}
    <Button icon={FolderOpen} tone="open" disabled={!workspace.info} title={workspace.info ? 'A file in the open folder' : 'Open a folder first'} onclick={() => workspace.pickFile('Checksum which file?', addPath)}>From the folder…</Button>
    <Button variant="primary" icon={FileUp} tone="open" onclick={() => picker?.click()}>Choose files…</Button>
  {/snippet}
  {#snippet body()}
    <!-- svelte-ignore a11y_no_static_element_interactions -->
    <div class="wrap" class:over {ondragover} ondragleave={(e) => !(e.currentTarget as HTMLElement).contains(e.relatedTarget as Node | null) && (over = false)} {ondrop}>
      <Pane title="Expected" grow={false}>
        {#snippet meta()}
          {#if entries.length}<span>{entries.length} {entries.length === 1 ? 'checksum' : 'checksums'}{named.length ? ` for ${named.length} ${named.length === 1 ? 'file' : 'files'}` : ''}</span>{/if}
        {/snippet}
        <textarea
          bind:value={expected}
          rows="3"
          spellcheck="false"
          aria-label="Expected checksum"
          placeholder="Optional: paste the expected checksum (hex or base64) or a checksums file (sha256sum or BSD lines); the algorithm follows from its length"
        ></textarea>
        {#if exp.error}<ErrorBanner error={exp.error} />{/if}
        {#if unchecked.length}
          <div class="pending">
            <span>Not checked yet: {unchecked.slice(0, 6).map((e) => e.name).join(', ')}{unchecked.length > 6 ? ` and ${unchecked.length - 6} more` : ''}</span>
            {#if workspace.info}<Button size="sm" icon={FileCheck} onclick={checkInFolder}>Check in the open folder</Button>{/if}
          </div>
        {/if}
      </Pane>
      <Pane title="Files">
        {#snippet meta()}
          {#if counts.ok}<Badge tone="ok" icon={CircleCheck}>{counts.ok} match</Badge>{/if}
          {#if counts.bad}<Badge tone="err" icon={CircleX}>{counts.bad} different</Badge>{/if}
        {/snippet}
        {#snippet actions()}
          <Button icon={Copy} tone="copy" variant="ghost" size="sm" title="Copy as sha256sum lines (Alt+C)" disabled={!done.length} onclick={() => copyText(asLines(), 'Checksums')}>Copy</Button>
          <IconButton icon={X} label="Clear" size="sm" disabled={!rows.length && !expected} onclick={clear} />
        {/snippet}
        {#if rows.length}
          <ul class="files">
            {#each rows as r (r.key)}
              {@const v = verdict(r)}
              <li class={v}>
                <div class="head">
                  {#if v === 'ok'}<CircleCheck size={16} strokeWidth={2} class="ok" />{:else if v === 'bad'}<CircleX size={16} strokeWidth={2} class="bad" />{/if}
                  <span class="name" title={r.name}>{r.name}</span>
                  <span class="muted">{r.folder ? 'open folder · ' : ''}{size(r.size)}</span>
                  {#if r.busy}<span class="muted">hashing…</span>{/if}
                  {#if v === 'ok'}<Badge tone="ok">matches {expectedFor(r)?.algorithm}</Badge>{:else if v === 'bad'}<Badge tone="err">does not match</Badge>{/if}
                  <span class="grow"></span>
                  <IconButton icon={X} label="Remove" size="sm" onclick={() => (rows = rows.filter((x) => x.key !== r.key))} />
                </div>
                {#if r.error}<p class="error">{r.error}</p>{/if}
                {#each shown(r) as d (d.algorithm)}
                  <div class="digest" class:want={expectedFor(r)?.algorithm === d.algorithm}>
                    <span class="alg">{d.algorithm}{d.weak ? ' (weak)' : ''}</span>
                    <code>{d.hex}</code>
                    <IconButton icon={Copy} tone="copy" label="Copy {d.algorithm}" size="sm" onclick={() => copyText(d.hex, d.algorithm)} />
                  </div>
                {/each}
                {#if v === 'bad'}
                  <div class="digest want"><span class="alg">expected</span><code>{expectedFor(r)?.hex}</code></div>
                {/if}
              </li>
            {/each}
          </ul>
        {:else}
          <EmptyState icon={FileCheck} title="Drop files here" description="Each is hashed with SHA-256, SHA-384, SHA-512, SHA-1 and MD5 in one pass, on this machine, whatever its size; nothing is kept. Paste what the download page lists above to see if they match.">
            <Button icon={FileUp} tone="open" onclick={() => picker?.click()}>Choose files…</Button>
          </EmptyState>
        {/if}
      </Pane>
    </div>
  {/snippet}
</ToolLayout>

<style>
  .wrap {
    flex: 1;
    min-height: 0;
    display: flex;
    flex-direction: column;
    gap: var(--s-3);
    border-radius: var(--r-lg);
  }
  .wrap.over {
    outline: 2px dashed var(--accent);
    outline-offset: 2px;
  }
  textarea {
    width: 100%;
    min-height: 64px;
    padding: var(--s-2) var(--s-3);
    font-family: var(--font-mono);
    font-size: var(--fs-sm);
    color: var(--fg-0);
    background: var(--bg-0);
    border: none;
    resize: vertical;
  }
  textarea:focus {
    outline: none;
  }
  .pending {
    display: flex;
    align-items: center;
    gap: var(--s-3);
    padding: var(--s-2) var(--s-3);
    font-size: var(--fs-sm);
    color: var(--fg-1);
    border-top: 1px solid var(--border);
  }
  .files {
    flex: 1;
    min-height: 0;
    overflow: auto;
    margin: 0;
    padding: var(--s-3);
    list-style: none;
    display: flex;
    flex-direction: column;
    gap: var(--s-2);
  }
  li {
    padding: var(--s-2) var(--s-3);
    background: var(--bg-1);
    border: 1px solid var(--border);
    border-radius: var(--r-md);
  }
  li.ok {
    border-color: color-mix(in srgb, var(--ok) 50%, var(--border));
  }
  li.bad {
    border-color: color-mix(in srgb, var(--err) 50%, var(--border));
  }
  .head {
    display: flex;
    align-items: center;
    gap: var(--s-2);
    min-width: 0;
  }
  .head :global(.ok) {
    color: var(--ok);
  }
  .head :global(.bad) {
    color: var(--err);
  }
  .name {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    font-weight: var(--fw-semibold);
  }
  .muted {
    flex: none;
    font-size: var(--fs-sm);
    color: var(--fg-2);
  }
  .grow {
    flex: 1;
  }
  .error {
    margin: var(--s-1) 0 0;
    font-size: var(--fs-sm);
    color: var(--err);
  }
  .digest {
    display: grid;
    grid-template-columns: 9ch 1fr auto;
    align-items: center;
    gap: var(--s-2);
    margin-top: var(--s-1);
  }
  .digest.want .alg {
    color: var(--accent);
  }
  .alg {
    font-size: var(--fs-sm);
    color: var(--fg-2);
  }
  code {
    min-width: 0;
    font-size: var(--fs-sm);
    overflow-wrap: anywhere;
    user-select: all;
  }
</style>
