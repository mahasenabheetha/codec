<script lang="ts">
  import WorkTabStrip from '../shared/WorkTabStrip.svelte'
  import { workTabShortcuts } from '../shared/worktabs.svelte'
  import CompareTab from './CompareTab.svelte'
  import { compareTabs } from './compare.svelte'

  // Compare with work tabs (decision 97): several comparisons open at once.
  let { active }: { active: boolean } = $props()

  const add = () => compareTabs.add()
  workTabShortcuts(() => active, compareTabs, add)
</script>

{#snippet strip()}
  <WorkTabStrip tabs={compareTabs} title={(t) => t.title} label="Open comparisons" addLabel="New comparison" onadd={add} rename={(t, name) => (t.name = name)} />
{/snippet}

{#each compareTabs.list as t (t.key)}
  <div class="tab" class:hidden={t !== compareTabs.current}>
    <CompareTab c={t} active={active && t === compareTabs.current} {strip} />
  </div>
{/each}

<style>
  .tab {
    height: 100%;
  }
  .tab.hidden {
    display: none;
  }
</style>
