<script lang="ts">
  import ListTree from '@lucide/svelte/icons/list-tree'
  import LensView from '../shared/LensView.svelte'
  import AnsiblePanel from './AnsiblePanel.svelte'

  // The Ansible view of one file: a playbook or task file in execution
  // order with roles and includes followed, or an inventory's groups.
  let { path, active }: { path: string; active: boolean } = $props()
</script>

<LensView {path} {active} icon={ListTree} title="Ansible · {path.split('/').pop()}">
  {#snippet children(ctx)}
    <AnsiblePanel {path} overrides={ctx.overrides} reload={ctx.reload} onopen={ctx.open} onstatus={ctx.status} />
  {/snippet}
</LensView>
