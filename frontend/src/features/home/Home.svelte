<script lang="ts">
  import ArrowRight from '@lucide/svelte/icons/arrow-right'
  import FolderOpen from '@lucide/svelte/icons/folder-open'
  import ShieldCheck from '@lucide/svelte/icons/shield-check'
  import Kbd from '../../lib/components/Kbd.svelte'
  import { layout } from '../../lib/stores/layout.svelte'
  import { toolGroups } from '../../lib/tools'

  const groups = toolGroups()
</script>

<div class="home">
  <div class="inner">
    <header class="hero">
      <img src="/favicon.svg" alt="" width="44" height="44" />
      <div>
        <h1>codec</h1>
        <p>Local developer tools for the text you handle all day.</p>
      </div>
    </header>

    <p class="hint">
      Press <Kbd combo="Mod+K" /> to search anything, or <Kbd combo="?" /> for shortcuts.
    </p>

    {#each groups as g (g.group)}
      <h2>{g.group}</h2>
      <div class="grid">
        {#each g.tools as t (t.id)}
          <button type="button" class="card" onclick={() => layout.openTool(t.id)}>
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

    <h2>Coming in codec 2.0</h2>
    <div class="grid">
      <div class="card soon" aria-disabled="true">
        <span class="icon"><FolderOpen size={18} strokeWidth={1.75} /></span>
        <span class="text">
          <span class="title">YAML workbench</span>
          <span class="desc">Open a repo folder to read, lint and render Kubernetes, Helm and Argo YAML — read-only.</span>
        </span>
      </div>
    </div>

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
  button.card:hover .go {
    opacity: 1;
    transform: none;
  }
  .soon {
    opacity: 0.6;
  }
  .soon .icon {
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
