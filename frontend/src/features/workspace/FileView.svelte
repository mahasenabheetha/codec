<script lang="ts">
  import { untrack } from 'svelte'
  import ChevronRight from '@lucide/svelte/icons/chevron-right'
  import Copy from '@lucide/svelte/icons/copy'
  import FileWarning from '@lucide/svelte/icons/file-warning'
  import Link from '@lucide/svelte/icons/link'
  import LocateFixed from '@lucide/svelte/icons/locate-fixed'
  import RefreshCw from '@lucide/svelte/icons/refresh-cw'
  import Trash from '@lucide/svelte/icons/trash-2'
  import Badge from '../../lib/components/Badge.svelte'
  import CodeView from '../../lib/components/CodeView.svelte'
  import EmptyState from '../../lib/components/EmptyState.svelte'
  import IconButton from '../../lib/components/IconButton.svelte'
  import { ApiError } from '../../lib/api/client'
  import { getContent, type FileContent } from '../../lib/api/workspace'
  import { layout } from '../../lib/stores/layout.svelte'
  import { copyText } from '../../lib/utils/clipboard'
  import FileIcon from './FileIcon.svelte'
  import { editorLang, isSpecific, kindOf, lookOf } from './filetypes'
  import { workspace as ws } from './workspace.svelte'

  // One workspace file, read-only. Reloads by itself when the file
  // changes on disk; edits (what-if) arrive in phase 04.
  interface Props {
    path: string
    active: boolean
  }

  let { path, active }: Props = $props()

  let content = $state<FileContent | null>(null)
  let error = $state<ApiError | Error | null>(null)
  let updated = $state(false) // brief "updated from disk" hint
  let updatedTimer: ReturnType<typeof setTimeout> | undefined

  const version = $derived(ws.versions.get(path) ?? 0)
  const deleted = $derived(ws.deleted.has(path))
  // CodeMirror stores lines without their break characters; hand it LF
  // text and show the file's real line endings in the header instead.
  const text = $derived(content ? content.text.replace(/\r\n?/g, '\n') : '')
  const lines = $derived(text ? text.split('\n').length - (text.endsWith('\n') ? 1 : 0) : 0)
  const kind = $derived(content ? kindOf(content) : '')
  const segments = $derived(path.split('/'))

  async function load(fromDisk: boolean) {
    try {
      const c = await getContent(path)
      const changed = content !== null && c.text !== content.text
      content = c
      error = null
      if (fromDisk && changed) flashUpdated()
    } catch (e) {
      error = e instanceof Error ? e : new Error(String(e))
    }
  }

  function flashUpdated() {
    updated = true
    clearTimeout(updatedTimer)
    updatedTimer = setTimeout(() => (updated = false), 2500)
  }

  // Load on open, and again whenever the watcher reports a change.
  // Background tabs reload too, so switching to them is instant.
  let seen = -1
  $effect(() => {
    const v = version
    if (deleted) return
    const fromDisk = seen >= 0
    seen = v
    untrack(() => load(fromDisk))
  })

  $effect(() => {
    if (active) {
      ws.noteOpened(path)
      untrack(() => ws.reveal(path))
    }
  })

  function copyPath(absolute: boolean) {
    if (!content || !ws.info?.root) return
    const sep = ws.info.sep
    copyText(absolute ? ws.info.root + sep + path.split('/').join(sep) : path, absolute ? 'Full path' : 'Path')
  }

  function formatSize(n: number): string {
    if (n < 1024) return `${n} B`
    if (n < 1024 * 1024) return `${(n / 1024).toFixed(1)} KB`
    return `${(n / 1024 / 1024).toFixed(1)} MB`
  }
</script>

<div class="fileview">
  <header>
    <nav class="crumbs" aria-label="File path">
      {#each segments as seg, i (i)}
        {#if i > 0}<ChevronRight size={12} strokeWidth={1.75} />{/if}
        {#if i === segments.length - 1}
          {#if content}<FileIcon file={content} />{/if}
          <span class="file">{seg}</span>
        {:else}
          <span class="dir">{seg}</span>
        {/if}
      {/each}
    </nav>
    {#if content && isSpecific(kind)}
      <Badge>{lookOf(kind).title}</Badge>
    {/if}
    {#if updated}<span class="updated" role="status"><RefreshCw size={12} strokeWidth={2} /> Updated from disk</span>{/if}
    <div class="spacer"></div>
    {#if content}
      <span class="meta">
        {lines} lines · {formatSize(content.size)}{content.crlf ? ' · CRLF' : ''}{content.bom ? ' · BOM' : ''}
      </span>
    {/if}
    <div class="actions">
      <IconButton icon={LocateFixed} label="Reveal in explorer" size="sm" onclick={() => { layout.explorerOpen = true; ws.reveal(path) }} />
      <IconButton icon={Link} label="Copy path" size="sm" onclick={() => copyPath(false)} disabled={!content} />
      <IconButton icon={Copy} label="Copy full path" size="sm" onclick={() => copyPath(true)} disabled={!content} />
    </div>
  </header>

  {#if deleted}
    <div class="banner"><Trash size={14} strokeWidth={1.75} /> This file was deleted on disk. What you see is the last version codec read.</div>
  {/if}

  <div class="body">
    {#if error && !content}
      <EmptyState icon={FileWarning} title="Can't show this file" description={error.message}>
        <IconButton icon={RefreshCw} label="Try again" onclick={() => load(false)} />
      </EmptyState>
    {:else if content}
      <CodeView value={text} label={path} language={editorLang(content.lang)} readonly />
    {/if}
  </div>
</div>

<style>
  .fileview {
    height: 100%;
    display: flex;
    flex-direction: column;
    min-width: 0;
  }
  header {
    display: flex;
    align-items: center;
    gap: var(--s-3);
    height: 36px;
    padding: 0 var(--s-2) 0 var(--s-4);
    border-bottom: 1px solid var(--border);
    background: var(--bg-1);
    flex: 0 0 auto;
    min-width: 0;
  }
  .crumbs {
    display: flex;
    align-items: center;
    gap: var(--s-1);
    min-width: 0;
    overflow: hidden;
    white-space: nowrap;
    font-size: var(--fs-md);
    color: var(--fg-2);
  }
  .dir {
    color: var(--fg-1);
  }
  .file {
    color: var(--fg-0);
    font-weight: var(--fw-medium);
  }
  .updated {
    display: inline-flex;
    align-items: center;
    gap: var(--s-1);
    font-size: var(--fs-sm);
    color: var(--info);
    white-space: nowrap;
    animation: fade-in var(--dur) var(--ease);
  }
  @keyframes fade-in {
    from {
      opacity: 0;
    }
  }
  .spacer {
    flex: 1;
  }
  .meta {
    font-size: var(--fs-sm);
    color: var(--fg-2);
    white-space: nowrap;
    font-variant-numeric: tabular-nums;
  }
  .actions {
    display: flex;
  }
  .banner {
    display: flex;
    align-items: center;
    gap: var(--s-2);
    padding: var(--s-2) var(--s-4);
    font-size: var(--fs-sm);
    color: var(--warn);
    background: var(--warn-soft);
    border-bottom: 1px solid var(--border);
  }
  .body {
    flex: 1;
    min-height: 0;
    display: flex;
    flex-direction: column;
  }
  @media (max-width: 720px) {
    .meta {
      display: none;
    }
  }
</style>
