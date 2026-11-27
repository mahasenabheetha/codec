// Narrow (Spectrum, 3.11.0): a window under 1000 px. The file view's
// panel goes under the editor, a tool's input over its output (unless
// the tool was set side by side) and Compare shows one side at a time.
import { MediaQuery } from 'svelte/reactivity'

export const narrow = new MediaQuery('max-width: 999.98px')
