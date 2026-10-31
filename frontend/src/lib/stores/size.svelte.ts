// The interface size: how big icons and controls are (decision 94). A
// display preference, persisted; the attribute it sets on <html> switches
// the size tokens in tokens.css. index.html applies it before the app
// loads, so the first paint is right.
import { persisted } from './persist.svelte'

export type UiSize = 'compact' | 'comfortable' | 'large'

const pref = persisted<UiSize>('size', 'comfortable')

$effect.root(() => {
  $effect(() => {
    document.documentElement.dataset.size = pref.value
  })
})

export const uiSize = {
  get value(): UiSize {
    return pref.value
  },
  set value(v: UiSize) {
    pref.value = v
  },
}
