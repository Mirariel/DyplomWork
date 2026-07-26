import { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { TrendingUp, TrendingDown, RefreshCw, ChevronLeft, ChevronRight, X } from 'lucide-react'
import { getFuturesPositions, getHistory, getSummary, type FuturesPosition, type HistoryEntry, type TradeSummary } from '../api'
import ExchangeSelector from '../components/ExchangeSelector'
import { isLong } from '../lib/side'
import { fmt$, fmtDate, parseLeverage, calcRR } from '../lib/format'

function fmt(n: number, decimals = 2) {
  return n.toLocaleString('en-US', { minimumFractionDigits: decimals, maximumFractionDigits: decimals })
}

/** Compact price: trim trailing zeros but keep at least 2 decimals for small values */
function fmtPrice(n: number) {
  if (n === 0) return '0'
  const abs = Math.abs(n)
  const dec = abs < 1 ? 6 : abs < 100 ? 4 : 2
  const s = n.toLocaleString('en-US', { minimumFractionDigits: 2, maximumFractionDigits: dec })
  // Strip trailing zeros after decimal (keep at least "X.XX")
  return s.replace(/(\.\d{2}\d*?)0+$/, '$1')
}

const OPEN_PAGE_SIZE = 8
const HISTORY_PAGE_SIZE = 10

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

// ─── Shared UI ───────────────────────────────────────────────────────────────

function SideBadge({ side }: { side: string }) {
  return (
    <span className={`inline-block px-2 py-0.5 rounded text-xs font-semibold ${
      isLong(side) ? 'bg-green-500/20 text-green-400' : 'bg-red-500/20 text-red-400'
    }`}>
      {side}
    </span>
  )
}

function PnlBadge({ value }: { value: number }) {
  const positive = value >= 0
  return (
    <span className={`inline-flex items-center gap-1 font-medium ${positive ? 'text-green-400' : 'text-red-400'}`}>
      {positive ? <TrendingUp size={14} /> : <TrendingDown size={14} />}
      {positive ? '+' : ''}{fmt(value)}
    </span>
  )
}

function PnlCell({ value }: { value: number }) {
  return (
    <span className={`font-medium ${value >= 0 ? 'text-green-400' : 'text-red-400'}`}>
      {value >= 0 ? '+' : ''}{fmt$(value)}
    </span>
  )
}

function Pagination({ page, totalPages, total, offset, limit, setOffset }: {
  page: number; totalPages: number; total: number; offset: number; limit: number; setOffset: (fn: (o: number) => number) => void
}) {
  if (totalPages <= 1) return null
  return (
    <div className="flex items-center justify-between px-4 py-2 border-t border-slate-700">
      <p className="text-xs text-slate-400">{page} / {totalPages} ({total})</p>
      <div className="flex gap-2">
        <button disabled={offset === 0} onClick={() => setOffset((o) => Math.max(0, o - limit))}
          className="p-1.5 rounded bg-slate-700 hover:bg-slate-600 disabled:opacity-40 text-slate-300">
          <ChevronLeft size={14} />
        </button>
        <button disabled={offset + limit >= total} onClick={() => setOffset((o) => o + limit)}
          className="p-1.5 rounded bg-slate-700 hover:bg-slate-600 disabled:opacity-40 text-slate-300">
          <ChevronRight size={14} />
        </button>
      </div>
    </div>
  )
}

// Generate array of `size` items, padding with nulls when data is shorter
function padRows<T>(data: T[], size: number): (T | null)[] {
  const result: (T | null)[] = [...data]
  while (result.length < size) result.push(null)
  return result
}

// ─── Open Positions Panel ────────────────────────────────────────────────────

function OpenPositionsPanel() {
  const [exchange, setExchange] = useState('')
  const [offset, setOffset] = useState(0)

  const { data, isLoading, refetch, isFetching } = useQuery({
    queryKey: ['futures-positions', exchange],
    queryFn: () => getFuturesPositions(exchange || undefined),
    refetchInterval: 30_000,
  })

  const positions = data?.positions ?? []
  const totalPnl = data?.total_unrealized_pnl ?? 0

  // Client-side pagination
  const total = positions.length
  const totalPages = Math.ceil(total / OPEN_PAGE_SIZE)
  const page = Math.floor(offset / OPEN_PAGE_SIZE) + 1
  const visible = positions.slice(offset, offset + OPEN_PAGE_SIZE)
  const rows = padRows<FuturesPosition>(visible, OPEN_PAGE_SIZE)

  const handleExchangeChange = (v: string) => { setExchange(v); setOffset(0) }

  return (
    <div className="bg-slate-800 rounded-xl border border-slate-700 flex flex-col">
      {/* Header */}
      <div className="flex items-center justify-between px-4 py-3 border-b border-slate-700">
        <div className="flex items-center gap-3">
          <h2 className="text-sm font-semibold text-white">Open Positions</h2>
          <span className={`text-sm font-bold ${totalPnl >= 0 ? 'text-green-400' : 'text-red-400'}`}>
            {totalPnl >= 0 ? '+' : ''}{fmt(totalPnl)} USDT
          </span>
          <span className="text-xs text-slate-500">{positions.length} pos</span>
        </div>
        <div className="flex items-center gap-2">
          <ExchangeSelector value={exchange} onChange={handleExchangeChange} className="!py-1.5 !text-xs" />
          <button onClick={() => void refetch()} disabled={isFetching}
            className="p-1.5 rounded bg-slate-700 hover:bg-slate-600 text-slate-300 disabled:opacity-50">
            <RefreshCw size={14} className={isFetching ? 'animate-spin' : ''} />
          </button>
        </div>
      </div>

      {/* Table */}
      <div className="flex-1">
        <table className="w-full text-sm">
          <thead>
            <tr className="border-b border-slate-700 text-xs text-slate-400 uppercase tracking-wider">
              <th className="text-left px-3 py-2">Symbol</th>
              <th className="text-center px-2 py-2">Side</th>
              <th className="text-right px-3 py-2">Size</th>
              <th className="text-right px-3 py-2">Entry</th>
              <th className="text-right px-3 py-2">Mark</th>
              <th className="text-right px-3 py-2">PnL</th>
              <th className="text-center px-2 py-2">Margin</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-slate-700/50">
            {isLoading ? (
              <tr><td colSpan={7} className="px-4 py-3 text-center">
                <div className="w-5 h-5 border-2 border-blue-500 border-t-transparent rounded-full animate-spin mx-auto" />
              </td></tr>
            ) : rows.map((p, i) => p ? (
              <tr key={p.id} className="hover:bg-slate-700/30 transition-colors">
                <td className="px-3 py-2">
                  <div className="flex items-center gap-1.5">
                    <ExchangeIcon name={p.exchange} />
                    <div>
                      <div className="font-medium text-white text-sm leading-tight">{p.symbol}</div>
                      <div className="text-[10px] text-slate-500">{p.leverage}x &middot; {p.margin_type}</div>
                    </div>
                  </div>
                </td>
                <td className="px-2 py-2 text-center"><SideBadge side={p.side} /></td>
                <td className="px-3 py-2 text-right text-slate-200 text-xs">{fmt(p.size, 4)}</td>
                <td className="px-3 py-2 text-right text-slate-200 text-xs">{fmtPrice(p.entry_price)}</td>
                <td className="px-3 py-2 text-right text-slate-200 text-xs">{fmtPrice(p.mark_price)}</td>
                <td className="px-3 py-2 text-right"><PnlBadge value={p.unrealized_pnl} /></td>
                <td className="px-2 py-2 text-center text-xs text-slate-400 capitalize">{p.margin_type}</td>
              </tr>
            ) : (
              <tr key={`empty-${i}`}><td colSpan={7} className="px-3 py-2">&nbsp;</td></tr>
            ))}
          </tbody>
        </table>
      </div>

      <Pagination page={page} totalPages={totalPages} total={total}
        offset={offset} limit={OPEN_PAGE_SIZE} setOffset={setOffset} />
    </div>
  )
}

// ─── Position History Panel ──────────────────────────────────────────────────

function HistoryPanel() {
  const [exchange, setExchange] = useState('')
  const [offset, setOffset] = useState(0)

  // Draft / applied pattern: query fires only on GO click
  const [draft, setDraft] = useState({ from: '', to: '', preset: '' })
  const [applied, setApplied] = useState({ from: '', to: '', preset: '' })

  const draftChanged = draft.from !== applied.from || draft.to !== applied.to

  const applyPreset = (label: string, days: number) => {
    const to = new Date()
    const from = new Date()
    from.setDate(from.getDate() - (days - 1))
    const fmt = (d: Date) => d.toISOString().slice(0, 10)
    setDraft({ from: fmt(from), to: fmt(to), preset: label })
  }

  const applyFilters = () => {
    setApplied({ ...draft })
    setOffset(0)
  }

  const clearFilters = () => {
    const empty = { from: '', to: '', preset: '' }
    setDraft(empty)
    setApplied(empty)
    setOffset(0)
  }

  const handleExchangeChange = (v: string) => {
    setExchange(v)
    setOffset(0)
  }

  const { data, isLoading } = useQuery({
    queryKey: ['futures-history', HISTORY_PAGE_SIZE, offset, applied.from, applied.to, exchange],
    queryFn: () => getHistory(HISTORY_PAGE_SIZE, offset, applied.from || undefined, applied.to || undefined, exchange || undefined),
  })

  const { data: summary } = useQuery({
    queryKey: ['futures-summary', applied.from, applied.to, exchange],
    queryFn: () => getSummary(0, exchange, applied.from || undefined, applied.to || undefined),
    staleTime: 60_000,
  })

  const entries: HistoryEntry[] = data?.data ?? []
  const total = data?.total ?? 0
  const totalPages = Math.ceil(total / HISTORY_PAGE_SIZE)
  const page = Math.floor(offset / HISTORY_PAGE_SIZE) + 1
  const rows = padRows<HistoryEntry>(entries, HISTORY_PAGE_SIZE)

  const aggRR = summary ? calcRR(summary) : null

  return (
    <div className="bg-slate-800 rounded-xl border border-slate-700 flex flex-col">
      {/* Header with filters */}
      <div className="px-4 py-3 border-b border-slate-700 space-y-2">
        <div className="flex items-center justify-between">
          <h2 className="text-sm font-semibold text-white">Position History</h2>
          <ExchangeSelector value={exchange} onChange={handleExchangeChange} className="!py-1.5 !text-xs" />
        </div>

        {/* Period filter */}
        <div className="flex items-center gap-2 flex-wrap">
          {[{ label: '1d', days: 1 }, { label: '7d', days: 7 }, { label: '1m', days: 30 }, { label: '3m', days: 90 }].map((p) => (
            <button
              key={p.label}
              onClick={() => applyPreset(p.label, p.days)}
              className={`px-2.5 py-1 text-xs font-medium rounded-lg transition-colors ${
                draft.preset === p.label
                  ? 'bg-blue-600 text-white'
                  : 'bg-slate-700 text-slate-400 hover:bg-slate-600 hover:text-white'
              }`}
            >
              {p.label}
            </button>
          ))}
          <span className="text-slate-700 text-xs">|</span>
          <input type="date" value={draft.from}
            onChange={(e) => setDraft({ ...draft, from: e.target.value, preset: '' })}
            className="text-xs bg-slate-700 border border-slate-600 text-slate-300 rounded-lg px-2 py-1 focus:outline-none focus:border-blue-500" />
          <span className="text-slate-600 text-xs">&mdash;</span>
          <input type="date" value={draft.to}
            onChange={(e) => setDraft({ ...draft, to: e.target.value, preset: '' })}
            className="text-xs bg-slate-700 border border-slate-600 text-slate-300 rounded-lg px-2 py-1 focus:outline-none focus:border-blue-500" />
          <button onClick={applyFilters} disabled={!draftChanged}
            className="px-3 py-1 text-xs font-semibold rounded-lg bg-blue-600 text-white hover:bg-blue-500 disabled:opacity-40 disabled:cursor-not-allowed transition-colors">
            GO
          </button>
          {(applied.from || applied.to) && (
            <button onClick={clearFilters}
              className="text-xs text-slate-500 hover:text-slate-300 p-1 rounded bg-slate-700">
              <X size={11} />
            </button>
          )}
          <span className="ml-auto text-xs text-slate-500">{total} trades</span>
        </div>
      </div>

      {/* Stats bar */}
      {summary && (
        <SummaryBar summary={summary} aggRR={aggRR} />
      )}

      {/* Table */}
      <div className="flex-1">
        <table className="w-full text-sm">
          <thead>
            <tr className="border-b border-slate-700 text-xs text-slate-400 uppercase tracking-wider">
              <th className="text-left px-3 py-2">Symbol</th>
              <th className="text-center px-2 py-2">Side</th>
              <th className="text-right px-3 py-2">Size</th>
              <th className="text-right px-3 py-2">Entry</th>
              <th className="text-right px-3 py-2">Exit</th>
              <th className="text-right px-3 py-2">PnL</th>
              <th className="text-left px-3 py-2">Closed</th>
            </tr>
          </thead>
          <tbody className="divide-y divide-slate-700/50">
            {isLoading ? (
              <tr><td colSpan={7} className="px-4 py-3 text-center">
                <div className="w-5 h-5 border-2 border-blue-500 border-t-transparent rounded-full animate-spin mx-auto" />
              </td></tr>
            ) : rows.map((e, i) => e ? (
              <tr key={e.id} className="hover:bg-slate-700/30 transition-colors">
                <td className="px-3 py-2">
                  <div className="flex items-center gap-1.5">
                    <ExchangeIcon name={e.exchange} />
                    <div>
                      <div className="font-medium text-white text-sm leading-tight">{e.symbol}</div>
                      <div className="text-[10px] text-slate-500">
                        {parseLeverage(e.leverage) > 0 ? e.leverage : e.margin_mode === 'cross' ? 'Cross' : (e.margin_mode || '\u2014')}
                      </div>
                    </div>
                  </div>
                </td>
                <td className="px-2 py-2 text-center"><SideBadge side={e.side} /></td>
                <td className="px-3 py-2 text-right">
                  <div className="text-slate-200 text-xs">{e.max_size > 0 ? fmt$(e.max_size) : <span className="text-slate-600">&mdash;</span>}</div>
                  {e.margin > 0 && <div className="text-[10px] text-slate-500">{fmt$(e.margin)}</div>}
                </td>
                <td className="px-3 py-2 text-right text-slate-300 text-xs">{fmtPrice(e.entry_price)}</td>
                <td className="px-3 py-2 text-right text-slate-300 text-xs">{fmtPrice(e.exit_price)}</td>
                <td className="px-3 py-2 text-right"><PnlCell value={e.realized_pnl} /></td>
                <td className="px-3 py-2 text-slate-300 whitespace-nowrap text-xs">{fmtDate(e.closed_at)}</td>
              </tr>
            ) : (
              <tr key={`empty-h-${i}`}><td colSpan={7} className="px-3 py-2">&nbsp;</td></tr>
            ))}
          </tbody>
        </table>
      </div>

      <Pagination page={page} totalPages={totalPages} total={total}
        offset={offset} limit={HISTORY_PAGE_SIZE} setOffset={setOffset} />
    </div>
  )
}

// ─── Summary Stats Bar ───────────────────────────────────────────────────────

function SummaryBar({ summary, aggRR }: { summary: TradeSummary; aggRR: number | null }) {
  return (
    <div className="flex items-center gap-6 px-4 py-2 border-b border-slate-700 text-xs flex-wrap">
      <div>
        <span className="text-slate-500">PnL </span>
        <span className={`font-semibold ${summary.total_realized_pnl >= 0 ? 'text-green-400' : 'text-red-400'}`}>
          {summary.total_realized_pnl >= 0 ? '+' : ''}{fmt$(summary.total_realized_pnl)}
        </span>
      </div>
      <div>
        <span className="text-slate-500">Trades </span>
        <span className="text-white font-semibold">{summary.total_trades}</span>
      </div>
      <div>
        <span className="text-slate-500">Win </span>
        <span className={`font-semibold ${summary.winrate >= 50 ? 'text-green-400' : 'text-red-400'}`}>
          {summary.winrate.toFixed(1)}%
        </span>
      </div>
      <div>
        <span className="text-slate-500">Avg W </span>
        <span className="text-green-400 font-medium">+{fmt$(summary.avg_win)}</span>
      </div>
      <div>
        <span className="text-slate-500">Avg L </span>
        <span className="text-red-400 font-medium">{fmt$(summary.avg_loss)}</span>
      </div>
      {aggRR !== null && (
        <div>
          <span className="text-slate-500">R:R </span>
          <span className={`font-semibold ${aggRR >= 1.5 ? 'text-green-400' : aggRR >= 1 ? 'text-yellow-400' : 'text-red-400'}`}>
            1:{aggRR.toFixed(2)}
          </span>
        </div>
      )}
      {summary.total_fees > 0 && (
        <div>
          <span className="text-slate-500">Fees </span>
          <span className="text-slate-300">-{fmt$(summary.total_fees)}</span>
        </div>
      )}
      <div>
        <span className="text-slate-500">PF </span>
        <span className={`font-semibold ${summary.profit_factor >= 1.5 ? 'text-green-400' : summary.profit_factor >= 1 ? 'text-yellow-400' : 'text-red-400'}`}>
          {summary.profit_factor.toFixed(2)}
        </span>
      </div>
    </div>
  )
}

// ─── Main Page ───────────────────────────────────────────────────────────────

export default function FuturesPositions() {
  return (
    <div className="space-y-6">
      <div>
        <h1 className="text-2xl font-bold text-white">Futures</h1>
        <p className="text-slate-400 text-sm mt-1">Open positions and closed trade history</p>
      </div>

      <div className="flex flex-col gap-6">
        <OpenPositionsPanel />
        <HistoryPanel />
      </div>
    </div>
  )
}
