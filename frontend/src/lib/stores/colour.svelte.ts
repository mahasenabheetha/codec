// Tool colours: whether tools and actions show their own colours or
// all take the accent (decision 105). A display preference, persisted;
// off sets data-colour='plain' on <html>, which zeroes the tints in
// tokens.css. index.html applies it before the app loads.
import { persisted } from './persist.svelte'

const pref = persisted<boolean>('toolColours', true)

$effect.root(() => {
  $effect(() => {
    if (pref.value) delete document.documentElement.dataset.colour
    else document.documentElement.dataset.colour = 'plain'
  })
})

export const toolColours = {
  get on(): boolean {
    return pref.value
  },
  set on(v: boolean) {
    pref.value = v
  },
}
