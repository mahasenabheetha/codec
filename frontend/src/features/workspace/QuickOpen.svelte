<script lang="ts">
  import { Command, Dialog } from 'bits-ui'
  import Search from '@lucide/svelte/icons/search'
  import { layout } from '../../lib/stores/layout.svelte'
  import FileIcon from './FileIcon.svelte'
  import { rank, type Match } from './fuzzy'
  import { workspace as ws } from './workspace.svelte'

  // Go to file (Ctrl+P): fuzzy search over every file in the workspace.
  // Ranking is ours (fuzzy.ts); Bits UI's Command only provides the
  // keyboard navigation, so its own filter is off.
  let query = $state('')

  const paths = $derived(ws.files.map((f) => f.path))
  const results = $derived.by<Match[]>(() => {
    if (query.trim()) return rank(query, paths)
    // No query yet: recently opened files, then the rest in order.
    const recent = ws.recentFiles.filter((p) => ws.byPath.has(p))
    const rest = paths.filter((p) => !recent.includes(p))
    return [...recent, ...rest].slice(0, 60).map((path) => ({ path, score: 0, positions: [] }))
  })

  // Same focus dance as the command palette: Bits UI's auto-focus runs
  // before the input exists.
  $effect(() => {
    if (!ws.quickOpen) return
    let tries = 0
    let timer: ReturnType<typeof setTimeout>
    const attempt = () => {
      const el = document.querySelector<HTMLInputElement>('.codec-quickopen-input')
      if (el) el.focus()
      else if (tries++ < 30) timer = setTimeout(attempt, 16)
    }
    timer = setTimeout(attempt, 0)
    return () => clearTimeout(timer)
  })

  function pick(path: string) {
    ws.quickOpen = false
    query = ''
    ws.reveal(path)
    layout.openFile(path)
  }

  // Split a path into highlighted and plain runs for display.
  function parts(m: Match, from: number, to: number) {
    const hit = new Set(m.positions)
    const out: { text: string; hit: boolean }[] = []
    for (let i = from; i < to; i++) {
      const h = hit.has(i)
      const last = out[out.length - 1]
      if (last && last.hit === h) last.text += m.path[i]
      else out.push({ text: m.path[i], hit: h })
    }
    return out
  }
</script>

<Dialog.Root bind:open={ws.quickOpen} onOpenChange={(o) => !o && (query = '')}>
  <Dialog.Portal>
    <Dialog.Overlay class="codec-overlay" />
    <Dialog.Content class="codec-palette" aria-label="Go to file" onOpenAutoFocus={(e) => e.preventDefault()}>
      <Command.Root shouldFilter={false} loop>
        <div class="codec-palette-search">
          <Search size={16} strokeWidth={1.75} />
          <Command.Input
            bind:value={query}
            placeholder={ws.info?.open ? `Search files in ${ws.info.name}…` : 'Open a folder first'}
            class="codec-palette-input codec-quickopen-input"
          />
        </div>
        <Command.List class="codec-palette-list">
          <Command.Viewport>
            <Command.Empty class="codec-palette-empty">
              {ws.info?.open ? 'No matching files' : 'No folder is open'}
            </Command.Empty>
            {#each results as m (m.path)}
              {@const nameAt = m.path.lastIndexOf('/') + 1}
              {@const file = ws.byPath.get(m.path)}
              <Command.Item value={m.path} onSelect={() => pick(m.path)} class="codec-palette-item">
                {#if file}<FileIcon {file} size={15} />{/if}
                <span class="title">
                  <span class="name">
                    {#each parts(m, nameAt, m.path.length) as p, i (i)}{#if p.hit}<mark>{p.text}</mark>{:else}{p.text}{/if}{/each}
                  </span>
                  {#if nameAt > 0}
                    <span class="dir">
                      {#each parts(m, 0, nameAt - 1) as p, i (i)}{#if p.hit}<mark>{p.text}</mark>{:else}{p.text}{/if}{/each}
                    </span>
                  {/if}
                </span>
              </Command.Item>
            {/each}
          </Command.Viewport>
        </Command.List>
      </Command.Root>
    </Dialog.Content>
  </Dialog.Portal>
</Dialog.Root>

<style>
  .title {
    display: flex;
    align-items: baseline;
    gap: var(--s-2);
    overflow: hidden;
    white-space: nowrap;
  }
  .name {
    color: var(--fg-0);
  }
  .dir {
    overflow: hidden;
    text-overflow: ellipsis;
    font-size: var(--fs-sm);
    color: var(--fg-2);
  }
  mark {
    color: var(--accent);
    background: none;
    font-weight: var(--fw-semibold);
  }
</style>
