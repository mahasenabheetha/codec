<script lang="ts">
  import FileSearch from '@lucide/svelte/icons/file-search'
  import { isAbort } from '../../lib/api/client'
  import { findTask, type TaskMatch } from '../../lib/api/ansible'
  import { layout } from '../../lib/stores/layout.svelte'
  import { editorNav } from '../editor/active.svelte'
  import { workspace } from '../workspace/workspace.svelte'

  // Links a task from a pasted log to where it is defined in the open
  // folder: the log's "task path" first, else the task name (with the
  // role prefix, and {{ variables }} matching any value). Only the name
  // and path are sent to codec's own server; the log is not.
  let { name, path }: { name?: string; path?: string } = $props()

  let matches = $state<TaskMatch[] | null>(null)
  let error = $state('')
  let more = $state(false)
  let ctrl: AbortController | null = null

  const open = $derived(workspace.info?.open ?? false)

  $effect(() => {
    const n = name ?? ''
    const p = path ?? ''
    matches = null
    error = ''
    more = false
    if (!open || (!n && !p)) return
    ctrl?.abort()
    const c = new AbortController()
    ctrl = c
    findTask({ name: n, path: p }, c.signal)
      .then((r) => {
        if (!c.signal.aborted) matches = r.matches
      })
      .catch((e) => {
        if (!isAbort(e)) error = e instanceof Error ? e.message : String(e)
      })
    return () => c.abort()
  })

  function go(m: TaskMatch) {
    layout.openFile(m.file)
    editorNav.request(m.file, m.line, 1)
  }
</script>

{#if open && (name || path)}
  <div class="def">
    <FileSearch size={14} strokeWidth={1.75} />
    {#if error}
      <span class="muted">Can't search the folder: {error}</span>
    {:else if matches === null}
      <span class="muted">Looking for its definition in the open folder…</span>
    {:else if matches.length === 0}
      <span class="muted">Not defined in the open folder{workspace.info?.name ? ` (${workspace.info.name})` : ''}.</span>
    {:else}
      <span>Defined in</span>
      <button type="button" class="link" onclick={() => go(matches![0])}>{matches[0].file}:{matches[0].line}</button>
      <span class="muted">{matches[0].why}</span>
      {#if matches.length > 1}
        <button type="button" class="link small" onclick={() => (more = !more)}>{more ? 'hide' : `${matches.length - 1} more`}</button>
      {/if}
    {/if}
  </div>
  {#if more && matches}
    <ul class="more">
      {#each matches.slice(1) as m, i (i)}
        <li><button type="button" class="link" onclick={() => go(m)}>{m.file}:{m.line}</button> <span class="muted">{m.name} · {m.why}</span></li>
      {/each}
    </ul>
  {/if}
{/if}

<style>
  .def {
    display: flex;
    align-items: center;
    flex-wrap: wrap;
    gap: var(--s-2);
    margin-top: calc(-1 * var(--s-2));
    font-size: var(--fs-sm);
    color: var(--fg-1);
  }
  .muted {
    color: var(--fg-2);
    font-size: var(--fs-xs);
  }
  .link {
    padding: 0;
    font-family: var(--font-mono);
    font-size: var(--fs-sm);
    color: var(--accent);
    background: none;
    border: none;
    text-align: left;
    word-break: break-all;
  }
  .link:hover {
    text-decoration: underline;
  }
  .link.small {
    font-family: var(--font-ui);
    font-size: var(--fs-xs);
  }
  .more {
    margin: 0;
    padding-left: var(--s-5);
    font-size: var(--fs-sm);
  }
</style>
