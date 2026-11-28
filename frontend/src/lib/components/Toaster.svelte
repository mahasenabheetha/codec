<script lang="ts">
  import { toasts } from '../stores/toast.svelte'
  import Check from '@lucide/svelte/icons/check'
  import TriangleAlert from '@lucide/svelte/icons/triangle-alert'
  import X from '@lucide/svelte/icons/x'
  import Info from '@lucide/svelte/icons/info'
  import Tile from './Tile.svelte'

  const marks = {
    ok: { icon: Check, status: 'ok' },
    warn: { icon: TriangleAlert, status: 'warn' },
    err: { icon: X, status: 'err' },
    neutral: { icon: Info, status: 'info' },
  } as const
</script>

<div class="toaster" role="status" aria-live="polite">
  {#each toasts.items as t (t.id)}
    {@const mark = marks[t.tone]}
    <div class="toast">
      <Tile icon={mark.icon} status={mark.status} size="sm" solid />
      <span>{t.message}</span>
      {#if t.action}
        {@const action = t.action}
        <button
          type="button"
          class="action"
          onpointerenter={() => toasts.hold(t.id)}
          onpointerleave={() => toasts.resume(t.id)}
          onfocus={() => toasts.hold(t.id)}
          onblur={() => toasts.resume(t.id)}
          onclick={() => {
            toasts.dismiss(t.id)
            action.run()
          }}>{action.label}</button
        >
      {/if}
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
    gap: var(--s-3);
    padding: var(--s-2) var(--s-3);
    font-size: var(--fs-md);
    background: var(--bg-3);
    border-radius: var(--r-lg);
    box-shadow: var(--shadow-pop);
    animation: toast-in 160ms var(--ease);
  }
  /* A small ghost button in the accent. */
  .action {
    height: var(--control-h-sm);
    padding: 0 var(--s-2);
    font-size: var(--fs-sm);
    font-weight: var(--fw-semibold);
    color: var(--accent);
    background: none;
    border: none;
    border-radius: var(--r-md);
    pointer-events: auto;
    transition: background var(--dur-fast) var(--ease);
  }
  .action:hover {
    background: var(--bg-4);
  }
  @keyframes toast-in {
    from {
      opacity: 0;
      transform: translateY(6px);
    }
  }
</style>
