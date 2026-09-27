import { mount } from 'svelte'
// Fonts are bundled (no network requests); only the subsets a page
// actually uses are downloaded, thanks to unicode-range.
import '@fontsource-variable/inter'
import '@fontsource-variable/jetbrains-mono'
import './lib/styles/tokens.css'
import './lib/styles/global.css'
import App from './App.svelte'

const app = mount(App, {
  target: document.getElementById('app')!,
})

export default app
