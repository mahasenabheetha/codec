/**
 * Combine event handlers into one, skipping anything that isn't a
 * function. Used when a component spreads library-provided props (e.g.
 * a tooltip trigger's handlers) and also needs its own handler.
 */
export function chain<E extends Event>(...handlers: unknown[]): (e: E) => void {
  return (e: E) => {
    for (const h of handlers) if (typeof h === 'function') h(e)
  }
}
