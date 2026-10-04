// The colour theme. The preference is System, Dark or Light (a display
// preference, persisted); `resolved` is what is shown. The attribute it
// sets on <html> switches the token set in tokens.css. index.html applies
// the same choice before the app loads, so the first paint is right.
import { persisted } from './persist.svelte'
import { windowTheme } from '../platform'

export type ThemePref = 'system' | 'dark' | 'light'
export type Theme = 'dark' | 'light'

const pref = persisted<ThemePref>('theme', 'dark')
const media = window.matchMedia('(prefers-color-scheme: light)')
let systemLight = $state(media.matches)
media.addEventListener('change', (e) => (systemLight = e.matches))

const resolved = $derived<Theme>(
  pref.value === 'system' ? (systemLight ? 'light' : 'dark') : pref.value === 'light' ? 'light' : 'dark',
)

$effect.root(() => {
  $effect(() => {
    document.documentElement.dataset.theme = resolved
    // Browser chrome (and an installed app's title bar) follows --bg-0.
    const bg = getComputedStyle(document.documentElement).getPropertyValue('--bg-0').trim()
    document.querySelector('meta[name="theme-color"]')?.setAttribute('content', bg)
    windowTheme(resolved)
  })
})

export const theme = {
  get pref(): ThemePref {
    return pref.value
  },
  set pref(v: ThemePref) {
    pref.value = v
  },
  get resolved(): Theme {
    return resolved
  },
}
