// Brings in the Material Icon Theme (MIT) for the file tree (decision 100):
//   node scripts/file-icons.mjs <unpacked material-icon-theme package>
// e.g. after `npm pack material-icon-theme` and extracting it. Copies the
// SVGs the mapping uses to public/file-icons/ (served as they are, never
// inlined into the bundle) and writes the mapping, trimmed to what codec
// looks up, to src/assets/file-icons.json. Run it again to update.

import { copyFileSync, mkdirSync, readFileSync, readdirSync, rmSync, writeFileSync } from 'node:fs'
import { join, resolve } from 'node:path'

const pkg = process.argv[2]
if (!pkg) {
  console.error('usage: node scripts/file-icons.mjs <unpacked material-icon-theme package>')
  process.exit(2)
}
const here = resolve(import.meta.dirname, '..')
const version = JSON.parse(readFileSync(join(pkg, 'package.json'), 'utf8')).version
const m = JSON.parse(readFileSync(join(pkg, 'dist', 'material-icons.json'), 'utf8'))

// folderNamesExpanded is always the folder's icon plus "-open", so only
// the closed names are kept.
const map = {
  version,
  fileExtensions: m.fileExtensions,
  fileNames: m.fileNames,
  folderNames: m.folderNames,
  languageIds: m.languageIds,
  light: {
    fileExtensions: m.light.fileExtensions,
    fileNames: m.light.fileNames,
    folderNames: m.light.folderNames,
  },
}

const used = new Set(['file', 'folder', 'folder-open'])
const add = (names) => Object.values(names).forEach((n) => used.add(n))
for (const t of [map, map.light]) {
  add(t.fileExtensions)
  add(t.fileNames)
  add(t.folderNames)
  Object.values(t.folderNames).forEach((n) => used.add(n + '-open'))
}
add(map.languageIds)

const out = join(here, 'public', 'file-icons')
rmSync(out, { recursive: true, force: true })
mkdirSync(out, { recursive: true })
const have = new Set(readdirSync(join(pkg, 'icons')).map((f) => f.replace(/\.svg$/, '')))
let n = 0
for (const name of used) {
  if (!have.has(name)) continue
  copyFileSync(join(pkg, 'icons', name + '.svg'), join(out, name + '.svg'))
  n++
}
copyFileSync(join(pkg, 'LICENSE'), join(out, 'LICENSE.txt'))
writeFileSync(join(here, 'src', 'assets', 'file-icons.json'), JSON.stringify(map) + '\n')
console.log(`material-icon-theme ${version}: ${n} icons, mapping written`)
