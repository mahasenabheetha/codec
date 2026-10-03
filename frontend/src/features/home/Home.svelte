<script lang="ts">
  import ArrowRight from '@lucide/svelte/icons/arrow-right'
  import GitCompare from '@lucide/svelte/icons/git-compare'
  import FolderGit from '@lucide/svelte/icons/folder-git-2'
  import FolderOpen from '@lucide/svelte/icons/folder-open'
  import History from '@lucide/svelte/icons/history'
  import ShieldCheck from '@lucide/svelte/icons/shield-check'
  import FlaskConical from '@lucide/svelte/icons/flask-conical'
  import Button from '../../lib/components/Button.svelte'
  import Kbd from '../../lib/components/Kbd.svelte'
  import { layout } from '../../lib/stores/layout.svelte'
  import { toolEntries, toolGroups } from '../../lib/tools'
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
</script>

<div class="home">
  <div class="inner">
    <header class="hero">
      <img src="/icon.svg" alt="" width="44" height="44" />
      <div>
        <h1>codec</h1>
        <p>A read-only workbench for your YAML, and tools for the text you handle all day.</p>
      </div>
    </header>

    <p class="hint">
      Press <Kbd combo="Mod+K" /> to search anything, or <Kbd combo="?" /> for shortcuts.
    </p>

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
            <Button icon={FolderOpen} onclick={() => (ws.dialogOpen = true)}>Open folder…</Button>
          </li>
        </ol>
      </section>
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
        <button type="button" class="card" onclick={() => (ws.dialogOpen = true)}>
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
            <span class="icon"><GitCompare size={18} strokeWidth={1.75} /></span>
            <span class="text">
              <span class="title">Compare text</span>
              <span class="desc">Paste two YAML, JSON or plain texts and see what differs. No folder needed.</span>
            </span>
            <span class="go"><ArrowRight size={16} strokeWidth={1.75} /></span>
          </button>
        {/if}
        {#each g.tools.flatMap(toolEntries) as t (t.key)}
          <button type="button" class="card" onclick={() => layout.openTool(t.tool, t.tab)}>
            <span class="icon"><t.icon size={18} strokeWidth={1.75} /></span>
            <span class="text">
              <span class="title">{t.title}</span>
              <span class="desc">{t.description}</span>
            </span>
            <span class="go"><ArrowRight size={16} strokeWidth={1.75} /></span>
          </button>
        {/each}
      </div>
    {/each}

    <p class="privacy"><ShieldCheck size={14} strokeWidth={2} /> Everything runs on this machine. Nothing you paste leaves it.</p>
  </div>
</div>

<style>
  .home {
    height: 100%;
    overflow-y: auto;
  }
  .inner {
    max-width: 880px;
    margin: 0 auto;
    padding: var(--s-12) var(--s-6);
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
    margin-top: var(--s-6);
    color: var(--fg-1);
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
