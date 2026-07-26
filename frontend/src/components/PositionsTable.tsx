import { isLong } from '../lib/side'
import { fmt$, fmtPct, calcRoi } from '../lib/format'

// ─── Common row type ─────────────────────────────────────────────────────────

export interface PosRow {
  key: string
  symbol: string
  leverage: number
  exchange: string
  side: string
  tradeSize: number
  margin: number
  marginMode: string
  entryPrice: number
  markPrice: number
  pnl: number
  /** If pre-computed (e.g. from WS) */
  pnlPct?: number
  comment?: string | null
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

function fmtPrice(n: number) {
  if (n === 0) return '0'
  const abs = Math.abs(n)
  const dec = abs < 1 ? 6 : abs < 100 ? 4 : 2
  const s = n.toLocaleString('en-US', { minimumFractionDigits: 2, maximumFractionDigits: dec })
  return s.replace(/(\.\d{2}\d*?)0+$/, '$1')
}

function fmtCompact(n: number) {
  if (n === 0) return '$0'
  return '$' + n.toLocaleString('en-US', { minimumFractionDigits: 2, maximumFractionDigits: 2 })
}

const EXCHANGE_ICONS: Record<string, string> = {
  okx: 'OKX',
  binance: 'BIN',
  bybit: 'BYB',
}

function ExchangeIcon({ name }: { name: string }) {
  const label = EXCHANGE_ICONS[name.toLowerCase()] ?? name.slice(0, 3).toUpperCase()
  const colors: Record<string, string> = {
    okx: 'bg-white/10 text-white',
    binance: 'bg-yellow-500/15 text-yellow-400',
    bybit: 'bg-orange-500/15 text-orange-400',
  }
  const cls = colors[name.toLowerCase()] ?? 'bg-slate-600 text-slate-300'
  return (
    <span className={`inline-block px-1.5 py-0.5 rounded text-[10px] font-bold tracking-wide ${cls}`}>
      {label}
    </span>
  )
}

function SideBadge({ side }: { side: string }) {
  return (
    <span className={`inline-block px-2 py-0.5 rounded text-xs font-semibold ${
      isLong(side) ? 'bg-green-500/20 text-green-400' : 'bg-red-500/20 text-red-400'
    }`}>
      {side.toUpperCase()}
    </span>
  )
}

// ─── Component ───────────────────────────────────────────────────────────────

interface Props {
  rows: PosRow[]
  /** Show comment column (Portfolio only) */
  showComment?: boolean
  onRowClick?: (row: PosRow) => void
  emptyText?: string
  commentRenderer?: (row: PosRow) => React.ReactNode
}

export default function PositionsTable({ rows, showComment, onRowClick, emptyText, commentRenderer }: Props) {
  if (rows.length === 0) {
    return <p className="text-slate-500 text-sm p-6 text-center">{emptyText ?? 'No open positions.'}</p>
  }

  return (
    <div className="overflow-x-auto">
      <table className="w-full text-sm">
        <thead>
          <tr className="border-b border-slate-700 text-xs text-slate-400 uppercase tracking-wider">
            <th className="text-left px-3 py-2">Symbol / Lev</th>
            <th className="text-left px-3 py-2">Exchange</th>
            <th className="text-center px-2 py-2">Side</th>
            <th className="text-right px-3 py-2">Size</th>
            <th className="text-center px-2 py-2">Margin Type</th>
            <th className="text-right px-3 py-2">Entry</th>
            <th className="text-right px-3 py-2">Mark</th>
            <th className="text-right px-3 py-2">PnL</th>
            {showComment && <th className="text-left px-3 py-2">Comment</th>}
          </tr>
        </thead>
        <tbody className="divide-y divide-slate-700/50">
          {rows.map((r) => {
            const roi = r.pnlPct != null ? r.pnlPct : calcRoi(r.pnl, r.margin)
            const positive = r.pnl >= 0
            return (
              <tr
                key={r.key}
                className={`hover:bg-slate-700/30 transition-colors ${onRowClick ? 'cursor-pointer' : ''}`}
                onClick={onRowClick ? () => onRowClick(r) : undefined}
              >
                {/* Symbol + Leverage · MarginMode */}
                <td className="px-3 py-2">
                  <div className="font-medium text-white text-sm leading-tight">{r.symbol}</div>
                  <div className="text-[10px] text-slate-500">
                    {[r.leverage > 0 && `${r.leverage}x`, r.marginMode].filter(Boolean).join(' · ')}
                  </div>
                </td>

                {/* Exchange */}
                <td className="px-3 py-2">
                  <ExchangeIcon name={r.exchange} />
                </td>

                {/* Side */}
                <td className="px-2 py-2 text-center">
                  <SideBadge side={r.side} />
                </td>

                {/* Size: trade_size + margin below */}
                <td className="px-3 py-2 text-right">
                  <div className="text-slate-200 text-xs">{fmtCompact(r.tradeSize)}</div>
                  {r.margin > 0 && (
                    <div className="text-[10px] text-slate-500">margin {fmtCompact(r.margin)}</div>
                  )}
                </td>

                {/* Margin Type */}
                <td className="px-2 py-2 text-center text-xs text-slate-400 capitalize">
                  {r.marginMode || '\u2014'}
                </td>

                {/* Entry */}
                <td className="px-3 py-2 text-right text-slate-200 text-xs">{fmtPrice(r.entryPrice)}</td>

                {/* Mark */}
                <td className="px-3 py-2 text-right text-slate-200 text-xs">{fmtPrice(r.markPrice)}</td>

                {/* PnL: $ amount + ROI% */}
                <td className="px-3 py-2 text-right">
                  <div className={`font-medium ${positive ? 'text-green-400' : 'text-red-400'}`}>
                    {positive ? '+' : ''}{fmt$(r.pnl)}
                  </div>
                  {roi != null && (
                    <div className={`text-[10px] ${positive ? 'text-green-500/70' : 'text-red-500/70'}`}>
                      {fmtPct(roi)}
                    </div>
                  )}
                </td>

                {/* Comment */}
                {showComment && (
                  <td className="px-3 py-2" onClick={(e) => e.stopPropagation()}>
                    {commentRenderer ? commentRenderer(r) : (r.comment || '\u2014')}
                  </td>
                )}
              </tr>
            )
          })}
        </tbody>
      </table>
    </div>
  )
}
