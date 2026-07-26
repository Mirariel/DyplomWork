import type { TradeSummary } from '../api'

export const fmt$ = (v: number) =>
  `$${(v ?? 0).toLocaleString('en-US', { minimumFractionDigits: 2, maximumFractionDigits: 2 })}`

export const fmtDate = (iso: string | null | undefined) => {
  if (!iso) return '\u2014'
  return new Date(iso).toLocaleDateString('en-US', { year: 'numeric', month: 'short', day: 'numeric' })
}

export function parseLeverage(lev: string): number {
  if (!lev) return 0
  const n = parseFloat(lev.replace('x', ''))
  return isNaN(n) ? 0 : n
}

/** Format percentage with sign, e.g. "+12.34%" or "-5.67%" */
export const fmtPct = (v: number) =>
  `${v >= 0 ? '+' : ''}${v.toFixed(2)}%`

/** ROI = pnl / initialMargin × 100. null if margin unknown. */
export function calcRoi(pnl: number, margin: number): number | null {
  if (!margin || margin <= 0) return null
  return (pnl / margin) * 100
}

/** Aggregate Risk/Reward ratio: avgWin / |avgLoss|. null if no losing trades. */
export function calcRR(summary: TradeSummary | null | undefined): number | null {
  if (!summary || !(summary.avg_loss < 0)) return null
  return summary.avg_win / Math.abs(summary.avg_loss)
}
