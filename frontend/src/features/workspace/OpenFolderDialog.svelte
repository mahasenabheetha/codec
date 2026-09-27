<script lang="ts">
  import { untrack } from 'svelte'
  import ArrowUp from '@lucide/svelte/icons/arrow-up'
  import Clock from '@lucide/svelte/icons/clock'
  import Folder from '@lucide/svelte/icons/folder'
  import GitBranch from '@lucide/svelte/icons/git-branch'
  import HardDrive from '@lucide/svelte/icons/hard-drive'
  import House from '@lucide/svelte/icons/house'
  import Button from '../../lib/components/Button.svelte'
  import Dialog from '../../lib/components/Dialog.svelte'
  import { listDirs, type DirListing } from '../../lib/api/workspace'
  import { workspace as ws } from './workspace.svelte'

  // Pick a folder: type or paste a path, browse, or pick a recent one.
  let input = $state('')
  let listing = $state<DirListing | null>(null)
  let places = $state<{ home?: string; roots: string[] }>({ roots: [] })
  let error = $state('')
  let busy = $state(false)
  let inputEl = $state<HTMLInputElement>()

  // Every time the dialog opens: fetch places, then start next to the
  // open folder (in its parent, so sibling repos show) or at home.
  $effect(() => {
    if (!ws.dialogOpen) return
    const root = untrack(() => (ws.info?.open ? ws.info.root : undefined))
    error = ''
    start(root)
  })

  async function start(root: string | undefined) {
    try {
      const home = await listDirs()
      places = { home: home.home, roots: home.roots ?? [] }
      const here = root ? await listDirs(root) : null
      if (here?.parent) await browse(here.parent)
      else show(here ?? home)
      if (root) input = root
    } catch (e) {
      error = e instanceof Error ? e.message : String(e)
    }
    inputEl?.focus()
    inputEl?.select()
  }

  function show(l: DirListing) {
    listing = l
    input = l.path
    error = ''
  }

  async function browse(path: string) {
    try {
      show(await listDirs(path))
    } catch (e) {
      error = e instanceof Error ? e.message : String(e)
    }
  }

  async function open(path: string) {
    if (!path.trim() || busy) return
    busy = true
    error = ''
    try {
      await ws.open(path)
    } catch (e) {
      error = e instanceof Error ? e.message : String(e)
    } finally {
      busy = false
    }
  }

  function nameOf(p: string): string {
    return p.split(/[\\/]/).filter(Boolean).pop() ?? p
  }

  // Breadcrumb segments of the listed path, each with its full path.
  const crumbs = $derived.by(() => {
    if (!listing) return []
    const { path, sep } = listing
    const parts = path.split(sep).filter(Boolean)
    const absolute = path.startsWith(sep) // "/" on macOS/Linux
    return parts.map((name, i) => {
      let full = (absolute ? sep : '') + parts.slice(0, i + 1).join(sep)
      if (i === 0 && !absolute) full += sep // "C:" -> "C:\"
      return { name, path: full }
    })
  })
</script>

<Dialog bind:open={ws.dialogOpen} title="Open folder" description="codec reads the folder and never writes to it." width="760px">
  <form
    class="path"
    onsubmit={(e) => {
      e.preventDefault()
      open(input)
    }}
  >
    <input
      bind:this={inputEl}
      bind:value={input}
      spellcheck="false"
      autocomplete="off"
      placeholder="Paste or type a folder path"
      aria-label="Folder path"
    />
    <Button type="submit" variant="primary" disabled={busy || !input.trim()}>{busy ? 'Opening…' : 'Open'}</Button>
  </form>
  {#if error}<p class="error" role="alert">{error}</p>{/if}

  <div class="browser">
    <nav class="places" aria-label="Places">
      {#if places.home}
        <button type="button" class="place" onclick={() => browse(places.home!)}>
          <House size={14} strokeWidth={1.75} /> Home
        </button>
      {/if}
      {#each places.roots as r (r)}
        <button type="button" class="place" onclick={() => browse(r)}>
          <HardDrive size={14} strokeWidth={1.75} />
          {r}
        </button>
      {/each}
      {#if ws.info?.recent.length}
        <div class="section">Recent</div>
        {#each ws.info.recent as dir (dir)}
          <button type="button" class="place recent" title={dir} onclick={() => open(dir)}>
            <Clock size={14} strokeWidth={1.75} />
            <span>{nameOf(dir)}</span>
          </button>
        {/each}
      {/if}
    </nav>

    <div class="listing">
      {#if listing}
        <div class="crumbs">
          <button
            type="button"
            class="up"
            aria-label="Parent folder"
            disabled={!listing.parent}
            onclick={() => listing?.parent && browse(listing.parent)}
          >
            <ArrowUp size={14} strokeWidth={1.75} />
          </button>
          {#each crumbs as c, i (c.path)}
            {#if i > 0}<span class="sep">/</span>{/if}
            <button type="button" class="crumb" onclick={() => browse(c.path)}>{c.name}</button>
          {/each}
        </div>
        <ul class="dirs">
          {#each listing.dirs as d (d.path)}
            <li>
              <button type="button" class="dir" ondblclick={() => open(d.path)} onclick={() => browse(d.path)}>
                {#if d.repo}
                  <span class="repo-icon"><GitBranch size={14} strokeWidth={1.75} /></span>
                {:else}
                  <span class="dir-icon"><Folder size={14} strokeWidth={1.75} /></span>
                {/if}
                <span class="dir-name">{d.name}</span>
              </button>
              {#if d.repo}
                <button type="button" class="open-repo" onclick={() => open(d.path)}>Open</button>
              {/if}
            </li>
          {:else}
            <li class="empty">No subfolders</li>
          {/each}
        </ul>
      {/if}
    </div>
  </div>
  <p class="foot">Click a folder to look inside, double-click (or <b>Open</b> on a repository) to open it.</p>
</Dialog>

<style>
  .path {
    display: flex;
    gap: var(--s-2);
  }
  .path input {
    flex: 1;
    height: var(--control-h);
    padding: 0 var(--s-3);
    font-family: var(--font-mono);
    font-size: var(--fs-sm);
    color: var(--fg-0);
    background: var(--bg-0);
    border: 1px solid var(--border-strong);
    border-radius: var(--r-md);
    outline: none;
  }
  .path input:focus {
    border-color: var(--accent);
  }
  .error {
    margin-top: var(--s-2);
    font-size: var(--fs-sm);
    color: var(--err);
  }
  .browser {
    display: grid;
    grid-template-columns: 180px 1fr;
    height: min(380px, 48vh);
    margin-top: var(--s-3);
    border: 1px solid var(--border);
    border-radius: var(--r-md);
    overflow: hidden;
  }
  .places {
    display: flex;
    flex-direction: column;
    gap: 2px;
    padding: var(--s-2);
    background: var(--bg-0);
    border-right: 1px solid var(--border);
    overflow-y: auto;
  }
  .section {
    margin: var(--s-3) 0 var(--s-1);
    padding: 0 var(--s-2);
    font-size: var(--fs-xs);
    font-weight: var(--fw-semibold);
    color: var(--fg-2);
    text-transform: uppercase;
    letter-spacing: 0.07em;
  }
  .place,
  .dir {
    display: flex;
    align-items: center;
    gap: var(--s-2);
    width: 100%;
    min-height: 28px;
    padding: 0 var(--s-2);
    font-size: var(--fs-md);
    color: var(--fg-1);
    text-align: left;
    background: none;
    border: none;
    border-radius: var(--r-sm);
  }
  .place:hover,
  .dir:hover {
    color: var(--fg-0);
    background: var(--bg-3);
  }
  .place span,
  .dir-name {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .listing {
    display: flex;
    flex-direction: column;
    min-width: 0;
    background: var(--bg-2);
  }
  .crumbs {
    display: flex;
    align-items: center;
    flex-wrap: wrap;
    gap: 2px;
    padding: var(--s-2);
    border-bottom: 1px solid var(--border);
    font-size: var(--fs-sm);
  }
  .up {
    display: grid;
    place-items: center;
    width: var(--control-h-sm);
    height: var(--control-h-sm);
    margin-right: var(--s-1);
    padding: 0;
    color: var(--fg-1);
    background: none;
    border: none;
    border-radius: var(--r-sm);
  }
  .up:hover:not(:disabled) {
    color: var(--fg-0);
    background: var(--bg-3);
  }
  .up:disabled {
    opacity: 0.4;
  }
  .crumb {
    padding: 2px var(--s-1);
    color: var(--fg-1);
    background: none;
    border: none;
    border-radius: var(--r-sm);
  }
  .crumb:hover {
    color: var(--fg-0);
    background: var(--bg-3);
  }
  .crumb:last-of-type {
    color: var(--fg-0);
    font-weight: var(--fw-medium);
  }
  .sep {
    color: var(--fg-2);
  }
  .dirs {
    flex: 1;
    margin: 0;
    padding: var(--s-1);
    list-style: none;
    overflow-y: auto;
  }
  .dirs li {
    position: relative;
    display: flex;
    align-items: center;
  }
  .dir-icon {
    display: grid;
    color: var(--fg-2);
  }
  .repo-icon {
    display: grid;
    color: var(--accent);
  }
  .open-repo {
    position: absolute;
    right: var(--s-1);
    height: 22px;
    padding: 0 var(--s-2);
    font-size: var(--fs-sm);
    color: var(--accent);
    background: var(--accent-soft);
    border: none;
    border-radius: var(--r-sm);
    opacity: 0;
  }
  .dirs li:hover .open-repo,
  .open-repo:focus-visible {
    opacity: 1;
  }
  .empty {
    padding: var(--s-3);
    font-size: var(--fs-sm);
    color: var(--fg-2);
  }
  .foot {
    margin-top: var(--s-2);
    font-size: var(--fs-sm);
    color: var(--fg-2);
  }
</style>
