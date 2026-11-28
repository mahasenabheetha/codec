// The colour theme. The preference is System or one of the themes (a
// display preference, persisted); `resolved` is what is shown. System
// follows the operating system, using the chosen light style (Light or
// Paper) when it is light and the dark style (Dark, Dusk, Midnight,
// Forest, Rosé, Ocean or High contrast) when it is dark (decisions 94
// and 95, 3.10.0). The attribute it sets on <html> switches the token set
// in tokens.css. index.html applies the same choice before the app
// loads, so the first paint is right.
import { persisted } from './persist.svelte'
import { windowTheme } from '../platform'
import { themes, type DarkStyle, type LightStyle, type Theme, type ThemePref } from '../themes'

export { themes }
export type { DarkStyle, LightStyle, Theme, ThemePref }

const known = (v: string): v is Theme => themes.some((t) => t.value === v)

const pref = persisted<ThemePref>('theme', 'dark')
const darkStyle = persisted<DarkStyle>('darkStyle', 'dark')
const lightStyle = persisted<LightStyle>('lightStyle', 'light')
const media = window.matchMedia('(prefers-color-scheme: light)')
let systemLight = $state(media.matches)
media.addEventListener('change', (e) => (systemLight = e.matches))

const resolved = $derived.by<Theme>(() => {
  const p = pref.value === 'system' ? (systemLight ? lightStyle.value : darkStyle.value) : pref.value
  return known(p) ? p : 'dark'
})
const isLight = $derived(themes.find((t) => t.value === resolved)?.light ?? false)

$effect.root(() => {
  $effect(() => {
    document.documentElement.dataset.theme = resolved
    // Browser chrome (and an installed app's title bar) follows --bg-0.
    const bg = getComputedStyle(document.documentElement).getPropertyValue('--bg-0').trim()
    document.querySelector('meta[name="theme-color"]')?.setAttribute('content', bg)
    windowTheme(isLight ? 'light' : 'dark')
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
  /** What System uses when the operating system is light. */
  get lightStyle(): LightStyle {
    return lightStyle.value
  },
  set lightStyle(v: LightStyle) {
    lightStyle.value = v
  },
  get resolved(): Theme {
    return resolved
  },
  /** A dark background (every theme but Light and Paper). */
  get dark(): boolean {
    return !isLight
  },
  /** Pick a theme, and make it what System uses for its kind. */
  use(t: Theme) {
    pref.value = t
    if (themes.find((x) => x.value === t)?.light) lightStyle.value = t as LightStyle
    else darkStyle.value = t as DarkStyle
  },
}
