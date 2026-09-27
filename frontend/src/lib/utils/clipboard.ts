import { toast } from '../stores/toast.svelte'

/** Copy text and confirm with a toast; never throws. */
export async function copyText(text: string, what = 'Output'): Promise<void> {
  if (!text) {
    toast(`Nothing to copy`, 'warn')
    return
  }
  try {
    await navigator.clipboard.writeText(text)
    toast(`${what} copied`, 'ok')
  } catch {
    toast('Clipboard unavailable in this browser context', 'err')
  }
}
