// Query's hint for what it found: a word on the results when they are of
// a kind that has a well-known pitfall. Only container images so far:
// one with no tag or the `latest` tag can change under you on the next
// pull. Pure, so the test can run it without the app.

interface Hit {
  path?: string
  value: string
}

// A reference as an image is written: [registry[:port]/]name[:tag][@digest].
const imageRef = /^[a-z0-9][a-z0-9._-]*(:[0-9]+)?(\/[a-z0-9][a-z0-9._-]*)*(:[\w][\w.-]{0,127})?(@sha256:[a-f0-9]{64})?$/

/** The image a hit holds, if the hit is one: a string under an `image`
 *  key, or any string when the expression asks for images. */
function imageOf(h: Hit, askedForImages: boolean): string | null {
  if (!askedForImages && !/\.image$/.test(h.path ?? '')) return null
  const v = h.value.trim().replace(/^(['"])(.*)\1$/, '$2')
  return imageRef.test(v) ? v : null
}

/** An image that a pull can change: no digest, and no tag or `latest`. */
export function unpinned(image: string): boolean {
  if (image.includes('@')) return false
  const name = image.slice(image.lastIndexOf('/') + 1)
  const at = name.indexOf(':')
  return at < 0 || name.slice(at + 1) === 'latest'
}

export function queryHint(expr: string, hits: Hit[]): string {
  const asked = /\bimage\b/.test(expr)
  const images = hits.map((h) => imageOf(h, asked)).filter((i): i is string => i !== null)
  const n = images.filter(unpinned).length
  if (!n) return ''
  return n === 1
    ? "1 image uses latest or no tag. Pin a tag so a pull can't change it."
    : `${n} images use latest or no tag. Pin a tag so a pull can't change them.`
}
