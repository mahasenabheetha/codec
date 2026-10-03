<script lang="ts">
  import { layout } from '../../lib/stores/layout.svelte'
  import { currentTab, toolById } from '../../lib/tools'
  import HashTab from './HashTab.svelte'
  import HexTab from './HexTab.svelte'
  import HtpasswdTab from './HtpasswdTab.svelte'
  import SecretsTab from './SecretsTab.svelte'
  import UrlTab from './UrlTab.svelte'

  // Encode & hash: one sub-tab per job. A sub-tab is mounted when first
  // shown and then kept, so switching back finds its input as it was.
  let { active }: { active: boolean } = $props()

  const tool = toolById('encode')!
  const tabs = { url: UrlTab, hex: HexTab, hash: HashTab, secrets: SecretsTab, htpasswd: HtpasswdTab }
  const tab = $derived(currentTab(tool, layout.toolTab(tool.id)))
  const seen = $state<Record<string, boolean>>({})
  $effect(() => {
    seen[tab] = true
  })
</script>

{#each Object.entries(tabs) as [id, Tab] (id)}
  {#if seen[id] || id === tab}
    <div class="tab" data-subtab={id} hidden={id !== tab}>
      <Tab {tool} active={active && id === tab} />
    </div>
  {/if}
{/each}

<style>
  .tab {
    height: 100%;
  }
  .tab[hidden] {
    display: none;
  }
</style>
