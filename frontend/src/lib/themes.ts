// The colour themes, as Settings lists them. Plain data, so a test can
// check the copied colours against tokens.css (themes.test.mjs).

export type Theme = 'dark' | 'dusk' | 'midnight' | 'forest' | 'rose' | 'ocean' | 'contrast' | 'light' | 'paper' | 'mist'
export type ThemePref = 'system' | Theme
export type DarkStyle = Exclude<Theme, 'light' | 'paper' | 'mist'>
export type LightStyle = 'light' | 'paper' | 'mist'

export interface ThemeInfo {
  value: Theme
  label: string
  light: boolean
  note: string
  /** `--bg-2`, `--fg-0`, `--accent` and one of the hues. */
  swatch: [string, string, string, string]
  /** `--hue-indigo` … `--hue-cyan`, in the order of the spectrum strip. */
  hues: readonly string[]
}

// Copied from tokens.css: a card can't read another theme's variables
// without switching to it.
const duskHues = ['#a8bcf5', '#c6b2ed', '#d8acdf', '#e1a9d4', '#ebae8d', '#b1c88b', '#7ad0c5', '#7ccbe4']

export const themes: ThemeInfo[] = [
  {
    value: 'dark',
    label: 'Dark',
    light: false,
    note: 'neutral, sharp',
    swatch: ['#171b25', '#eceef4', '#7c9cff', '#8f9dff'],
    hues: ['#8f9dff', '#b794ff', '#e391f2', '#ff8fc8', '#ffa877', '#b4e06a', '#4fdcc2', '#5cc6ff'],
  },
  { value: 'dusk', label: 'Dusk', light: false, note: 'soft blue-violet, colourful', swatch: ['#252636', '#d5d6e4', '#acb1f0', '#e1a9d4'], hues: duskHues },
  { value: 'midnight', label: 'Midnight', light: false, note: "Dusk's colours on deep navy", swatch: ['#111f35', '#cbcfdc', '#acb1f0', '#7ad0c5'], hues: duskHues },
  { value: 'forest', label: 'Forest', light: false, note: 'green-grey, calm', swatch: ['#1c2520', '#cbd5cd', '#a6c2e6', '#b1c88b'], hues: duskHues },
  { value: 'rose', label: 'Rosé', light: false, note: 'pink-violet, from Dusk', swatch: ['#2b2330', '#e2d5e1', '#e8a8d0', '#c6b2ed'], hues: duskHues },
  { value: 'ocean', label: 'Ocean', light: false, note: 'blue-teal on deep blue', swatch: ['#122532', '#c3d5dd', '#76cfd6', '#7ccbe4'], hues: duskHues },
  {
    value: 'contrast',
    label: 'High contrast',
    light: false,
    note: 'black and white, 7:1 everywhere',
    swatch: ['#0b0b0b', '#ffffff', '#5cc8ff', '#ffb3da'],
    hues: ['#aab6ff', '#cdb8ff', '#f0bbf8', '#ffb3da', '#ffc7a3', '#c6ef93', '#82ead8', '#92ddff'],
  },
  {
    value: 'light',
    label: 'Light',
    light: true,
    note: 'neutral, bright',
    swatch: ['#ffffff', '#151922', '#3756d8', '#00796a'],
    hues: ['#4652d6', '#7444d4', '#9b3ab0', '#bf2f7a', '#b64d0c', '#4a7400', '#00796a', '#006da8'],
  },
  {
    value: 'paper',
    label: 'Paper',
    light: true,
    note: 'warm, low glare',
    swatch: ['#fbf7ef', '#2d261d', '#2d5fa8', '#a83a76'],
    hues: ['#4353b5', '#7046ad', '#8b4490', '#a83a76', '#a3501f', '#50691a', '#1a6f67', '#1b678c'],
  },
  {
    value: 'mist',
    label: 'Mist',
    light: true,
    note: 'light, dimmed to soft grey',
    swatch: ['#c7cad1', '#07090c', '#1b3b8a', '#7b2b59'],
    hues: ['#2f3d8e', '#533487', '#64346c', '#7b2b59', '#77391a', '#3a5214', '#11514b', '#124c66'],
  },
]
