// Colour arithmetic for the Colour tool (3.8.0), all in the browser:
// reading a colour written any common CSS way, writing it in every
// format, WCAG 2 contrast, OKLCH, and scales of shades. sRGB throughout.

/** A colour: red, green and blue 0–255, alpha 0–1. */
export interface RGB {
  r: number
  g: number
  b: number
  a: number
}

const clamp = (n: number, lo: number, hi: number) => Math.min(hi, Math.max(lo, n))
const round = (n: number, d = 0) => Math.round(n * 10 ** d) / 10 ** d

/** Reads #rgb, #rgba, #rrggbb, #rrggbbaa (the # optional), rgb()/rgba(),
 *  hsl()/hsla(), hsv()/hsb(), oklch() and the CSS colour names. */
export function parseColour(input: string): RGB | null {
  const s = input.trim().toLowerCase()
  if (!s) return null
  if (s in NAMES) return hex(NAMES[s])
  if (s === 'transparent') return { r: 0, g: 0, b: 0, a: 0 }
  const h = hex(s)
  if (h) return h
  const m = s.match(/^(rgba?|hsla?|hsv|hsb|oklch)\(\s*(.*?)\s*\)$/)
  if (!m) return null
  // Comma or space separated, an optional "/ alpha".
  const parts = m[2].split(/\s*[,/]\s*|\s+/).filter(Boolean)
  if (parts.length < 3 || parts.length > 4) return null
  const num = (p: string, scale: number) => (p.endsWith('%') ? (parseFloat(p) / 100) * scale : parseFloat(p))
  const alpha = parts[3] === undefined ? 1 : clamp(num(parts[3], 1), 0, 1)
  if ([0, 1, 2].some((i) => Number.isNaN(parseFloat(parts[i]))) || Number.isNaN(alpha)) return null
  const hue = (p: string) => (p.endsWith('turn') ? parseFloat(p) * 360 : p.endsWith('rad') ? (parseFloat(p) * 180) / Math.PI : parseFloat(p))
  switch (m[1]) {
    case 'rgb':
    case 'rgba':
      return { r: clamp(num(parts[0], 255), 0, 255), g: clamp(num(parts[1], 255), 0, 255), b: clamp(num(parts[2], 255), 0, 255), a: alpha }
    case 'hsl':
    case 'hsla':
      return { ...fromHSL(hue(parts[0]), clamp(num(parts[1], 100), 0, 100), clamp(num(parts[2], 100), 0, 100)), a: alpha }
    case 'hsv':
    case 'hsb':
      return { ...fromHSV(hue(parts[0]), clamp(num(parts[1], 100), 0, 100), clamp(num(parts[2], 100), 0, 100)), a: alpha }
    case 'oklch':
      return { ...fromOKLCH(clamp(num(parts[0], 1), 0, 1), Math.max(0, num(parts[1], 0.4)), hue(parts[2])), a: alpha }
  }
  return null
}

function hex(s: string): RGB | null {
  const m = s.replace(/^#/, '').match(/^([0-9a-f]{3,4}|[0-9a-f]{6}|[0-9a-f]{8})$/)
  if (!m) return null
  let x = m[1]
  if (x.length <= 4) x = [...x].map((c) => c + c).join('')
  const v = (i: number) => parseInt(x.slice(i, i + 2), 16)
  return { r: v(0), g: v(2), b: v(4), a: x.length === 8 ? round(v(6) / 255, 3) : 1 }
}

export function fromHSL(h: number, s: number, l: number): Omit<RGB, 'a'> {
  h = ((h % 360) + 360) % 360
  s /= 100
  l /= 100
  const k = (n: number) => (n + h / 30) % 12
  const a = s * Math.min(l, 1 - l)
  const f = (n: number) => l - a * Math.max(-1, Math.min(k(n) - 3, 9 - k(n), 1))
  return { r: f(0) * 255, g: f(8) * 255, b: f(4) * 255 }
}

export function fromHSV(h: number, s: number, v: number): Omit<RGB, 'a'> {
  h = ((h % 360) + 360) % 360
  s /= 100
  v /= 100
  const f = (n: number) => {
    const k = (n + h / 60) % 6
    return v - v * s * Math.max(0, Math.min(k, 4 - k, 1))
  }
  return { r: f(5) * 255, g: f(3) * 255, b: f(1) * 255 }
}

/** Hue (degrees), saturation and lightness (percent). */
export function toHSL({ r, g, b }: RGB): [number, number, number] {
  r /= 255
  g /= 255
  b /= 255
  const max = Math.max(r, g, b)
  const min = Math.min(r, g, b)
  const l = (max + min) / 2
  const d = max - min
  if (!d) return [0, 0, l * 100]
  const s = d / (1 - Math.abs(2 * l - 1))
  const h = max === r ? ((g - b) / d) % 6 : max === g ? (b - r) / d + 2 : (r - g) / d + 4
  return [(h * 60 + 360) % 360, s * 100, l * 100]
}

/** Hue (degrees), saturation and value (percent). */
export function toHSV({ r, g, b }: RGB): [number, number, number] {
  r /= 255
  g /= 255
  b /= 255
  const max = Math.max(r, g, b)
  const d = max - Math.min(r, g, b)
  const h = !d ? 0 : max === r ? ((g - b) / d) % 6 : max === g ? (b - r) / d + 2 : (r - g) / d + 4
  return [(h * 60 + 360) % 360, max ? (d / max) * 100 : 0, max * 100]
}

// sRGB ⇄ linear light.
const lin = (c: number) => (c <= 0.04045 ? c / 12.92 : ((c + 0.055) / 1.055) ** 2.4)
const gam = (c: number) => (c <= 0.0031308 ? c * 12.92 : 1.055 * c ** (1 / 2.4) - 0.055)

/** OKLCH: lightness 0–1, chroma, hue in degrees (Björn Ottosson's Oklab). */
export function toOKLCH({ r, g, b }: RGB): [number, number, number] {
  const [R, G, B] = [r, g, b].map((c) => lin(c / 255))
  const l = Math.cbrt(0.4122214708 * R + 0.5363325363 * G + 0.0514459929 * B)
  const m = Math.cbrt(0.2119034982 * R + 0.6806995451 * G + 0.1073969566 * B)
  const s = Math.cbrt(0.0883024619 * R + 0.2817188376 * G + 0.6299787005 * B)
  const L = 0.2104542553 * l + 0.793617785 * m - 0.0040720468 * s
  const A = 1.9779984951 * l - 2.428592205 * m + 0.4505937099 * s
  const Bk = 0.0259040371 * l + 0.7827717662 * m - 0.808675766 * s
  const C = Math.hypot(A, Bk)
  return [L, C, C < 1e-4 ? 0 : ((Math.atan2(Bk, A) * 180) / Math.PI + 360) % 360]
}

export function fromOKLCH(L: number, C: number, H: number): Omit<RGB, 'a'> {
  const A = C * Math.cos((H * Math.PI) / 180)
  const B = C * Math.sin((H * Math.PI) / 180)
  const l = (L + 0.3963377774 * A + 0.2158037573 * B) ** 3
  const m = (L - 0.1055613458 * A - 0.0638541728 * B) ** 3
  const s = (L - 0.0894841775 * A - 1.291485548 * B) ** 3
  const R = 4.0767416621 * l - 3.3077115913 * m + 0.2309699292 * s
  const G = -1.2684380046 * l + 2.6097574011 * m - 0.3413193965 * s
  const Bl = -0.0041960863 * l - 0.7034186147 * m + 1.707614701 * s
  return { r: clamp(gam(R), 0, 1) * 255, g: clamp(gam(G), 0, 1) * 255, b: clamp(gam(Bl), 0, 1) * 255 }
}

/** Every common way to write c, for copying. */
export function formats(c: RGB): { label: string; value: string }[] {
  const [r, g, b] = [c.r, c.g, c.b].map((v) => Math.round(v))
  const a = round(c.a, 3)
  const [h, s, l] = toHSL(c)
  const [hv, sv, vv] = toHSV(c)
  const [L, C, H] = toOKLCH(c)
  const alpha = a < 1 ? ` / ${a}` : ''
  return [
    { label: 'HEX', value: toHex(c) },
    { label: 'RGB', value: a < 1 ? `rgb(${r} ${g} ${b} / ${a})` : `rgb(${r}, ${g}, ${b})` },
    { label: 'HSL', value: a < 1 ? `hsl(${round(h)} ${round(s)}% ${round(l)}%${alpha})` : `hsl(${round(h)}, ${round(s)}%, ${round(l)}%)` },
    { label: 'HSV', value: `hsv(${round(hv)}, ${round(sv)}%, ${round(vv)}%)` },
    { label: 'OKLCH', value: `oklch(${round(L * 100, 1)}% ${round(C, 3)} ${round(H, 1)}${alpha})` },
    { label: 'Name', value: nameOf(c) },
  ]
}

export function toHex(c: RGB): string {
  const x = (v: number) => Math.round(v).toString(16).padStart(2, '0')
  return '#' + x(c.r) + x(c.g) + x(c.b) + (c.a < 1 ? x(c.a * 255) : '')
}

/** The CSS name for c, or the nearest one (by OKLab distance) with "≈". */
export function nameOf(c: RGB): string {
  const h = toHex({ ...c, a: 1 })
  let best = ''
  let dist = Infinity
  const [L1, C1, H1] = toOKLCH(c)
  const lab = (L: number, C: number, H: number) => [L, C * Math.cos((H * Math.PI) / 180), C * Math.sin((H * Math.PI) / 180)]
  const p = lab(L1, C1, H1)
  for (const [name, v] of Object.entries(NAMES)) {
    if (v === h.slice(1)) return name
    const q = lab(...toOKLCH(hex(v)!))
    const d = Math.hypot(p[0] - q[0], p[1] - q[1], p[2] - q[2])
    if (d < dist) [best, dist] = [name, d]
  }
  return `≈ ${best}`
}

/** WCAG 2 relative luminance. */
export function luminance({ r, g, b }: RGB): number {
  return 0.2126 * lin(r / 255) + 0.7152 * lin(g / 255) + 0.0722 * lin(b / 255)
}

/** Colour fg drawn over bg (for a translucent foreground). */
export function over(fg: RGB, bg: RGB): RGB {
  const a = fg.a
  return { r: fg.r * a + bg.r * (1 - a), g: fg.g * a + bg.g * (1 - a), b: fg.b * a + bg.b * (1 - a), a: 1 }
}

/** WCAG 2 contrast ratio, 1 to 21; a translucent foreground is laid over
 *  the background first. */
export function contrast(fg: RGB, bg: RGB): number {
  const l1 = luminance(over(fg, { ...bg, a: 1 }))
  const l2 = luminance(bg)
  return (Math.max(l1, l2) + 0.05) / (Math.min(l1, l2) + 0.05)
}

/** The WCAG 2 checks a ratio passes. */
export function wcag(ratio: number) {
  return [
    { name: 'Text AA', need: 4.5, pass: ratio >= 4.5 },
    { name: 'Text AAA', need: 7, pass: ratio >= 7 },
    { name: 'Large text AA', need: 3, pass: ratio >= 3, title: '24 px, or 19 px bold, and up' },
    { name: 'Large text AAA', need: 4.5, pass: ratio >= 4.5, title: '24 px, or 19 px bold, and up' },
    { name: 'Icons and borders', need: 3, pass: ratio >= 3, title: 'User-interface components and graphics (AA)' },
  ]
}

/** A scale like Tailwind's 50–950: the colour's hue and chroma at
 *  lightness steps in OKLCH, which look evenly spaced; chroma eases off
 *  at the ends so the lightest and darkest stay in gamut. */
export function scale(c: RGB): { step: number; colour: RGB }[] {
  const [, C, H] = toOKLCH(c)
  const steps: [number, number][] = [
    [50, 0.97],
    [100, 0.93],
    [200, 0.87],
    [300, 0.79],
    [400, 0.7],
    [500, 0.62],
    [600, 0.54],
    [700, 0.46],
    [800, 0.38],
    [900, 0.3],
    [950, 0.22],
  ]
  return steps.map(([step, L]) => {
    const ease = 1 - Math.abs(L - 0.6) / 0.6
    return { step, colour: { ...fromOKLCH(L, C * Math.max(0.15, ease), H), a: 1 } }
  })
}

/** The CSS Color 4 named colours, as hex without #. */
export const NAMES: Record<string, string> = {
  aliceblue: 'f0f8ff', antiquewhite: 'faebd7', aqua: '00ffff', aquamarine: '7fffd4', azure: 'f0ffff',
  beige: 'f5f5dc', bisque: 'ffe4c4', black: '000000', blanchedalmond: 'ffebcd', blue: '0000ff',
  blueviolet: '8a2be2', brown: 'a52a2a', burlywood: 'deb887', cadetblue: '5f9ea0', chartreuse: '7fff00',
  chocolate: 'd2691e', coral: 'ff7f50', cornflowerblue: '6495ed', cornsilk: 'fff8dc', crimson: 'dc143c',
  cyan: '00ffff', darkblue: '00008b', darkcyan: '008b8b', darkgoldenrod: 'b8860b', darkgray: 'a9a9a9',
  darkgreen: '006400', darkgrey: 'a9a9a9', darkkhaki: 'bdb76b', darkmagenta: '8b008b', darkolivegreen: '556b2f',
  darkorange: 'ff8c00', darkorchid: '9932cc', darkred: '8b0000', darksalmon: 'e9967a', darkseagreen: '8fbc8f',
  darkslateblue: '483d8b', darkslategray: '2f4f4f', darkslategrey: '2f4f4f', darkturquoise: '00ced1', darkviolet: '9400d3',
  deeppink: 'ff1493', deepskyblue: '00bfff', dimgray: '696969', dimgrey: '696969', dodgerblue: '1e90ff',
  firebrick: 'b22222', floralwhite: 'fffaf0', forestgreen: '228b22', fuchsia: 'ff00ff', gainsboro: 'dcdcdc',
  ghostwhite: 'f8f8ff', gold: 'ffd700', goldenrod: 'daa520', gray: '808080', green: '008000',
  greenyellow: 'adff2f', grey: '808080', honeydew: 'f0fff0', hotpink: 'ff69b4', indianred: 'cd5c5c',
  indigo: '4b0082', ivory: 'fffff0', khaki: 'f0e68c', lavender: 'e6e6fa', lavenderblush: 'fff0f5',
  lawngreen: '7cfc00', lemonchiffon: 'fffacd', lightblue: 'add8e6', lightcoral: 'f08080', lightcyan: 'e0ffff',
  lightgoldenrodyellow: 'fafad2', lightgray: 'd3d3d3', lightgreen: '90ee90', lightgrey: 'd3d3d3', lightpink: 'ffb6c1',
  lightsalmon: 'ffa07a', lightseagreen: '20b2aa', lightskyblue: '87cefa', lightslategray: '778899', lightslategrey: '778899',
  lightsteelblue: 'b0c4de', lightyellow: 'ffffe0', lime: '00ff00', limegreen: '32cd32', linen: 'faf0e6',
  magenta: 'ff00ff', maroon: '800000', mediumaquamarine: '66cdaa', mediumblue: '0000cd', mediumorchid: 'ba55d3',
  mediumpurple: '9370db', mediumseagreen: '3cb371', mediumslateblue: '7b68ee', mediumspringgreen: '00fa9a', mediumturquoise: '48d1cc',
  mediumvioletred: 'c71585', midnightblue: '191970', mintcream: 'f5fffa', mistyrose: 'ffe4e1', moccasin: 'ffe4b5',
  navajowhite: 'ffdead', navy: '000080', oldlace: 'fdf5e6', olive: '808000', olivedrab: '6b8e23',
  orange: 'ffa500', orangered: 'ff4500', orchid: 'da70d6', palegoldenrod: 'eee8aa', palegreen: '98fb98',
  paleturquoise: 'afeeee', palevioletred: 'db7093', papayawhip: 'ffefd5', peachpuff: 'ffdab9', peru: 'cd853f',
  pink: 'ffc0cb', plum: 'dda0dd', powderblue: 'b0e0e6', purple: '800080', rebeccapurple: '663399',
  red: 'ff0000', rosybrown: 'bc8f8f', royalblue: '4169e1', saddlebrown: '8b4513', salmon: 'fa8072',
  sandybrown: 'f4a460', seagreen: '2e8b57', seashell: 'fff5ee', sienna: 'a0522d', silver: 'c0c0c0',
  skyblue: '87ceeb', slateblue: '6a5acd', slategray: '708090', slategrey: '708090', snow: 'fffafa',
  springgreen: '00ff7f', steelblue: '4682b4', tan: 'd2b48c', teal: '008080', thistle: 'd8bfd8',
  tomato: 'ff6347', turquoise: '40e0d0', violet: 'ee82ee', wheat: 'f5deb3', white: 'ffffff',
  whitesmoke: 'f5f5f5', yellow: 'ffff00', yellowgreen: '9acd32',
}
