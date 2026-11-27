<script lang="ts">
  import SegmentedControl from '../../lib/components/SegmentedControl.svelte'
  import Select from '../../lib/components/Select.svelte'
  import Toggle from '../../lib/components/Toggle.svelte'
  import type { Level } from '../../lib/api/lint'
  import { groupTitles, lint } from '../lint/lint.svelte'
  import SettingRow from './SettingRow.svelte'
  import SettingSection from './SettingSection.svelte'

  // Rule levels, the target Kubernetes version and schema options.
  // They apply to the editor, workspace lint, Helm output and
  // `codec yaml lint` alike.

  // Quietest first, as the levels read left to right.
  const levels: { value: Level; label: string }[] = [
    { value: 'off', label: 'Off' },
    { value: 'info', label: 'Info' },
    { value: 'warning', label: 'Warning' },
    { value: 'error', label: 'Error' },
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

<SettingSection id="kubernetes" title="Kubernetes">
  <SettingRow title="Target version" description="Schemas and API removals are checked for this version.">
    <Select
      items={versionItems}
      value={lint.settings.k8sVersion ?? ''}
      label="Kubernetes version"
      onchange={(v) => lint.update({ k8sVersion: v || undefined })}
    />
  </SettingRow>
</SettingSection>

<SettingSection id="schemas" title="Schemas">
  <SettingRow
    title="Validate against schemas"
    description="Kubernetes and CRDs, GitHub Actions, GitLab CI, Azure Pipelines, Compose. Also powers key completion and field docs."
  >
    <Toggle checked={!lint.settings.noSchemas} name="Validate against schemas" label={lint.settings.noSchemas ? 'Off' : 'On'} onchange={(on) => lint.update({ noSchemas: !on || undefined })} />
  </SettingRow>
  <SettingRow title="Offline" description="Never download: use schemas already cached and your custom folder.">
    <Toggle checked={!!lint.settings.offline} name="Offline" label={lint.settings.offline ? 'On' : 'Off'} onchange={(on) => lint.update({ offline: on || undefined })} />
  </SettingRow>
  <SettingRow title="Custom schema folder" for="schema-dir">
    {#snippet details()}
      Checked first. Put files there by name (<code>deployment-apps-v1.json</code>), in the CRD catalog layout (<code
        >argoproj.io/workflow_v1alpha1.json</code
      >), or copy codec's cache folder.
    {/snippet}
    <input
      id="schema-dir"
      bind:value={schemaDir}
      placeholder="e.g. C:\schemas"
      spellcheck="false"
      onblur={saveDir}
      onkeydown={(e) => e.key === 'Enter' && saveDir()}
    />
  </SettingRow>
</SettingSection>

{#each groups as g (g.id)}
  {@const off = g.rules.filter((r) => lint.level(r.id) === 'off').length}
  <SettingSection id="rules-{g.id}" title="{g.title} rules" description="{g.rules.length - off} on, {off} off">
    {#each g.rules as r (r.id)}
      <SettingRow title={r.title} description={r.why} code={r.id}>
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
        <SegmentedControl options={levels} value={lint.level(r.id)} label="Level of {r.title}" onchange={(v) => lint.setLevel(r.id, v)} />
      </SettingRow>
    {/each}
  </SettingSection>
{/each}

<style>
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
  input:focus-visible {
    border-color: var(--accent);
    box-shadow: 0 0 0 1px var(--accent);
  }
</style>
