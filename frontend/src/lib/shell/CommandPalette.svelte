<script lang="ts">
  import { Command, Dialog } from 'bits-ui'
  import Search from '@lucide/svelte/icons/search'
  import Kbd from '../components/Kbd.svelte'
  import { commands, type Command as Cmd } from '../stores/commands.svelte'
  import { layout } from '../stores/layout.svelte'

  // Ctrl/⌘+K palette over every registered command. Filtering, ranking
  // and keyboard navigation come from Bits UI's Command component.
  let search = $state('')

  const groups = $derived.by(() => {
    const byGroup = new Map<string, Cmd[]>()
    for (const c of commands.list) {
      if (!byGroup.has(c.group)) byGroup.set(c.group, [])
      byGroup.get(c.group)!.push(c)
    }
    return [...byGroup]
  })

  // Land in the search box however the palette was opened. The dialog
  // content mounts a few frames after `open` flips (and after Bits UI's
  // own auto-focus has run), so look for the input until it appears.
  $effect(() => {
    if (!layout.paletteOpen) return
    // setTimeout rather than requestAnimationFrame: rAF never fires while
    // the window is hidden or not painting.
    let timer: ReturnType<typeof setTimeout>
    let tries = 0
    const attempt = () => {
      const el = document.querySelector<HTMLInputElement>('.codec-palette-input')
      if (el) el.focus()
      else if (tries++ < 30) timer = setTimeout(attempt, 16)
    }
    timer = setTimeout(attempt, 0)
    return () => clearTimeout(timer)
  })

  function run(cmd: Cmd) {
    layout.paletteOpen = false
    search = ''
    // Let the dialog close (and return focus) before the command acts.
    setTimeout(cmd.run, 0)
  }
</script>

<Dialog.Root bind:open={layout.paletteOpen} onOpenChange={(o) => !o && (search = '')}>
  <Dialog.Portal>
    <Dialog.Overlay class="codec-overlay" />
    <Dialog.Content
      class="codec-palette"
      aria-label="Command palette"
      onOpenAutoFocus={(e) => e.preventDefault()}
    >
      <Command.Root loop>
        <div class="search">
          <Search size={16} strokeWidth={1.75} />
          <Command.Input bind:value={search} placeholder="Search tools and actions…" class="codec-palette-input" />
        </div>
        <Command.List class="codec-palette-list">
          <Command.Viewport>
            <Command.Empty class="codec-palette-empty">No matching commands</Command.Empty>
            {#each groups as [group, cmds] (group)}
              <Command.Group>
                <Command.GroupHeading class="codec-palette-heading">{group}</Command.GroupHeading>
                <Command.GroupItems>
                  {#each cmds as cmd (cmd.id)}
                    <Command.Item
                      value={cmd.id}
                      keywords={[cmd.title, cmd.group, ...(cmd.keywords ?? [])]}
                      onSelect={() => run(cmd)}
                      class="codec-palette-item"
                    >
                      {#if cmd.icon}<cmd.icon size={15} strokeWidth={1.75} />{/if}
                      <span class="title">{cmd.title}</span>
                      {#if cmd.shortcut}<Kbd combo={cmd.shortcut} />{/if}
                    </Command.Item>
                  {/each}
                </Command.GroupItems>
              </Command.Group>
            {/each}
          </Command.Viewport>
        </Command.List>
      </Command.Root>
    </Dialog.Content>
  </Dialog.Portal>
</Dialog.Root>

<style>
  :global(.codec-palette) {
    position: fixed;
    top: 12vh;
    left: 50%;
    transform: translateX(-50%);
    z-index: var(--z-dialog);
    width: min(600px, calc(100vw - 32px));
    background: var(--bg-1);
    border-radius: var(--r-lg);
    box-shadow: var(--shadow-pop);
    overflow: hidden;
    animation: palette-in 140ms var(--ease);
  }
  .search {
    display: flex;
    align-items: center;
    gap: var(--s-3);
    padding: 0 var(--s-4);
    color: var(--fg-2);
    border-bottom: 1px solid var(--border);
  }
  :global(.codec-palette-input) {
    flex: 1;
    height: 48px;
    font-size: var(--fs-lg);
    color: var(--fg-0);
    background: none;
    border: none;
    outline: none;
  }
  :global(.codec-palette-input::placeholder) {
    color: var(--fg-2);
  }
  :global(.codec-palette-list) {
    max-height: min(420px, 60vh);
    overflow-y: auto;
    padding: var(--s-2);
  }
  :global(.codec-palette-heading) {
    padding: var(--s-2) var(--s-2) var(--s-1);
    font-size: var(--fs-xs);
    font-weight: var(--fw-semibold);
    color: var(--fg-2);
    text-transform: uppercase;
    letter-spacing: 0.07em;
  }
  :global(.codec-palette-item) {
    display: flex;
    align-items: center;
    gap: var(--s-3);
    height: 36px;
    padding: 0 var(--s-3);
    font-size: var(--fs-md);
    color: var(--fg-1);
    border-radius: var(--r-md);
    cursor: pointer;
  }
  :global(.codec-palette-item[data-selected]) {
    color: var(--fg-0);
    background: var(--bg-3);
  }
  :global(.codec-palette-item .title) {
    flex: 1;
  }
  :global(.codec-palette-empty) {
    padding: var(--s-6);
    text-align: center;
    color: var(--fg-2);
  }
  @keyframes palette-in {
    from {
      opacity: 0;
      transform: translate(-50%, -4px);
    }
  }
</style>
