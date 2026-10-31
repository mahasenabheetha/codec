<script lang="ts">
  import WorkTabStrip from '../shared/WorkTabStrip.svelte'
  import { workTabShortcuts } from '../shared/worktabs.svelte'
  import AnsibleTab from './AnsibleTab.svelte'
  import { ansibleTabs } from './tabs.svelte'

  // The Ansible log tool with work tabs (decision 97): several logs open
  // at once. A tab's name turns red when its log has failures.
  let { active }: { active: boolean } = $props()

  const add = () => ansibleTabs.add()
  workTabShortcuts(() => active, ansibleTabs, add)
</script>

{#snippet strip()}
  <WorkTabStrip
    tabs={ansibleTabs}
    title={(t) => t.title}
    label="Open logs"
    addLabel="New log"
    onadd={add}
    rename={(t, name) => (t.name = name)}
    tip={(t) => (t.failures ? `${t.failures} failed or unreachable` : t.title)}
    bad={(t) => t.failures > 0}
  />
{/snippet}

{#each ansibleTabs.list as t (t.key)}
  <div class="tab" class:hidden={t !== ansibleTabs.current}>
    <AnsibleTab tab={t} active={active && t === ansibleTabs.current} {strip} />
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
