<script lang="ts">
  import WorkTabStrip from '../shared/WorkTabStrip.svelte'
  import { workTabShortcuts } from '../shared/worktabs.svelte'
  import SmartTab from './SmartTab.svelte'
  import { smartTabs } from './tabs.svelte'

  // Smart paste with work tabs (decision 97): several pastes open at once.
  let { active }: { active: boolean } = $props()

  const add = () => smartTabs.add()
  workTabShortcuts(() => active, smartTabs, add)
</script>

{#snippet strip()}
  <WorkTabStrip tabs={smartTabs} title={(t) => t.title} label="Open pastes" addLabel="New paste" onadd={add} rename={(t, name) => (t.name = name)} />
{/snippet}

{#each smartTabs.list as t (t.key)}
  <div class="tab" class:hidden={t !== smartTabs.current}>
    <SmartTab tab={t} active={active && t === smartTabs.current} {strip} />
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
