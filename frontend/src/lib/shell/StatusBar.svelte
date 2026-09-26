<script lang="ts">
  import ShieldCheck from '@lucide/svelte/icons/shield-check'
  import ExternalLink from '@lucide/svelte/icons/external-link'
  import FolderGit from '@lucide/svelte/icons/folder-git-2'
  import Unplug from '@lucide/svelte/icons/unplug'
  import Popover from '../components/Popover.svelte'
  import FileIcon from '../../features/workspace/FileIcon.svelte'
  import { lookOf } from '../../features/workspace/filetypes'
  import { workspace } from '../../features/workspace/workspace.svelte'
  import { getVersion, type VersionInfo } from '../api/version'
  import { layout } from '../stores/layout.svelte'
  import { router } from '../stores/router.svelte'
  import { toolById } from '../tools'

  let version = $state<VersionInfo | null>(null)
  getVersion()
    .then((v) => (version = v))
    .catch(() => (version = null))

  const tool = $derived(toolById(router.toolId))
  const file = $derived(router.filePath)
  const entry = $derived(file ? workspace.byPath.get(file) : undefined)

  const watchText: Record<string, string> = {
    '': 'Starting to watch for changes…',
    native: 'Watching for changes (native events).',
    poll: 'Watching for changes by polling every second.',
  }
</script>

<footer class="statusbar">
  <div class="left">
    {#if tool}
      <span class="where">{tool.title}</span>
      {#if layout.status.text}
        <span class="msg {layout.status.tone}"><span class="dot"></span>{layout.status.text}</span>
      {/if}
    {:else if file}
      {#if entry}
        <span class="where type"><FileIcon file={entry} size={12} /> {lookOf(entry).title}</span>
      {/if}
      <span class="path">{file}</span>
    {:else}
      <span class="where">Home</span>
    {/if}
  </div>

  <div class="right">
    {#if workspace.disconnected}
      <button type="button" class="warn-btn" onclick={() => location.reload()} title="codec was restarted or stopped">
        <Unplug size={12} strokeWidth={2} /> Disconnected — reload
      </button>
    {/if}
    {#if workspace.info?.open}
      <button
        type="button"
        class="ws"
        title={`${workspace.info.root}\n${watchText[workspace.info.watch ?? ''] ?? watchText['']}\nClick to open another folder.`}
        onclick={() => (workspace.dialogOpen = true)}
      >
        <FolderGit size={12} strokeWidth={2} />
        {workspace.info.name}
        <span class="watch {workspace.info.watch ?? ''}"></span>
      </button>
    {/if}
    <span class="local" title="codec runs entirely on this machine; nothing is sent anywhere">
      <ShieldCheck size={12} strokeWidth={2} /> Local only
    </span>
    <Popover>
      {#snippet trigger(props)}
        <button {...props} type="button" class="version">{version?.version ?? '—'}</button>
      {/snippet}
      <dl class="about">
        <dt>Version</dt>
        <dd>{version?.version ?? 'unknown'}</dd>
        {#if version?.commit}
          <dt>Commit</dt>
          <dd><code>{version.commit.slice(0, 12)}{version.dirty ? ' (modified)' : ''}</code></dd>
        {/if}
        {#if version?.date}
          <dt>Built</dt>
          <dd>{new Date(version.date).toLocaleString()}</dd>
        {/if}
      </dl>
      <a class="repo" href="https://github.com/mahasenabheetha/codec" target="_blank" rel="noopener">
        Source on GitHub <ExternalLink size={12} strokeWidth={2} />
      </a>
    </Popover>
  </div>
</footer>

<style>
  .type {
    display: inline-flex;
    align-items: center;
    gap: var(--s-1);
  }
  .path {
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    font-family: var(--font-mono);
    font-size: var(--fs-xs);
  }
  .ws,
  .warn-btn {
    display: inline-flex;
    align-items: center;
    gap: var(--s-1);
    height: 20px;
    padding: 0 var(--s-2);
    font-size: var(--fs-sm);
    color: var(--fg-1);
    background: none;
    border: none;
    border-radius: var(--r-sm);
    white-space: nowrap;
  }
  .ws:hover {
    color: var(--fg-0);
    background: var(--bg-3);
  }
  .warn-btn {
    color: var(--warn);
    background: var(--warn-soft);
  }
  /* Live-watch indicator: grey while starting, green for native
     events, amber when polling. */
  .watch {
    width: 6px;
    height: 6px;
    border-radius: 50%;
    background: var(--fg-2);
  }
  .watch.native {
    background: var(--ok);
  }
  .watch.poll {
    background: var(--warn);
  }
  .statusbar {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: var(--s-4);
    height: var(--statusbar-h);
    padding: 0 var(--s-3);
    font-size: var(--fs-sm);
    color: var(--fg-1);
    background: var(--bg-0);
    border-top: 1px solid var(--border);
  }
  .left,
  .right {
    display: flex;
    align-items: center;
    gap: var(--s-4);
    min-width: 0;
  }
  .where {
    color: var(--fg-0);
    font-weight: var(--fw-medium);
    white-space: nowrap;
  }
  .msg {
    display: flex;
    align-items: center;
    gap: var(--s-2);
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .dot {
    flex: 0 0 auto;
    width: 6px;
    height: 6px;
    border-radius: 50%;
    background: var(--fg-2);
  }
  .ok .dot {
    background: var(--ok);
  }
  .warn .dot {
    background: var(--warn);
  }
  .err .dot {
    background: var(--err);
  }
  .err {
    color: var(--err);
  }
  .local {
    display: flex;
    align-items: center;
    gap: var(--s-1);
    white-space: nowrap;
  }
  .local :global(svg) {
    color: var(--ok);
  }
  .version {
    height: 20px;
    padding: 0 var(--s-2);
    font-family: var(--font-mono);
    font-size: var(--fs-xs);
    color: var(--fg-1);
    background: none;
    border: none;
    border-radius: var(--r-sm);
  }
  .version:hover {
    color: var(--fg-0);
    background: var(--bg-3);
  }
  .about {
    display: grid;
    grid-template-columns: auto 1fr;
    gap: var(--s-1) var(--s-4);
    margin: 0 0 var(--s-3);
    font-size: var(--fs-sm);
  }
  dt {
    color: var(--fg-2);
  }
  dd {
    margin: 0;
  }
  .repo {
    display: inline-flex;
    align-items: center;
    gap: var(--s-1);
    font-size: var(--fs-sm);
  }
  @media (max-width: 640px) {
    .local {
      display: none;
    }
  }
</style>
