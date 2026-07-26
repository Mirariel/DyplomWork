import type { TradeSummary } from '../api'

export const fmt$ = (v: number) =>
  `$${v.toLocaleString('en-US', { minimumFractionDigits: 2, maximumFractionDigits: 2 })}`

export const fmtDate = (iso: string | null | undefined) => {
  if (!iso) return '\u2014'
  return new Date(iso).toLocaleDateString('en-US', { year: 'numeric', month: 'short', day: 'numeric' })
}

export function parseLeverage(lev: string): number {
  if (!lev) return 0
  const n = parseFloat(lev.replace('x', ''))
  return isNaN(n) ? 0 : n
}

/** Aggregate Risk/Reward ratio: avgWin / |avgLoss|. null if no losing trades. */
export function calcRR(summary: TradeSummary): number | null {
  return summary.avg_loss < 0
    ? summary.avg_win / Math.abs(summary.avg_loss)
    : null
}
