<script lang="ts">
  import WorkTabStrip from '../shared/WorkTabStrip.svelte'
  import { workTabShortcuts } from '../shared/worktabs.svelte'
  import Base64Tab from './Base64Tab.svelte'
  import { base64Tabs } from './tabs.svelte'

  // Base64 with work tabs (decision 97).
  let { active }: { active: boolean } = $props()

  const add = () => base64Tabs.add()
  workTabShortcuts(() => active, base64Tabs, add)
</script>

{#snippet strip()}
  <WorkTabStrip tabs={base64Tabs} title={(t) => t.title} label="Open base64 tabs" addLabel="New tab" onadd={add} rename={(t, name) => (t.name = name)} />
{/snippet}

{#each base64Tabs.list as t (t.key)}
  <div class="tab" class:hidden={t !== base64Tabs.current}>
    <Base64Tab tab={t} active={active && t === base64Tabs.current} {strip} />
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
