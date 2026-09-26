<script lang="ts">
  import type { FileEntry } from '../../lib/api/workspace'
  import { lookOf } from './filetypes'

  // A file's type logo (Kubernetes, Helm, …) or a generic icon.
  interface Props {
    file: Pick<FileEntry, 'type' | 'lang'> | string
    size?: number
  }

  let { file, size = 14 }: Props = $props()
  const look = $derived(lookOf(file))
</script>

<span class="file-icon" style="--size: {size}px; color: {look.color}">
  {#if look.logo}
    <!-- Bundled SVG from assets/logos, not user content. -->
    {@html look.logo}
  {:else if look.icon}
    <look.icon size={size} strokeWidth={1.75} />
  {/if}
</span>

<style>
  .file-icon {
    display: inline-grid;
    place-items: center;
    flex: 0 0 auto;
    width: var(--size);
    height: var(--size);
  }
  .file-icon :global(svg) {
    width: 100%;
    height: 100%;
  }
  /* Simple Icons marks are filled shapes; Lucide icons set their own stroke. */
  .file-icon :global(svg[role='img']) {
    fill: currentColor;
    /* Logos fill the full 24px box; shrink them to match Lucide's optical size. */
    transform: scale(0.86);
  }
</style>
