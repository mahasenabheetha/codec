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

export type Theme = 'dark' | 'dusk' | 'midnight' | 'forest' | 'rose' | 'ocean' | 'contrast' | 'light' | 'paper' | 'mist'
export type ThemePref = 'system' | Theme
export type DarkStyle = Exclude<Theme, 'light' | 'paper' | 'mist'>
export type LightStyle = 'light' | 'paper' | 'mist'

/** The themes, as Settings lists them. The swatch is the theme's ground,
 *  text, accent and one tool hue, copied from tokens.css (a menu can't
 *  read another theme's variables without switching to it). */
export const themes: { value: Theme; label: string; light: boolean; note: string; swatch: [string, string, string, string] }[] = [
  { value: 'dark', label: 'Dark', light: false, note: 'neutral, sharp', swatch: ['#161a23', '#e6e8ee', '#7aa2ff', '#9aa5f5'] },
  { value: 'dusk', label: 'Dusk', light: false, note: 'soft blue-violet, colourful', swatch: ['#252636', '#d5d6e4', '#acb1f0', '#e1a9d4'] },
  { value: 'midnight', label: 'Midnight', light: false, note: "Dusk's colours on deep navy", swatch: ['#111f35', '#cbcfdc', '#acb1f0', '#7ad0c5'] },
  { value: 'forest', label: 'Forest', light: false, note: 'green-grey, calm', swatch: ['#1c2520', '#cbd5cd', '#a6c2e6', '#b1c88b'] },
  { value: 'rose', label: 'Rosé', light: false, note: 'pink-violet, from Dusk', swatch: ['#2b2330', '#e2d5e1', '#e8a8d0', '#c6b2ed'] },
  { value: 'ocean', label: 'Ocean', light: false, note: 'blue-teal on deep blue', swatch: ['#122532', '#c3d5dd', '#76cfd6', '#7ccbe4'] },
  { value: 'contrast', label: 'High contrast', light: false, note: 'black and white, 7:1 everywhere', swatch: ['#000000', '#ffffff', '#5cc8ff', '#ffb3da'] },
  { value: 'light', label: 'Light', light: true, note: 'neutral, bright', swatch: ['#ffffff', '#1b1f27', '#2b5cd6', '#0b7268'] },
  { value: 'paper', label: 'Paper', light: true, note: 'warm, low glare', swatch: ['#f9f5ee', '#2a251f', '#2f5d9e', '#a3427a'] },
  { value: 'mist', label: 'Mist', light: true, note: 'light, dimmed to soft grey', swatch: ['#d1d4da', '#1e222a', '#22488a', '#7e2f5c'] },
]

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
