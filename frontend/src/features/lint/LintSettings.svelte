<script lang="ts">
  import Select from '../../lib/components/Select.svelte'
  import Toggle from '../../lib/components/Toggle.svelte'
  import type { Level } from '../../lib/api/lint'
  import { groupTitles, lint } from './lint.svelte'

  // Rule levels, the target Kubernetes version and schema options.
  // Saved in codec's settings (the user profile), never in a repo.
  interface Props {
    active: boolean
  }

  let { active }: Props = $props()

  if (!lint.loaded) lint.load()

  const levels: { value: Level; label: string }[] = [
    { value: 'error', label: 'Error' },
    { value: 'warning', label: 'Warning' },
    { value: 'info', label: 'Info' },
    { value: 'off', label: 'Off' },
  ]
  const versionItems = $derived([
    { value: '', label: `Default (${lint.defaultK8sVersion})` },
    ...lint.k8sVersions.map((v) => ({ value: v, label: v })),
  ])
  const groups = $derived(
    Object.keys(groupTitles).map((g) => ({ id: g, title: groupTitles[g], rules: lint.rules.filter((r) => r.group === g) })),
  )

  let schemaDir = $state('')
  let lineLength = $state('')
  $effect(() => {
    schemaDir = lint.settings.schemaDir ?? ''
    lineLength = lint.settings.lineLength ? String(lint.settings.lineLength) : ''
  })

  function saveDir() {
    const v = schemaDir.trim()
    if (v !== (lint.settings.schemaDir ?? '')) lint.update({ schemaDir: v || undefined })
  }
  function saveLength() {
    const n = parseInt(lineLength, 10)
    const v = Number.isFinite(n) && n > 0 ? n : undefined
    if (v !== lint.settings.lineLength) lint.update({ lineLength: v })
  }
</script>

<div class="settings" class:inactive={!active}>
  <header>
    <h1>Lint settings</h1>
    <p>Saved in your codec settings, not in the repository. Applies to the editor, workspace lint, Helm output and <code>codec yaml lint</code>.</p>
  </header>

  {#if !lint.loaded}
    <p class="muted">{lint.error ?? 'Loading…'}</p>
  {:else}
    <section>
      <h2>Kubernetes</h2>
      <div class="row">
        <div class="label">
          <span>Target version</span>
          <small>Schemas and API removals are checked for this version.</small>
        </div>
        <Select
          items={versionItems}
          value={lint.settings.k8sVersion ?? ''}
          label="Kubernetes version"
          onchange={(v) => lint.update({ k8sVersion: v || undefined })}
        />
      </div>
    </section>

    <section>
      <h2>Schemas</h2>
      <div class="row">
        <div class="label">
          <span>Validate against schemas</span>
          <small>Kubernetes and CRDs, GitHub Actions, GitLab CI, Azure Pipelines, Compose. Also powers key completion and field docs.</small>
        </div>
        <Toggle checked={!lint.settings.noSchemas} label={lint.settings.noSchemas ? 'Off' : 'On'} onchange={(on) => lint.update({ noSchemas: !on || undefined })} />
      </div>
      <div class="row">
        <div class="label">
          <span>Offline</span>
          <small>Never download: use schemas already cached and your custom folder.</small>
        </div>
        <Toggle checked={!!lint.settings.offline} label={lint.settings.offline ? 'On' : 'Off'} onchange={(on) => lint.update({ offline: on || undefined })} />
      </div>
      <div class="row">
        <label class="label" for="schema-dir">
          <span>Custom schema folder</span>
          <small>Checked first. Put files there by name (<code>deployment-apps-v1.json</code>), in the CRD catalog layout (<code>argoproj.io/workflow_v1alpha1.json</code>), or copy codec's cache folder.</small>
        </label>
        <input
          id="schema-dir"
          bind:value={schemaDir}
          placeholder="e.g. C:\schemas"
          spellcheck="false"
          onblur={saveDir}
          onkeydown={(e) => e.key === 'Enter' && saveDir()}
        />
      </div>
    </section>

    {#each groups as g (g.id)}
      <section>
        <h2>{g.title}</h2>
        {#each g.rules as r (r.id)}
          <div class="row">
            <div class="label">
              <span>{r.title} <code class="id">{r.id}</code></span>
              <small>{r.why}</small>
            </div>
            <div class="controls">
              {#if r.id === 'line-length' && lint.level(r.id) !== 'off'}
                <input
                  class="num"
                  bind:value={lineLength}
                  placeholder="120"
                  inputmode="numeric"
                  aria-label="Maximum line length"
                  onblur={saveLength}
                  onkeydown={(e) => e.key === 'Enter' && saveLength()}
                />
              {/if}
              <Select items={levels} value={lint.level(r.id)} label="Level of {r.title}" onchange={(v) => lint.setLevel(r.id, v as Level)} />
            </div>
          </div>
        {/each}
      </section>
    {/each}
  {/if}
</div>

<style>
  .settings {
    height: 100%;
    overflow: auto;
    padding: var(--s-6) var(--s-6) var(--s-8);
  }
  header,
  section {
    max-width: 820px;
  }
  h1 {
    margin: 0 0 var(--s-1);
    font-size: var(--fs-xl);
    font-weight: var(--fw-semibold);
  }
  header p,
  .muted {
    margin: 0;
    font-size: var(--fs-sm);
    color: var(--fg-2);
  }
  section {
    margin-top: var(--s-6);
  }
  h2 {
    margin: 0 0 var(--s-2);
    font-size: var(--fs-xs);
    font-weight: var(--fw-semibold);
    letter-spacing: 0.06em;
    text-transform: uppercase;
    color: var(--fg-2);
  }
  .row {
    display: flex;
    align-items: center;
    gap: var(--s-4);
    padding: var(--s-3) 0;
    border-top: 1px solid var(--border);
  }
  .label {
    flex: 1;
    display: flex;
    flex-direction: column;
    gap: 2px;
    min-width: 0;
    font-size: var(--fs-md);
  }
  .label small {
    font-size: var(--fs-sm);
    color: var(--fg-2);
    line-height: 1.4;
  }
  .id {
    margin-left: var(--s-1);
    font-size: var(--fs-xs);
    color: var(--fg-2);
  }
  .controls {
    display: flex;
    align-items: center;
    gap: var(--s-2);
  }
  code {
    font-family: var(--font-mono);
    font-size: 0.92em;
  }
  input {
    width: 260px;
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
  input.num {
    width: 64px;
  }
  input:focus {
    border-color: var(--accent);
  }
  @media (max-width: 720px) {
    .settings {
      padding: var(--s-4);
    }
    .row {
      flex-wrap: wrap;
    }
  }
</style>
