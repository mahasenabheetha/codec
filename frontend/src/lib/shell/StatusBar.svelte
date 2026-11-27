<script lang="ts">
  import ArrowUpCircle from '@lucide/svelte/icons/circle-arrow-up'
  import X from '@lucide/svelte/icons/x'
  import { openLink } from '../platform'
  import { updates } from '../stores/update.svelte'
  import WellnessWidget from '../../features/wellness/WellnessWidget.svelte'
  import ShieldCheck from '@lucide/svelte/icons/shield-check'
  import ExternalLink from '@lucide/svelte/icons/external-link'
  import FilePlain from '@lucide/svelte/icons/file'
  import House from '@lucide/svelte/icons/house'
  import SlidersHorizontal from '@lucide/svelte/icons/sliders-horizontal'
  import Unplug from '@lucide/svelte/icons/unplug'
  import CircleX from '@lucide/svelte/icons/circle-x'
  import TriangleAlert from '@lucide/svelte/icons/triangle-alert'
  import type { Component } from 'svelte'
  import PathFormats from '../../features/editor/PathFormats.svelte'
  import { activeEditor } from '../../features/editor/active.svelte'
  import Popover from '../components/Popover.svelte'
  import { lookOf } from '../../features/workspace/filetypes'
  import { workspace } from '../../features/workspace/workspace.svelte'
  import { lint } from '../../features/lint/lint.svelte'
  import { getVersion, type VersionInfo } from '../api/version'
  import { connection } from '../stores/connection.svelte'
  import { layout } from '../stores/layout.svelte'
  import { compareRoute, settingsRoute, newRoute, problemsRoute, queryRoute, router, routeAnsible, routeArgo, routeCI, routeClone, routeCompose, routeHelm, routeK8s, routeKustomize } from '../stores/router.svelte'
  import { hueVar, toolById, viewHue, viewIcon, type Hue } from '../tools'

  let version = $state<VersionInfo | null>(null)
  getVersion()
    .then((v) => (version = v))
    .catch(() => (version = null))

  const tool = $derived(toolById(router.toolId))
  const file = $derived(router.filePath)
  const entry = $derived(file ? workspace.byPath.get(file) : undefined)

  // The view's name first, on its colour (the accent for Home and
  // Settings), as on the rail and the tab bar.
  const lead = $derived.by((): { title: string; icon: Component; hue?: Hue } => {
    const p = router.path
    if (tool) return { title: tool.title, icon: tool.icon, hue: tool.hue }
    if (file) return { title: entry ? lookOf(entry).title : 'File', icon: FilePlain, hue: viewHue.explorer }
    if (routeHelm(p)) return { title: 'Helm', icon: viewIcon.helm, hue: viewHue.helm }
    if (p === problemsRoute) return { title: 'Problems', icon: viewIcon.problems, hue: viewHue.problems }
    if (routeK8s(p) !== null) return { title: 'Kubernetes', icon: viewIcon.k8s, hue: viewHue.k8s }
    if (routeKustomize(p) !== null) return { title: 'Kustomize', icon: viewIcon.kustomize, hue: viewHue.kustomize }
    if (routeArgo(p) !== null) return { title: 'Argo', icon: viewIcon.argo, hue: viewHue.argo }
    if (routeCI(p) !== null) return { title: 'Pipeline', icon: viewIcon.ci, hue: viewHue.ci }
    if (routeCompose(p) !== null) return { title: 'Compose', icon: viewIcon.compose, hue: viewHue.compose }
    if (routeAnsible(p) !== null) return { title: 'Ansible', icon: viewIcon.ansible, hue: viewHue.ansible }
    if (p === compareRoute) return { title: 'Compare', icon: viewIcon.compare, hue: viewHue.compare }
    if (p === queryRoute) return { title: 'Query', icon: viewIcon.query, hue: viewHue.query }
    if (p === newRoute) return { title: 'New', icon: viewIcon.new, hue: viewHue.new }
    if (routeClone(p) !== null) return { title: 'Clone', icon: viewIcon.clone, hue: viewHue.clone }
    if (p === settingsRoute) return { title: 'Settings', icon: SlidersHorizontal }
    return { title: 'Home', icon: House }
  })

  const watchText: Record<string, string> = {
    '': 'Starting to watch for changes…',
    native: 'Watching for changes (native events).',
    poll: 'Watching for changes by polling every second.',
  }
</script>

{#snippet counts(errors: number, warnings: number, title: string)}
  <span class="counts" {title}>
    <span><span class="e"><CircleX size={12} /></span>{errors}<span class="sr-only"> errors,</span></span>
    <span><span class="w"><TriangleAlert size={12} /></span>{warnings}<span class="sr-only"> warnings</span></span>
  </span>
{/snippet}

<footer class="statusbar">
  <div class="left">
    <span class="lead" class:hued={!!lead.hue} style:--tool={lead.hue && hueVar(lead.hue)}>
      <lead.icon size={12} />
      {lead.title}
    </span>
    {#if tool}
      {#if layout.status.text}
        <span class="ctx {layout.status.tone}">
          {#if layout.status.tone !== 'neutral'}<span class="pip"></span>{/if}
          <span class="text">{layout.status.text}</span>
        </span>
      {/if}
    {:else if file}
      {@const s = activeEditor.session}
      {#if s?.isYAML && s.path === file}
        <span class="ctx">
          {#if s.crumbs.length}<span class="text">{file}</span><span class="sep">·</span>{/if}
          <Popover side="top" align="start">
            {#snippet trigger(props)}
              <button {...props} type="button" class="crumbs" title="Copy the path at the cursor">
                {#each s.crumbs as n, i (i)}
                  {#if i > 0}<span class="sep">›</span>{/if}<span>{n}</span>
                {:else}
                  <span class="text">{file}</span>
                {/each}
              </button>
            {/snippet}
            <PathFormats session={s} />
          </Popover>
        </span>
        {@render counts(s.counts.error, s.counts.warning, 'Problems in this file')}
      {:else}
        <span class="ctx"><span class="text">{file}</span></span>
      {/if}
    {:else if routeHelm(router.path)}
      <span class="ctx"><span class="text">{routeHelm(router.path)} · rendered like helm template (no cluster; lookup returns empty)</span></span>
    {:else if router.path === problemsRoute}
      <span class="ctx"><span class="text">{lint.results ? `${lint.checked} ${lint.checked === 1 ? 'file' : 'files'} checked · ` : ''}lint rules from your profile</span></span>
      {#if lint.results}{@render counts(lint.totals.error, lint.totals.warning, 'Problems in the workspace')}{/if}
    {:else if routeK8s(router.path) !== null}
      <span class="ctx"><span class="text">{routeK8s(router.path)} · files on disk; Helm templates are left out (render them)</span></span>
    {:else if routeKustomize(router.path) !== null}
      <span class="ctx"><span class="text">{routeKustomize(router.path)} · built in-process like kubectl kustomize; remote bases not supported</span></span>
    {:else if routeArgo(router.path) !== null}
      <span class="ctx"><span class="text">{routeArgo(router.path)} · resolved without a cluster; run-time values stay as written</span></span>
    {:else if routeCI(router.path) !== null}
      <span class="ctx"><span class="text">{routeCI(router.path)} · local includes and templates followed; remote ones listed, never fetched</span></span>
    {:else if routeCompose(router.path) !== null}
      <span class="ctx"><span class="text">{routeCompose(router.path)} · merged like docker compose config; variables from .env and what-if values, not your shell</span></span>
    {:else if routeAnsible(router.path) !== null}
      <span class="ctx"><span class="text">{routeAnsible(router.path)} · in execution order; roles and includes followed in the folder</span></span>
    {:else if router.path === compareRoute}
      <span class="ctx"><span class="text">by meaning: key and list order ignored, list items paired by name</span></span>
    {:else if router.path === queryRoute}
      <span class="ctx"><span class="text">jq (gojq) over YAML documents</span></span>
    {:else if router.path === newRoute}
      <span class="ctx"><span class="text">starters are text to copy or download; codec writes no files</span></span>
    {:else if routeClone(router.path) !== null}
      <span class="ctx"><span class="text">{routeClone(router.path)} · a renamed copy to paste into a new file; the original is untouched</span></span>
    {:else if router.path === settingsRoute}
      <span class="ctx"><span class="text">saved in your user profile, never in a repository</span></span>
    {:else}
      <span class="ctx"><span class="text">Everything runs on this machine. Nothing you paste leaves it.</span></span>
    {/if}
  </div>

  <div class="right">
    {#if updates.notice}
      {@const n = updates.notice}
      <span class="update">
        <button type="button" class="update-link" title="Open the release page" onclick={() => openLink(n.url)}>
          <ArrowUpCircle size={12} /> codec {n.version} is available
        </button>
        <button type="button" class="update-x" aria-label="Dismiss until the next release" title="Dismiss until the next release" onclick={() => updates.dismiss()}>
          <X size={11} />
        </button>
      </span>
    {/if}
    <WellnessWidget />
    {#if workspace.disconnected && !connection.restarted}
      <button type="button" class="warn-btn" onclick={() => location.reload()} title="codec was restarted or stopped">
        <Unplug size={12} /> Disconnected — reload
      </button>
    {/if}
    {#if workspace.info?.open}
      {@const ViewIcon = viewIcon.explorer}
      <button
        type="button"
        class="ws hued"
        style:--tool={hueVar(viewHue.explorer)}
        title={`${workspace.info.root}\n${watchText[workspace.info.watch ?? ''] ?? watchText['']}\nClick to open another folder.`}
        onclick={() => workspace.chooseFolder()}
      >
        <span class="watch {workspace.info.watch ?? ''}"></span>
        <ViewIcon size={12} />
        {workspace.info.name}
      </button>
    {/if}
    <span class="local" title="codec runs entirely on this machine; nothing is sent anywhere">
      <ShieldCheck size={12} /> Local only
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
        Source on GitHub <ExternalLink size={12} />
      </a>
    </Popover>
  </div>
</footer>

<style>
  .statusbar {
    display: flex;
    align-items: stretch;
    justify-content: space-between;
    gap: var(--s-4);
    height: var(--statusbar-h);
    padding-right: var(--s-2);
    font-size: var(--fs-sm);
    color: var(--fg-1);
    background: var(--bg-0);
    border-top: 1px solid var(--border);
  }
  .left,
  .right {
    display: flex;
    align-items: center;
    gap: var(--s-3);
    min-width: 0;
  }
  .right {
    flex: none;
  }
  /* The view's name on a block of its colour, the bar's full height. */
  .lead {
    flex: none;
    align-self: stretch;
    display: flex;
    align-items: center;
    gap: var(--s-2);
    padding: 0 var(--s-3);
    font-weight: var(--fw-semibold);
    white-space: nowrap;
    color: var(--accent-fg);
    background: var(--tool-accent, var(--accent));
  }
  .ctx {
    display: flex;
    align-items: center;
    gap: var(--s-2);
    min-width: 0;
    white-space: nowrap;
  }
  .text {
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .ctx.err {
    color: var(--err);
  }
  /* A tool's line that is good news, a warning or an error. */
  .pip {
    flex: none;
    width: 7px;
    height: 7px;
    border-radius: 50%;
  }
  .ok .pip {
    background: var(--ok);
  }
  .warn .pip {
    background: var(--warn);
  }
  .err .pip {
    background: var(--err);
  }
  .sep {
    color: var(--fg-2);
  }
  .counts {
    flex: none;
    display: inline-flex;
    align-items: center;
    gap: var(--s-2);
    font-variant-numeric: tabular-nums;
  }
  .counts > span {
    display: inline-flex;
    align-items: center;
    gap: var(--s-1);
  }
  .counts .e,
  .counts .w {
    display: inline-flex;
  }
  .counts .e {
    color: var(--err);
  }
  .counts .w {
    color: var(--warn);
  }
  .crumbs {
    display: inline-flex;
    align-items: center;
    gap: var(--s-1);
    min-width: 0;
    height: 20px;
    padding: 0 var(--s-1);
    overflow: hidden;
    white-space: nowrap;
    font-family: var(--font-mono);
    font-size: var(--fs-xs);
    color: var(--fg-1);
    background: none;
    border: none;
    border-radius: var(--r-sm);
  }
  .crumbs:hover {
    color: var(--fg-0);
    background: var(--bg-3);
  }
  .ws,
  .warn-btn {
    display: inline-flex;
    align-items: center;
    gap: var(--s-2);
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
  .ws > :global(svg) {
    color: var(--tool-icon);
  }
  .warn-btn {
    color: var(--warn);
    background: var(--warn-soft);
  }
  /* Live-watch indicator: grey while starting, green for native
     events, amber when polling. */
  .watch {
    flex: none;
    width: 7px;
    height: 7px;
    border-radius: 50%;
    background: var(--fg-2);
  }
  .watch.native {
    background: var(--ok);
  }
  .watch.poll {
    background: var(--warn);
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
    color: var(--fg-2);
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
  .update {
    display: inline-flex;
    align-items: center;
    gap: 1px;
    color: var(--accent);
  }
  .update-link,
  .update-x {
    display: inline-flex;
    align-items: center;
    gap: 4px;
    height: 20px;
    padding: 0 var(--s-2);
    font-size: var(--fs-xs);
    color: inherit;
    background: var(--accent-soft);
    border: none;
    border-radius: var(--r-sm) 0 0 var(--r-sm);
  }
  .update-x {
    padding: 0 6px;
    border-radius: 0 var(--r-sm) var(--r-sm) 0;
  }
  .update-link:hover,
  .update-x:hover {
    color: var(--fg-0);
  }
</style>
