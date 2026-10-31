// The colour theme. The preference is System, Dark, Dusk, Midnight or
// Light (a display preference, persisted); `resolved` is what is shown.
// System follows the operating system, using Dark, Dusk or Midnight when
// it is dark (the "dark style", decisions 94 and 95). The attribute it sets on <html> switches
// the token set in tokens.css. index.html applies the same choice before
// the app loads, so the first paint is right.
import { persisted } from './persist.svelte'
import { windowTheme } from '../platform'

export type ThemePref = 'system' | 'dark' | 'dusk' | 'midnight' | 'light'
export type Theme = 'dark' | 'dusk' | 'midnight' | 'light'
export type DarkStyle = 'dark' | 'dusk' | 'midnight'

const pref = persisted<ThemePref>('theme', 'dark')
const darkStyle = persisted<DarkStyle>('darkStyle', 'dark')
const media = window.matchMedia('(prefers-color-scheme: light)')
let systemLight = $state(media.matches)
media.addEventListener('change', (e) => (systemLight = e.matches))

const resolved = $derived<Theme>(
  pref.value === 'system' ? (systemLight ? 'light' : darkStyle.value) : pref.value,
)

$effect.root(() => {
  $effect(() => {
    document.documentElement.dataset.theme = resolved
    // Browser chrome (and an installed app's title bar) follows --bg-0.
    const bg = getComputedStyle(document.documentElement).getPropertyValue('--bg-0').trim()
    document.querySelector('meta[name="theme-color"]')?.setAttribute('content', bg)
    windowTheme(resolved === 'light' ? 'light' : 'dark')
  })
})

export const theme = {
  get pref(): ThemePref {
    return pref.value
  },
  set pref(v: ThemePref) {
    pref.value = v
  },
  /** What System uses when the operating system is dark. */
  get darkStyle(): DarkStyle {
    return darkStyle.value
  },
  set darkStyle(v: DarkStyle) {
    darkStyle.value = v
  },
  get resolved(): Theme {
    return resolved
  },
  /** Dark, Dusk or Midnight: a dark background. */
  get dark(): boolean {
    return resolved !== 'light'
  },
}
