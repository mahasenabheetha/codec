<script lang="ts">
  import Copy from '@lucide/svelte/icons/copy'
  import Plus from '@lucide/svelte/icons/plus'
  import X from '@lucide/svelte/icons/x'
  import Button from '../../lib/components/Button.svelte'
  import IconButton from '../../lib/components/IconButton.svelte'
  import SegmentedControl from '../../lib/components/SegmentedControl.svelte'
  import Skeleton from '../../lib/components/Skeleton.svelte'
  import Toggle from '../../lib/components/Toggle.svelte'
  import { forget, getSettings, type FolderID, type SettingsInfo } from '../../lib/api/settings'
  import { copyText } from '../../lib/utils/clipboard'
  import { toast } from '../../lib/stores/toast.svelte'
  import { theme, type ThemePref } from '../../lib/stores/theme.svelte'
  import { desktop } from '../../lib/platform'
  import { getDesktopSettings, setDesktopSettings, type DesktopSettings } from '../../lib/api/desktop'
  import { comparison } from '../compare/compare.svelte'
  import { editorNames, lensOpen, openIn, type ExternalEditor } from '../editor/active.svelte'
  import { lint } from '../lint/lint.svelte'
  import LintSections from './LintSections.svelte'
  import SettingRow from './SettingRow.svelte'
  import SettingSection from './SettingSection.svelte'

  // Every setting in one place. All of it lives in the user profile
  // (or, for display preferences, this browser), never in a repository.
  interface Props {
    active: boolean
  }

  let { active }: Props = $props()

  if (!lint.loaded) lint.load()
  comparison.loadIgnore()

  let info = $state.raw<SettingsInfo | null>(null)
  let error = $state('')
  let scroller = $state<HTMLElement>()

  async function load() {
    try {
      info = await getSettings()
      error = ''
    } catch (e) {
      error = e instanceof Error ? e.message : String(e)
    }
  }

  // Profiles are saved from the Helm view: fetch again on each visit.
  $effect(() => {
    if (active) load()
  })

  async function run(what: Parameters<typeof forget>[0], done: string) {
    try {
      info = await forget(what)
      toast(done, 'ok')
    } catch (e) {
      toast(e instanceof Error ? e.message : String(e), 'err')
    }
  }

  // Desktop-only choices; the section exists only in the desktop app.
  let desk = $state<DesktopSettings | null>(null)
  async function loadDesk() {
    if (!desktop) return
    try {
      desk = await getDesktopSettings()
    } catch (e) {
      toast(e instanceof Error ? e.message : String(e), 'err')
    }
  }
  async function saveDesk(next: DesktopSettings) {
    try {
      desk = await setDesktopSettings(next)
    } catch (e) {
      toast(e instanceof Error ? e.message : String(e), 'err')
    }
  }
  $effect(() => {
    if (active) loadDesk()
  })

  const nav = [
    { id: 'general', title: 'General' },
    ...(desktop ? [{ id: 'desktop', title: 'Desktop' }] : []),
    { id: 'kubernetes', title: 'Kubernetes' },
    { id: 'schemas', title: 'Schemas' },
    { id: 'rules-style', title: 'Lint rules' },
    { id: 'helm', title: 'Helm profiles' },
    { id: 'compare', title: 'Compare' },
    { id: 'folders', title: 'Folders' },
  ]

  function jump(id: string) {
    const el = scroller?.querySelector<HTMLElement>(`#settings-${id}`)
    // Instant, not smooth: moving focus cancels a smooth scroll in Chrome.
    el?.querySelector<HTMLElement>('h2')?.focus({ preventScroll: true })
    el?.scrollIntoView({ block: 'start' })
  }

  const themes: { value: ThemePref; label: string }[] = [
    { value: 'system', label: 'System' },
    { value: 'dark', label: 'Dark' },
    { value: 'light', label: 'Light' },
  ]
  const editors = (Object.keys(editorNames) as ExternalEditor[]).map((e) => ({ value: e, label: editorNames[e] }))

  const folderText: Record<FolderID, { title: string; description: string }> = {
    settings: { title: 'Settings file', description: 'Everything on this page except the display choices, which this browser keeps.' },
    templates: { title: 'Personal starters', description: 'Your own starters for New. Create the folder and add files; codec only reads it.' },
    schemas: { title: 'Schema cache', description: 'Downloaded schemas. Safe to delete; they are fetched again when needed.' },
    sample: { title: 'Sample workspace', description: 'The sample repository, rewritten each time you open it.' },
  }

  let pattern = $state('')
  function addPattern() {
    const p = pattern.trim()
    if (p) comparison.addIgnore(p)
    pattern = ''
  }

  const chartName = (p: string) => p.split(/[\\/]/).filter(Boolean).pop() ?? p
</script>

<div class="settings" class:inactive={!active}>
  <nav aria-label="Settings sections">
    <h1>Settings</h1>
    <ul>
      {#each nav as n (n.id)}
        <li><button type="button" onclick={() => jump(n.id)}>{n.title}</button></li>
      {/each}
    </ul>
  </nav>

  <div class="scroll" bind:this={scroller}>
    <div class="content">
      <p class="intro">Saved in your user profile, never in a repository. codec only reads the folders you open.</p>

      <SettingSection id="general" title="General">
        <SettingRow title="Theme" description="System follows your operating system's light or dark setting.">
          <SegmentedControl options={themes} bind:value={theme.pref} label="Theme" />
        </SettingRow>
        <SettingRow title="Open files in" description={'The editor "Open in" buttons and problem links launch.'}>
          <SegmentedControl options={editors} bind:value={openIn.value} label="Open files in" />
        </SettingRow>
        <SettingRow title="Outline and problems panel" description="Show the side panel in file tabs. Toggle it per tab from the editor toolbar too.">
          <Toggle checked={lensOpen.value} label={lensOpen.value ? 'Shown' : 'Hidden'} onchange={(on) => (lensOpen.value = on)} />
        </SettingRow>
      </SettingSection>

      {#if desktop && desk}
        <SettingSection id="desktop" title="Desktop">
          <SettingRow title="Start at login" description="Start codec in the tray when you sign in to Windows.">
            <Toggle checked={desk.startAtLogin} label={desk.startAtLogin ? 'On' : 'Off'} onchange={(on) => saveDesk({ ...desk!, startAtLogin: on })} />
          </SettingRow>
          <SettingRow title="Keep running in the tray" description="Closing the window leaves codec in the tray; quit from the tray menu. Off: closing the window quits.">
            <Toggle checked={desk.keepInTray} label={desk.keepInTray ? 'On' : 'Off'} onchange={(on) => saveDesk({ ...desk!, keepInTray: on })} />
          </SettingRow>
        </SettingSection>
      {/if}

      {#if lint.loaded}
        <LintSections />
      {:else if lint.error}
        <p class="error">{lint.error}</p>
      {:else}
        <div class="skeletons" aria-busy="true" aria-label="Loading lint settings">
          {#each [0, 1, 2, 3] as i (i)}<Skeleton height="44px" />{/each}
        </div>
      {/if}

      <SettingSection id="helm" title="Helm profiles" description="Saved from a chart's Helm view, by chart folder.">
        {#if !info}
          {#if error}<p class="error">{error}</p>{:else}<Skeleton height="44px" />{/if}
        {:else if info.helm.length === 0}
          <p class="none">No saved profiles yet. In a chart's Helm view, pick values files and save them as a profile.</p>
        {:else}
          {#each info.helm as c (c.chart)}
            <SettingRow title={chartName(c.chart)}>
              {#snippet details()}
                <span class="mono">{c.chart}</span>{#if !c.exists}<span class="gone"> · folder not found</span>{/if}
                <span class="chips">
                  {#each Object.keys(c.profiles).sort() as p (p)}
                    <span class="chip" class:current={p === c.active}>
                      {p}
                      <button
                        type="button"
                        aria-label="Forget profile {p} of {chartName(c.chart)}"
                        onclick={() => run({ helm: { chart: c.chart, profile: p } }, `Profile ${p} forgotten`)}><X size={12} strokeWidth={2} /></button
                      >
                    </span>
                  {/each}
                </span>
              {/snippet}
              <Button size="sm" variant="ghost" onclick={() => run({ helm: { chart: c.chart } }, `Profiles of ${chartName(c.chart)} forgotten`)}>Forget all</Button>
            </SettingRow>
          {/each}
        {/if}
      </SettingSection>

      <SettingSection id="compare" title="Compare" description="Paths semantic diffs leave out, such as checksums and generated names. * matches any characters, dots included.">
        {#each comparison.ignore as p (p)}
          <div class="pattern">
            <code>{p}</code>
            <IconButton icon={X} size="sm" label="Stop ignoring {p}" onclick={() => comparison.setIgnore(comparison.ignore.filter((x) => x !== p))} />
          </div>
        {:else}
          <p class="none">Nothing ignored. Add a path here or from a change in the Compare view.</p>
        {/each}
        <form class="add" onsubmit={(e) => (e.preventDefault(), addPattern())}>
          <input bind:value={pattern} placeholder="metadata.annotations.checksum/*" spellcheck="false" aria-label="Path to ignore" />
          <Button size="sm" icon={Plus} type="submit" disabled={!pattern.trim()}>Add</Button>
        </form>
      </SettingSection>

      <SettingSection id="folders" title="Folders">
        {#if info}
          {#each info.folders as f (f.id)}
            <SettingRow title={folderText[f.id].title} description={folderText[f.id].description}>
              {#snippet details()}
                <span class="mono">{f.path || 'not available on this system'}</span>{#if f.path && !f.exists}<span class="gone"> · not created yet</span>{/if}
              {/snippet}
              <IconButton icon={Copy} size="sm" label="Copy the path" disabled={!f.path} onclick={() => copyText(f.path, 'Path')} />
            </SettingRow>
          {/each}
          <SettingRow title="Recent folders" description={info.recent.length ? `${info.recent.length} remembered for Home and Open folder.` : 'None remembered.'}>
            <Button size="sm" variant="ghost" disabled={!info.recent.length} onclick={() => run({ recent: true }, 'Recent folders forgotten')}>Forget</Button>
          </SettingRow>
        {:else if !error}
          <Skeleton height="44px" />
        {/if}
      </SettingSection>
    </div>
  </div>
</div>

<style>
  .settings {
    height: 100%;
    display: grid;
    grid-template-columns: 200px minmax(0, 1fr);
    min-height: 0;
  }
  .inactive {
    display: none;
  }
  nav {
    padding: var(--s-6) var(--s-3) var(--s-6) var(--s-6);
    border-right: 1px solid var(--border);
  }
  h1 {
    margin: 0 0 var(--s-4);
    font-size: var(--fs-xl);
    font-weight: var(--fw-semibold);
  }
  ul {
    list-style: none;
    margin: 0;
    padding: 0;
  }
  nav button {
    width: 100%;
    padding: var(--s-1) var(--s-2);
    text-align: left;
    color: var(--fg-1);
    background: none;
    border: none;
    border-radius: var(--r-sm);
  }
  nav button:hover {
    color: var(--fg-0);
    background: var(--bg-2);
  }
  .scroll {
    overflow: auto;
  }
  .content {
    max-width: 820px;
    padding: var(--s-6) var(--s-6) var(--s-12);
  }
  .content :global(h2:focus) {
    outline: none;
  }
  .intro {
    margin: 0 0 var(--s-6);
    color: var(--fg-1);
  }
  .skeletons {
    display: flex;
    flex-direction: column;
    gap: var(--s-2);
    margin-top: var(--s-8);
  }
  .none {
    margin: 0;
    padding: var(--s-3) 0;
    border-top: 1px solid var(--border);
    font-size: var(--fs-sm);
    color: var(--fg-1);
  }
  .error {
    color: var(--err);
  }
  .mono,
  code {
    font-family: var(--font-mono);
    font-size: var(--fs-xs);
    word-break: break-all;
  }
  .gone {
    color: var(--warn);
  }
  .chips {
    display: flex;
    flex-wrap: wrap;
    gap: var(--s-1);
    margin-top: var(--s-1);
  }
  .chip {
    display: inline-flex;
    align-items: center;
    gap: 2px;
    padding: 0 2px 0 var(--s-2);
    font-size: var(--fs-xs);
    color: var(--fg-0);
    background: var(--bg-3);
    border: 1px solid var(--border);
    border-radius: 999px;
  }
  .chip.current {
    border-color: var(--accent);
  }
  .chip button {
    display: grid;
    place-items: center;
    width: 18px;
    height: 18px;
    color: var(--fg-1);
    background: none;
    border: none;
    border-radius: 50%;
  }
  .chip button:hover {
    color: var(--fg-0);
    background: var(--bg-2);
  }
  .pattern {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: var(--s-2);
    padding: var(--s-1) 0;
    border-top: 1px solid var(--border);
  }
  .add {
    display: flex;
    gap: var(--s-2);
    padding-top: var(--s-3);
    border-top: 1px solid var(--border);
  }
  .add input {
    flex: 1;
    max-width: 360px;
    height: var(--control-h-sm);
    padding: 0 var(--s-2);
    font-family: var(--font-mono);
    font-size: var(--fs-sm);
    color: var(--fg-0);
    background: var(--bg-0);
    border: 1px solid var(--border-strong);
    border-radius: var(--r-sm);
    outline: none;
  }
  .add input:focus-visible {
    border-color: var(--accent);
    box-shadow: 0 0 0 1px var(--accent);
  }
  @media (max-width: 720px) {
    .settings {
      grid-template-columns: 1fr;
      grid-template-rows: auto minmax(0, 1fr);
    }
    nav {
      padding: var(--s-3) var(--s-4) 0;
      border-right: none;
    }
    ul {
      display: flex;
      gap: var(--s-1);
      overflow-x: auto;
      padding-bottom: var(--s-2);
    }
    nav button {
      white-space: nowrap;
    }
    .content {
      padding: var(--s-4);
    }
  }
</style>
