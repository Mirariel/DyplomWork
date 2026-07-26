/** Case-insensitive check: "long" or "buy" → true */
export function isLong(side: string): boolean {
  const s = side.toLowerCase()
  return s === 'long' || s === 'buy'
}
