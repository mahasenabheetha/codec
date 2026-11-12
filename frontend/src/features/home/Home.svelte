<script lang="ts">
  import ArrowRight from '@lucide/svelte/icons/arrow-right'
  import GitCompare from '@lucide/svelte/icons/git-compare'
  import FolderGit from '@lucide/svelte/icons/folder-git-2'
  import FolderOpen from '@lucide/svelte/icons/folder-open'
  import History from '@lucide/svelte/icons/history'
  import ShieldCheck from '@lucide/svelte/icons/shield-check'
  import FlaskConical from '@lucide/svelte/icons/flask-conical'
  import Star from '@lucide/svelte/icons/star'
  import Button from '../../lib/components/Button.svelte'
  import Kbd from '../../lib/components/Kbd.svelte'
  import SearchInput from '../../lib/components/SearchInput.svelte'
  import { layout } from '../../lib/stores/layout.svelte'
  import { usage } from '../../lib/stores/usage.svelte'
  import { entryByKey, hueVar, toolEntries, toolGroups, tools, viewHue, type ToolEntry } from '../../lib/tools'
  import { comparison } from '../compare/compare.svelte'
  import { openFolder, openSample, workspace as ws } from '../workspace/workspace.svelte'

  const groups = toolGroups()
  // Recent folders other than the open one.
  const recent = $derived((ws.info?.recent ?? []).filter((d) => d !== ws.info?.root).slice(0, 4))
  // First run: nothing open and nothing opened before.
  const firstRun = $derived(ws.info !== null && !ws.info.open && ws.info.recent.length === 0)
  let opening = $state(false)
  async function trySample() {
    opening = true
    await openSample()
    opening = false
  }

  // Find a tool by its name, what it does or a keyword; pinned ones and
  // the ones opened last come first (3.10.1).
  let query = $state('')
  const all = tools.flatMap(toolEntries)
  const words = $derived(query.toLowerCase().split(/\s+/).filter(Boolean))
  const found = $derived(
    words.length
      ? all.filter((t) => {
          const hay = (t.title + ' ' + t.description + ' ' + t.keywords.join(' ')).toLowerCase()
          return words.every((w) => hay.includes(w))
        })
      : [],
  )
  const known = (keys: string[]) => keys.map(entryByKey).filter((e): e is ToolEntry => !!e)
  const pinned = $derived(known(usage.pinned))
  const lately = $derived(known(usage.recent).filter((e) => !usage.isPinned(e.key)).slice(0, 4))
</script>

{#snippet toolCard(t: ToolEntry, small = false)}
  {@const on = usage.isPinned(t.key)}
  <div class="card tool" class:small class:pinned={on}>
    <button type="button" class="open" onclick={() => layout.openTool(t.tool, t.tab)}>
      <span class="icon hued" style:--tool={hueVar(t.hue)}><t.icon size={18} strokeWidth={1.75} /></span>
      <span class="text">
        <span class="title">{t.title}</span>
        {#if !small}<span class="desc">{t.description}</span>{/if}
      </span>
    </button>
    <button type="button" class="pin" aria-pressed={on} aria-label="{on ? 'Unpin' : 'Pin'} {t.title}" title={on ? 'Unpin from the top' : 'Pin to the top of Home and the rail'} onclick={() => usage.togglePin(t.key)}>
      <Star size={14} strokeWidth={1.75} fill={on ? 'currentColor' : 'none'} />
    </button>
  </div>
{/snippet}

<div class="home">
  <div class="inner">
    <header class="hero">
      <img src="/icon.svg" alt="" width="44" height="44" />
      <div>
        <h1>codec</h1>
        <p>A read-only workbench for your YAML, and tools for the text you handle all day.</p>
      </div>
    </header>

    <div class="find">
      <SearchInput bind:value={query} placeholder="Find a tool: base64, cron, diff, password…" label="Find a tool" />
      <p class="hint"><Kbd combo="Mod+K" /> searches everything · <Kbd combo="?" /> shortcuts</p>
    </div>

    {#if words.length}
      <h2>{found.length} {found.length === 1 ? 'tool' : 'tools'}</h2>
      {#if found.length}
        <div class="grid">
          {#each found as t (t.key)}{@render toolCard(t)}{/each}
        </div>
      {:else}
        <p class="none">No tool matches "{query}". <Kbd combo="Mod+K" /> also searches files, charts and actions.</p>
      {/if}
    {:else}

    {#if firstRun}
      <section class="welcome" aria-labelledby="welcome-title">
        <h2 id="welcome-title">Get started</h2>
        <ol>
          <li>
            <span class="step">1</span>
            <span class="text">
              <span class="title">Try the sample repository</span>
              <span class="desc">A small made-up service with a Helm chart, Kubernetes, Argo, CI, Compose and Ansible files. It opens on the chart, rendered.</span>
            </span>
            <Button variant="primary" icon={FlaskConical} disabled={opening} onclick={trySample}>Open the sample</Button>
          </li>
          <li>
            <span class="step">2</span>
            <span class="text">
              <span class="title">Open your own folder</span>
              <span class="desc">A repository on this machine. codec reads it and never writes to it.</span>
            </span>
            <Button icon={FolderOpen} tone="open" onclick={() => ws.chooseFolder()}>Open folder…</Button>
          </li>
        </ol>
      </section>
    {/if}

    {#if pinned.length}
      <h2>Pinned</h2>
      <div class="grid compact">
        {#each pinned as t (t.key)}{@render toolCard(t, true)}{/each}
      </div>
    {/if}
    {#if lately.length}
      <h2>Opened lately</h2>
      <div class="grid compact">
        {#each lately as t (t.key)}{@render toolCard(t, true)}{/each}
      </div>
    {/if}

    {#if !firstRun}
      <h2>Workspace</h2>
      <div class="grid">
        {#if ws.info?.open}
          <button type="button" class="card" onclick={() => (ws.quickOpen = true)}>
            <span class="icon"><FolderGit size={18} strokeWidth={1.75} /></span>
            <span class="text">
              <span class="title">{ws.info.name}</span>
              <span class="desc path">{ws.info.root}</span>
              <span class="desc">Go to a file with <Kbd combo="Mod+P" /></span>
            </span>
            <span class="go"><ArrowRight size={16} strokeWidth={1.75} /></span>
          </button>
        {/if}
        <button type="button" class="card" onclick={() => ws.chooseFolder()}>
          <span class="icon"><FolderOpen size={18} strokeWidth={1.75} /></span>
          <span class="text">
            <span class="title">{ws.info?.open ? 'Open another folder' : 'Open a folder'}</span>
            <span class="desc">Browse a repository's Kubernetes, Helm, Argo and CI YAML. codec only reads it.</span>
          </span>
          <span class="go"><ArrowRight size={16} strokeWidth={1.75} /></span>
        </button>
        {#if !ws.info?.sample}
          <button type="button" class="card" onclick={trySample} disabled={opening}>
            <span class="icon muted"><FlaskConical size={18} strokeWidth={1.75} /></span>
            <span class="text">
              <span class="title">Sample repository</span>
              <span class="desc">Every file type codec understands, in a made-up repo. Opens on a rendered Helm chart.</span>
            </span>
            <span class="go"><ArrowRight size={16} strokeWidth={1.75} /></span>
          </button>
        {/if}
        {#each recent as dir (dir)}
          <button type="button" class="card" onclick={() => openFolder(dir)}>
            <span class="icon muted"><History size={18} strokeWidth={1.75} /></span>
            <span class="text">
              <span class="title">{dir.split(/[\\/]/).filter(Boolean).pop()}</span>
              <span class="desc path">{dir}</span>
            </span>
            <span class="go"><ArrowRight size={16} strokeWidth={1.75} /></span>
          </button>
        {/each}
      </div>
    {/if}

    {#each groups as g, i (g.group)}
      <h2>{g.group}</h2>
      <div class="grid">
        {#if i === 0}
          <button type="button" class="card" onclick={() => comparison.pasteBoth()}>
            <span class="icon hued" style:--tool={hueVar(viewHue.compare)}><GitCompare size={18} strokeWidth={1.75} /></span>
            <span class="text">
              <span class="title">Compare text</span>
              <span class="desc">Paste two YAML, JSON or plain texts and see what differs. No folder needed.</span>
            </span>
            <span class="go"><ArrowRight size={16} strokeWidth={1.75} /></span>
          </button>
        {/if}
        {#each g.tools.flatMap(toolEntries) as t (t.key)}{@render toolCard(t)}{/each}
      </div>
    {/each}
    {/if}

    <p class="privacy"><ShieldCheck size={14} strokeWidth={2} /> Everything runs on this machine. Nothing you paste leaves it.</p>
  </div>
</div>

<style>
  .home {
    height: 100%;
    overflow-y: auto;
  }
  .inner {
    max-width: 1040px;
    margin: 0 auto;
    padding: var(--s-8) var(--s-6) var(--s-12);
  }
  .find {
    display: flex;
    align-items: center;
    flex-wrap: wrap;
    gap: var(--s-3) var(--s-4);
    margin-top: var(--s-6);
  }
  .find > :global(.search) {
    flex: 1 1 320px;
    max-width: 520px;
    height: calc(var(--control-h) + 4px);
  }
  .none {
    color: var(--fg-1);
  }
  .grid.compact {
    grid-template-columns: repeat(auto-fill, minmax(200px, 1fr));
  }
  /* A tool's card: the card opens it, the star pins it. */
  .card.tool {
    position: relative;
    padding: 0;
  }
  .card.tool:hover,
  .card.tool:focus-within {
    background: var(--bg-2);
    border-color: var(--border-strong);
    box-shadow: var(--shadow-raise);
  }
  .open {
    flex: 1;
    min-width: 0;
    display: flex;
    align-items: flex-start;
    gap: var(--s-3);
    padding: var(--s-4);
    padding-right: var(--s-8);
    text-align: left;
    color: inherit;
    background: none;
    border: none;
    border-radius: var(--r-lg);
  }
  .small .open {
    align-items: center;
    padding: var(--s-2) var(--s-8) var(--s-2) var(--s-3);
  }
  .small .title {
    font-size: var(--fs-md);
  }
  .pin {
    position: absolute;
    top: var(--s-2);
    right: var(--s-2);
    display: grid;
    place-items: center;
    width: 24px;
    height: 24px;
    padding: 0;
    color: var(--fg-2);
    background: none;
    border: none;
    border-radius: var(--r-sm);
    opacity: 0;
    transition: opacity var(--dur) var(--ease);
  }
  .small .pin {
    top: 50%;
    transform: translateY(-50%);
  }
  .card.tool:hover .pin,
  .pin:focus-visible,
  .pinned .pin {
    opacity: 1;
  }
  .pin:hover {
    color: var(--fg-0);
    background: var(--bg-3);
  }
  .pinned .pin {
    color: var(--warn);
  }
  .hero {
    display: flex;
    align-items: center;
    gap: var(--s-4);
  }
  h1 {
    font-size: var(--fs-3xl);
    font-weight: var(--fw-semibold);
    letter-spacing: -0.03em;
    line-height: 1.1;
  }
  .hero p {
    margin-top: var(--s-1);
    font-size: var(--fs-lg);
    color: var(--fg-1);
  }
  .hint {
    font-size: var(--fs-sm);
    color: var(--fg-2);
  }
  h2 {
    margin: var(--s-8) 0 var(--s-3);
    font-size: var(--fs-xs);
    font-weight: var(--fw-semibold);
    color: var(--fg-2);
    text-transform: uppercase;
    letter-spacing: 0.07em;
  }
  .grid {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(260px, 1fr));
    gap: var(--s-3);
  }
  .card {
    display: flex;
    align-items: flex-start;
    gap: var(--s-3);
    padding: var(--s-4);
    text-align: left;
    background: var(--bg-1);
    border: 1px solid var(--border);
    border-radius: var(--r-lg);
    transition:
      background var(--dur) var(--ease),
      border-color var(--dur) var(--ease);
  }
  button.card:hover {
    background: var(--bg-2);
    border-color: var(--border-strong);
  }
  .icon {
    flex: 0 0 auto;
    display: grid;
    place-items: center;
    width: 34px;
    height: 34px;
    color: var(--accent);
    background: var(--accent-soft);
    border-radius: var(--r-md);
  }
  .text {
    flex: 1;
    display: flex;
    flex-direction: column;
    gap: var(--s-1);
  }
  .title {
    font-size: var(--fs-lg);
    font-weight: var(--fw-medium);
  }
  .desc {
    font-size: var(--fs-sm);
    color: var(--fg-1);
  }
  .go {
    color: var(--fg-2);
    opacity: 0;
    transform: translateX(-4px);
    transition:
      opacity var(--dur) var(--ease),
      transform var(--dur) var(--ease);
  }
  button.card:focus-visible .go,
  button.card:hover .go {
    opacity: 1;
    transform: none;
  }
  .welcome {
    margin-top: var(--s-6);
    padding: var(--s-4);
    background: var(--bg-1);
    border: 1px solid var(--border-strong);
    border-radius: var(--r-lg);
  }
  .welcome h2 {
    margin: 0 0 var(--s-3);
  }
  ol {
    list-style: none;
    margin: 0;
    padding: 0;
    display: flex;
    flex-direction: column;
    gap: var(--s-4);
  }
  li {
    display: flex;
    align-items: center;
    gap: var(--s-3);
  }
  .step {
    flex: 0 0 auto;
    display: grid;
    place-items: center;
    width: 24px;
    height: 24px;
    font-size: var(--fs-sm);
    font-weight: var(--fw-semibold);
    color: var(--accent);
    background: var(--accent-soft);
    border-radius: 50%;
  }
  @media (max-width: 640px) {
    li {
      flex-wrap: wrap;
    }
  }
  .path {
    font-family: var(--font-mono);
    font-size: var(--fs-xs);
    word-break: break-all;
  }
  .icon.hued {
    color: var(--tool-accent);
    background: var(--tool-accent-soft);
  }
  .icon.muted {
    color: var(--fg-1);
    background: var(--bg-3);
  }
  .privacy {
    display: flex;
    align-items: center;
    gap: var(--s-2);
    margin-top: var(--s-12);
    font-size: var(--fs-sm);
    color: var(--fg-2);
  }
  .privacy :global(svg) {
    color: var(--ok);
  }
</style>
