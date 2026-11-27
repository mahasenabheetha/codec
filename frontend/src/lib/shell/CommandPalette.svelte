<script lang="ts">
  import { Command, Dialog, computeCommandScore } from 'bits-ui'
  import type { Component } from 'svelte'
  import ArrowRight from '@lucide/svelte/icons/arrow-right'
  import Search from '@lucide/svelte/icons/search'
  import Kbd from '../components/Kbd.svelte'
  import Spectrum from '../components/Spectrum.svelte'
  import Tile from '../components/Tile.svelte'
  import { commands, type Command as Cmd } from '../stores/commands.svelte'
  import { layout } from '../stores/layout.svelte'
  import { usage } from '../stores/usage.svelte'
  import { hueVar, tools, viewHue, viewIcon, type Hue, type ToolDef } from '../tools'
  import { isMac } from '../utils/platform'

  // Ctrl/⌘+K palette over every registered command. Filtering, ranking
  // and keyboard navigation come from Bits UI's Command component.
  let search = $state('')
  // The highlighted row's value (Bits UI keeps it current).
  let highlighted = $state('')

  const groups = $derived.by(() => {
    const byGroup = new Map<string, Cmd[]>()
    for (const c of commands.list) {
      if (!byGroup.has(c.group)) byGroup.set(c.group, [])
      byGroup.get(c.group)!.push(c)
    }
    return [...byGroup]
  })

  // The commands run last, first in the list while nothing is typed
  // (3.10.1); one that is no longer registered (a closed tool's, a file
  // of another folder) is left out.
  const recent = $derived(search ? [] : usage.commands.map((id) => commands.list.find((c) => c.id === id)).filter((c): c is Cmd => !!c))

  const keywordsOf = (c: Cmd) => [c.title, c.group, ...(c.keywords ?? [])]
  // Counted as Bits UI filters: a row shows when it scores above 0.
  const count = $derived(
    search ? commands.list.filter((c) => computeCommandScore(c.id, search, keywordsOf(c)) > 0).length : recent.length + commands.list.length,
  )
  const current = $derived(commands.list.find((c) => c.id === highlighted || 'recent:' + c.id === highlighted))

  // Groups named after a view: its colour and icon for the commands in it.
  const viewGroups: Record<string, keyof typeof viewHue> = {
    Problems: 'problems',
    Compare: 'compare',
    File: 'explorer',
    Helm: 'helm',
    Kubernetes: 'k8s',
    Kustomize: 'kustomize',
    Argo: 'argo',
    CI: 'ci',
    Compose: 'compose',
    Ansible: 'ansible',
  }

  // A tool's commands are grouped under its name, a sub-tab's (Cron,
  // Hash) or its rail group's when it is alone there (Wellness).
  function toolOfGroup(g: string): ToolDef | undefined {
    const t = tools.find((t) => t.title === g || t.tabs?.some((s) => s.title === g))
    if (t) return t
    const alone = tools.filter((t) => t.group === g)
    return alone.length === 1 ? alone[0] : undefined
  }

  /** A row's colour and icon: the command's own, else its group's; the
   *  accent for the rest (Settings, View, Navigation). */
  function look(c: Cmd): { hue?: Hue; icon: Component } {
    const v = viewGroups[c.group]
    const t = v ? undefined : toolOfGroup(c.group)
    return { hue: c.hue ?? (v ? viewHue[v] : t?.hue), icon: c.icon ?? (v ? viewIcon[v] : t?.icon) ?? ArrowRight }
  }

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

  // Bits UI runs the highlighted row on Enter with or without Ctrl, after
  // the input has seen the key: note which it was.
  let newTab = false
  function onkeydown(e: KeyboardEvent) {
    newTab = e.key === 'Enter' && (isMac ? e.metaKey : e.ctrlKey)
  }

  function run(cmd: Cmd) {
    const go = newTab && cmd.runNew ? cmd.runNew : cmd.run
    newTab = false
    layout.paletteOpen = false
    search = ''
    usage.ran(cmd.id)
    // Let the dialog close (and return focus) before the command acts.
    setTimeout(go, 0)
  }
</script>

{#snippet row(cmd: Cmd, value: string, keywords: string[], hint: string | undefined)}
  {@const l = look(cmd)}
  <Command.Item
    {value}
    {keywords}
    onSelect={() => run(cmd)}
    class="codec-palette-item tiled{l.hue ? ' hued' : ''}"
    style={l.hue ? `--tool: ${hueVar(l.hue)}` : undefined}
  >
    <Tile icon={l.icon} solid={highlighted === value} />
    <span class="text">
      <span class="title">{cmd.title}</span>
      {#if hint}<span class="hint">{hint}</span>{/if}
    </span>
    {#if cmd.shortcut}<Kbd combo={cmd.shortcut} />{/if}
  </Command.Item>
{/snippet}

<Dialog.Root bind:open={layout.paletteOpen} onOpenChange={(o) => !o && (search = '')}>
  <Dialog.Portal>
    <Dialog.Overlay class="codec-overlay" />
    <Dialog.Content
      class="codec-palette"
      aria-label="Command palette"
      onOpenAutoFocus={(e) => e.preventDefault()}
    >
      <Spectrum flush />
      <Command.Root loop bind:value={highlighted}>
        <div class="codec-palette-search">
          <Search size={18} />
          <Command.Input bind:value={search} placeholder="Search tools and actions…" class="codec-palette-input" {onkeydown} />
          <Kbd combo="Escape" />
        </div>
        <Command.List class="codec-palette-list">
          <Command.Viewport>
            <Command.Empty class="codec-palette-empty">No matching commands</Command.Empty>
            {#if recent.length}
              <Command.Group>
                <Command.GroupHeading class="codec-palette-heading">Recent</Command.GroupHeading>
                <Command.GroupItems>
                  {#each recent as cmd (cmd.id)}
                    {@render row(cmd, 'recent:' + cmd.id, [cmd.title], cmd.hint ? `${cmd.group} · ${cmd.hint}` : cmd.group)}
                  {/each}
                </Command.GroupItems>
              </Command.Group>
            {/if}
            {#each groups as [group, cmds] (group)}
              <Command.Group>
                <Command.GroupHeading class="codec-palette-heading">{group}</Command.GroupHeading>
                <Command.GroupItems>
                  {#each cmds as cmd (cmd.id)}
                    {@render row(cmd, cmd.id, keywordsOf(cmd), cmd.hint)}
                  {/each}
                </Command.GroupItems>
              </Command.Group>
            {/each}
          </Command.Viewport>
        </Command.List>
      </Command.Root>
      <div class="foot">
        <span class="key"><Kbd combo="↑" /><Kbd combo="↓" /> move</span>
        <span class="key"><Kbd combo="Enter" /> open</span>
        {#if current?.runNew}<span class="key"><Kbd combo="Mod+Enter" /> in a new tab</span>{/if}
        <span class="count">{count} {count === 1 ? 'result' : 'results'}</span>
      </div>
    </Dialog.Content>
  </Dialog.Portal>
</Dialog.Root>

<style>
  .text {
    flex: 1;
    min-width: 0;
    display: flex;
    flex-direction: column;
  }
  .title {
    font-weight: var(--fw-medium);
  }
  .hint {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    font-size: var(--fs-xs);
    color: var(--fg-2);
  }
  .foot {
    display: flex;
    align-items: center;
    gap: var(--s-3);
    height: calc(var(--control-h) + 6px);
    padding: 0 var(--s-4);
    font-size: var(--fs-xs);
    color: var(--fg-2);
    background: var(--bg-1);
    border-top: 1px solid var(--border);
  }
  .key {
    display: inline-flex;
    align-items: center;
    gap: var(--s-1);
  }
  .count {
    margin-left: auto;
  }
</style>
