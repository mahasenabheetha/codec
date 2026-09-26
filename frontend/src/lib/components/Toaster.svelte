<script lang="ts">
  import { toasts } from '../stores/toast.svelte'
  import CircleCheck from '@lucide/svelte/icons/circle-check'
  import TriangleAlert from '@lucide/svelte/icons/triangle-alert'
  import CircleX from '@lucide/svelte/icons/circle-x'
  import Info from '@lucide/svelte/icons/info'

  const icons = { ok: CircleCheck, warn: TriangleAlert, err: CircleX, neutral: Info }
</script>

<div class="toaster" role="status" aria-live="polite">
  {#each toasts.items as t (t.id)}
    {@const Icon = icons[t.tone]}
    <div class="toast {t.tone}">
      <Icon size={15} strokeWidth={2} />
      <span>{t.message}</span>
    </div>
  {/each}
</div>

<style>
  .toaster {
    position: fixed;
    left: 50%;
    bottom: calc(var(--statusbar-h) + var(--s-4));
    transform: translateX(-50%);
    z-index: var(--z-toast);
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: var(--s-2);
    pointer-events: none;
  }
  .toast {
    display: flex;
    align-items: center;
    gap: var(--s-2);
    padding: var(--s-2) var(--s-3);
    font-size: var(--fs-md);
    background: var(--bg-3);
    border-radius: var(--r-md);
    box-shadow: var(--shadow-pop);
    animation: toast-in 160ms var(--ease);
  }
  .ok :global(svg) {
    color: var(--ok);
  }
  .warn :global(svg) {
    color: var(--warn);
  }
  .err :global(svg) {
    color: var(--err);
  }
  .neutral :global(svg) {
    color: var(--info);
  }
  @keyframes toast-in {
    from {
      opacity: 0;
      transform: translateY(6px);
    }
  }
</style>
