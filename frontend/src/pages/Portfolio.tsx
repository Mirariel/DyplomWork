import { useState, useRef, type FormEvent } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import {
  Trash2, Plus, ChevronLeft, ChevronRight, Check, X,
  TrendingUp, Pencil, ExternalLink,
} from 'lucide-react'
import {
  getPortfolio, getHistory, getCredentials,
  addCredential, deleteCredential,
  updatePositionComment, updateHistoryComment, updateAssetPrice,
  fullSync, getSpotTrades,
  type AddCredentialPayload, type Position, type HistoryEntry,
  type Credential, type UserAsset, type SpotTrade,
} from '../api'
import PriceChart from '../components/PriceChart'

// ─── Helpers ──────────────────────────────────────────────────────────────────

const fmt$ = (v: number) =>
  `$${v.toLocaleString('en-US', { minimumFractionDigits: 2, maximumFractionDigits: 2 })}`

const fmtDate = (iso: string | null | undefined) => {
  if (!iso) return '—'
  return new Date(iso).toLocaleDateString('en-US', { year: 'numeric', month: 'short', day: 'numeric' })
}

const fmtPct = (v: number) => `${v >= 0 ? '+' : ''}${v.toFixed(2)}%`

const ALL = 'all'
const KNOWN_EXCHANGES = ['binance', 'okx', 'bybit', 'kucoin', 'gate', 'kraken']

// ─── Shared UI ────────────────────────────────────────────────────────────────

function Spinner() {
  return (
    <div className="flex justify-center py-10">
      <div className="w-6 h-6 border-2 border-blue-500 border-t-transparent rounded-full animate-spin" />
    </div>
  )
}

function SideBadge({ side }: { side: string }) {
  const s = side.toLowerCase()
  const isLong = s === 'long' || s === 'buy'
  return (
    <span className={`text-xs font-semibold px-2 py-0.5 rounded-full ${
      isLong ? 'bg-green-900/50 text-green-400' : 'bg-red-900/50 text-red-400'
    }`}>
      {side.toUpperCase()}
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

// ─── Exchange filter tabs ─────────────────────────────────────────────────────

function ExchangeTabs({
  exchanges,
  active,
  onChange,
}: {
  exchanges: string[]
  active: string
  onChange: (ex: string) => void
}) {
  const tabs = [ALL, ...exchanges]
  return (
    <div className="flex gap-1 flex-wrap">
      {tabs.map((ex) => (
        <button
          key={ex}
          onClick={() => onChange(ex)}
          className={`px-3 py-1 text-xs font-medium rounded-full transition-colors ${
            active === ex
              ? 'bg-blue-600 text-white'
              : 'bg-slate-700 text-slate-400 hover:text-white'
          }`}
        >
          {ex === ALL ? 'All' : ex.charAt(0).toUpperCase() + ex.slice(1)}
        </button>
      ))}
    </div>
  )
}

// ─── Inline editable comment ──────────────────────────────────────────────────

function EditableComment({ value, onSave }: { value: string; onSave: (v: string) => Promise<void> }) {
  const [editing, setEditing] = useState(false)
  const [text, setText] = useState(value)
  const [saving, setSaving] = useState(false)

  const save = async () => {
    setSaving(true)
    try { await onSave(text); setEditing(false) }
    finally { setSaving(false) }
  }

  if (!editing)
    return (
      <span className="text-slate-400 cursor-pointer hover:text-slate-200 text-xs italic" onClick={() => setEditing(true)}>
        {text || 'Add comment…'}
      </span>
    )

  return (
    <div className="flex items-center gap-1">
      <input
        autoFocus value={text} onChange={(e) => setText(e.target.value)}
        maxLength={150}
        className="bg-slate-700 border border-slate-600 rounded px-2 py-0.5 text-xs text-white focus:outline-none focus:border-blue-500 w-32"
        onKeyDown={(e) => { if (e.key === 'Enter') void save(); if (e.key === 'Escape') setEditing(false) }}
      />
      <button onClick={() => void save()} disabled={saving} className="text-green-400 hover:text-green-300 disabled:opacity-50"><Check size={13} /></button>
      <button onClick={() => setEditing(false)} className="text-slate-400 hover:text-slate-200"><X size={13} /></button>
    </div>
  )
}

// ─── Detail modal ─────────────────────────────────────────────────────────────

function DetailModal({ item, exchange, onClose }: {
  item: UserAsset | Position
  exchange: string
  onClose: () => void
}) {
  const isAsset = 'avg_buy_price' in item
  const symbol = item.symbol.toUpperCase()
  const tvSymbol = `${exchange.toUpperCase()}:${symbol}USDT`

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/70 p-4" onClick={onClose}>
      <div
        className="bg-slate-800 border border-slate-700 rounded-2xl w-full max-w-2xl shadow-2xl overflow-hidden"
        onClick={(e) => e.stopPropagation()}
      >
        {/* Header */}
        <div className="flex items-center justify-between px-6 py-4 border-b border-slate-700">
          <div className="flex items-center gap-3">
            <span className="text-xl font-bold text-white">{symbol}</span>
            <span className="text-sm text-slate-400 capitalize">{exchange}</span>
            {!isAsset && <SideBadge side={(item as Position).side} />}
          </div>
          <div className="flex items-center gap-2">
            <a
              href={`https://www.tradingview.com/chart/?symbol=${tvSymbol}`}
              target="_blank"
              rel="noopener noreferrer"
              className="text-slate-400 hover:text-blue-400 transition-colors"
              title="Open in TradingView"
            >
              <ExternalLink size={16} />
            </a>
            <button onClick={onClose} className="text-slate-400 hover:text-white transition-colors">
              <X size={20} />
            </button>
          </div>
        </div>

        {/* Price chart */}
        <div className="bg-slate-900 px-2 py-2">
          <PriceChart symbol={symbol} height={240} />
        </div>

        {/* Stats */}
        <div className="grid grid-cols-2 sm:grid-cols-3 gap-4 p-6">
          {isAsset ? (
            <>
              <Stat label="Quantity" value={`${(item as UserAsset).quantity}`} />
              <Stat label="Avg Buy Price" value={fmt$((item as UserAsset).avg_buy_price)} />
              <Stat label="Current Price" value={fmt$((item as UserAsset).current_price)} />
              <Stat
                label="Position Value"
                value={fmt$((item as UserAsset).quantity * (item as UserAsset).current_price)}
              />
              {(item as UserAsset).avg_buy_price > 0 && (
                <Stat
                  label="Unrealized PnL"
                  value={fmt$(
                    ((item as UserAsset).current_price - (item as UserAsset).avg_buy_price) *
                      (item as UserAsset).quantity,
                  )}
                  pnl
                />
              )}
            </>
          ) : (
            <>
              <Stat label="Size" value={`${(item as Position).quantity}`} />
              <Stat label="Entry Price" value={fmt$((item as Position).entry_price)} />
              <Stat label="Mark Price" value={fmt$((item as Position).mark_price)} />
              <Stat label="Unrealized PnL" value={fmt$((item as Position).pnl)} pnl />
              <Stat label="Leverage" value={(item as Position).leverage} />
              <Stat label="Margin" value={(item as Position).margin_mode} />
            </>
          )}
        </div>
      </div>
    </div>
  )
}

function Stat({ label, value, pnl = false }: { label: string; value: string; pnl?: boolean }) {
  const numVal = parseFloat(value.replace(/[$,+]/g, ''))
  const color = pnl ? (numVal >= 0 ? 'text-green-400' : 'text-red-400') : 'text-white'
  return (
    <div>
      <p className="text-xs text-slate-500 mb-0.5">{label}</p>
      <p className={`text-sm font-medium ${color}`}>{value}</p>
    </div>
  )
}

// ─── Edit avg_buy_price modal ─────────────────────────────────────────────────

function EditPriceModal({
  asset,
  onClose,
  onSaved,
}: {
  asset: UserAsset
  onClose: () => void
  onSaved: () => void
}) {
  const [price, setPrice] = useState(asset.avg_buy_price > 0 ? String(asset.avg_buy_price) : '')
  const [saving, setSaving] = useState(false)
  const [err, setErr] = useState('')

  const save = async () => {
    const v = parseFloat(price)
    setSaving(true)
    setErr('')
    try {
      await updateAssetPrice(asset.id, isNaN(v) ? 0 : v, !isNaN(v) && v > 0)
      onSaved()
      onClose()
    } catch {
      setErr('Failed to save price')
    } finally {
      setSaving(false)
    }
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/70 p-4" onClick={onClose}>
      <div
        className="bg-slate-800 border border-slate-700 rounded-xl w-full max-w-sm p-6 shadow-2xl"
        onClick={(e) => e.stopPropagation()}
      >
        <h3 className="text-white font-semibold mb-1">Edit Entry Price</h3>
        <p className="text-slate-400 text-sm mb-4">{asset.symbol} · {asset.exchange}</p>

        {err && <p className="text-red-400 text-xs mb-3">{err}</p>}

        <label className="text-xs text-slate-400 mb-1 block">Average Buy Price (USD)</label>
        <input
          autoFocus
          type="number"
          step="any"
          value={price}
          onChange={(e) => setPrice(e.target.value)}
          placeholder="0.00"
          className="w-full bg-slate-700 border border-slate-600 rounded-lg px-3 py-2 text-white text-sm focus:outline-none focus:border-blue-500 mb-4"
          onKeyDown={(e) => { if (e.key === 'Enter') void save() }}
        />
        <p className="text-xs text-slate-500 mb-4">Leave empty to reset (auto-calculate from trades)</p>

        <div className="flex gap-2">
          <button
            onClick={() => void save()} disabled={saving}
            className="flex-1 py-2 bg-blue-600 hover:bg-blue-500 disabled:opacity-50 text-white text-sm font-medium rounded-lg transition-colors"
          >
            {saving ? 'Saving…' : 'Save'}
          </button>
          <button onClick={onClose} className="flex-1 py-2 bg-slate-700 hover:bg-slate-600 text-slate-300 text-sm rounded-lg transition-colors">
            Cancel
          </button>
        </div>
      </div>
    </div>
  )
}

// ─── Balances tab ─────────────────────────────────────────────────────────────

function BalancesTab() {
  const qc = useQueryClient()
  const { data, isLoading } = useQuery({ queryKey: ['portfolio'], queryFn: getPortfolio })
  const assets: UserAsset[] = data?.assets ?? []

  const exchanges = [...new Set(assets.map((a) => a.exchange).filter(Boolean))]
  const [exFilter, setExFilter] = useState(ALL)
  const [detail, setDetail] = useState<UserAsset | null>(null)
  const [editPrice, setEditPrice] = useState<UserAsset | null>(null)

  const filtered = exFilter === ALL ? assets : assets.filter((a) => a.exchange === exFilter)

  const totalValue = filtered.reduce((s, a) => s + a.quantity * a.current_price, 0)

  if (isLoading) return <Spinner />

  return (
    <div>
      {/* Header row */}
      <div className="flex items-center justify-between px-4 py-3 border-b border-slate-700">
        <ExchangeTabs exchanges={exchanges} active={exFilter} onChange={setExFilter} />
        <span className="text-sm text-slate-400">
          Total: <span className="text-white font-semibold">{fmt$(totalValue)}</span>
        </span>
      </div>

      {filtered.length === 0 ? (
        <p className="text-slate-500 text-sm p-6 text-center">No spot balances. Add API keys and run a sync.</p>
      ) : (
        <div className="overflow-x-auto">
          <table className="w-full text-sm">
            <thead>
              <tr className="text-xs text-slate-400 border-b border-slate-700">
                {['Asset', 'Exchange', 'Quantity', 'Avg Buy', 'Price', 'Value', 'PnL', ''].map((h) => (
                  <th key={h} className="px-4 py-3 text-left font-medium whitespace-nowrap">{h}</th>
                ))}
              </tr>
            </thead>
            <tbody>
              {filtered.map((a) => {
                const value = a.quantity * a.current_price
                const pnl = a.avg_buy_price > 0
                  ? (a.current_price - a.avg_buy_price) * a.quantity
                  : null
                const pnlPct = a.avg_buy_price > 0
                  ? ((a.current_price - a.avg_buy_price) / a.avg_buy_price) * 100
                  : null
                return (
                  <tr
                    key={a.id}
                    className="border-b border-slate-700/50 hover:bg-slate-700/40 transition-colors cursor-pointer"
                    onClick={() => setDetail(a)}
                  >
                    <td className="px-4 py-3 font-medium text-white">{a.symbol}</td>
                    <td className="px-4 py-3 text-slate-400 capitalize">{a.exchange}</td>
                    <td className="px-4 py-3 text-slate-300">{a.quantity.toFixed(6)}</td>
                    <td className="px-4 py-3 text-slate-300">
                      {a.avg_buy_price > 0 ? fmt$(a.avg_buy_price) : <span className="text-slate-600">—</span>}
                      {a.manually_set && <span className="ml-1 text-xs text-blue-400" title="Manually set">✎</span>}
                    </td>
                    <td className="px-4 py-3 text-slate-300">{fmt$(a.current_price)}</td>
                    <td className="px-4 py-3 text-slate-200 font-medium">{fmt$(value)}</td>
                    <td className="px-4 py-3">
                      {pnl !== null ? (
                        <span className={`text-xs ${pnl >= 0 ? 'text-green-400' : 'text-red-400'}`}>
                          {pnl >= 0 ? '+' : ''}{fmt$(pnl)} ({fmtPct(pnlPct!)})
                        </span>
                      ) : <span className="text-slate-600 text-xs">—</span>}
                    </td>
                    <td className="px-4 py-3" onClick={(e) => e.stopPropagation()}>
                      <button
                        onClick={() => setEditPrice(a)}
                        className="text-slate-500 hover:text-blue-400 transition-colors p-1"
                        title="Edit entry price"
                      >
                        <Pencil size={13} />
                      </button>
                    </td>
                  </tr>
                )
              })}
            </tbody>
          </table>
        </div>
      )}

      {detail && (
        <DetailModal
          item={detail}
          exchange={detail.exchange}
          onClose={() => setDetail(null)}
        />
      )}
      {editPrice && (
        <EditPriceModal
          asset={editPrice}
          onClose={() => setEditPrice(null)}
          onSaved={() => qc.invalidateQueries({ queryKey: ['portfolio'] })}
        />
      )}
    </div>
  )
}

// ─── Positions tab ─────────────────────────────────────────────────────────────

function PositionsTab() {
  const qc = useQueryClient()
  const { data, isLoading } = useQuery({ queryKey: ['portfolio'], queryFn: getPortfolio })
  const positions: Position[] = data?.positions ?? []

  const exchanges = [...new Set(positions.map((p) => p.exchange).filter(Boolean))]
  const [exFilter, setExFilter] = useState(ALL)
  const [detail, setDetail] = useState<Position | null>(null)

  const filtered = exFilter === ALL ? positions : positions.filter((p) => p.exchange === exFilter)

  const totalPnl = filtered.reduce((s, p) => s + p.pnl, 0)

  const commentMutation = useMutation({
    mutationFn: ({ id, comment }: { id: number; comment: string }) => updatePositionComment(id, comment),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['portfolio'] }),
  })

  if (isLoading) return <Spinner />

  return (
    <div>
      {/* Header */}
      <div className="flex items-center justify-between px-4 py-3 border-b border-slate-700">
        <ExchangeTabs exchanges={exchanges} active={exFilter} onChange={setExFilter} />
        <span className="text-sm text-slate-400">
          Total PnL:{' '}
          <span className={`font-semibold ${totalPnl >= 0 ? 'text-green-400' : 'text-red-400'}`}>
            {totalPnl >= 0 ? '+' : ''}{fmt$(totalPnl)}
          </span>
        </span>
      </div>

      {filtered.length === 0 ? (
        <p className="text-slate-500 text-sm p-6 text-center">No open positions found.</p>
      ) : (
        <div className="overflow-x-auto">
          <table className="w-full text-sm">
            <thead>
              <tr className="text-xs text-slate-400 border-b border-slate-700">
                {['Symbol', 'Exchange', 'Side', 'Size', 'Entry', 'Mark', 'PnL', 'Leverage', 'Comment'].map(
                  (h) => <th key={h} className="px-4 py-3 text-left font-medium whitespace-nowrap">{h}</th>,
                )}
              </tr>
            </thead>
            <tbody>
              {filtered.map((p) => (
                <tr
                  key={p.id}
                  className="border-b border-slate-700/50 hover:bg-slate-700/40 transition-colors cursor-pointer"
                  onClick={() => setDetail(p)}
                >
                  <td className="px-4 py-3 font-medium text-white">{p.symbol}</td>
                  <td className="px-4 py-3 text-slate-300 capitalize">{p.exchange}</td>
                  <td className="px-4 py-3"><SideBadge side={p.side} /></td>
                  <td className="px-4 py-3 text-slate-300">{p.quantity}</td>
                  <td className="px-4 py-3 text-slate-300">{fmt$(p.entry_price)}</td>
                  <td className="px-4 py-3 text-slate-300">{fmt$(p.mark_price)}</td>
                  <td className="px-4 py-3"><PnlCell value={p.pnl} /></td>
                  <td className="px-4 py-3 text-slate-300">{p.leverage}</td>
                  <td className="px-4 py-3" onClick={(e) => e.stopPropagation()}>
                    <EditableComment
                      value={p.comment ?? ''}
                      onSave={(comment) =>
                        commentMutation.mutateAsync({ id: p.id, comment }).then(() => undefined)
                      }
                    />
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      {detail && (
        <DetailModal item={detail} exchange={detail.exchange} onClose={() => setDetail(null)} />
      )}
    </div>
  )
}

// ─── History tab ──────────────────────────────────────────────────────────────

const PRESETS = [
  { label: 'Today', days: 1 },
  { label: '7d', days: 7 },
  { label: '30d', days: 30 },
  { label: 'All', days: 0 },
] as const

function HistoryTab() {
  const qc = useQueryClient()
  const [offset, setOffset] = useState(0)
  const [analyticsDays, setAnalyticsDays] = useState<number>(30)
  const LIMIT = 15

  const { data, isLoading } = useQuery({
    queryKey: ['history', LIMIT, offset],
    queryFn: () => getHistory(LIMIT, offset),
  })

  const { data: summary } = useQuery({
    queryKey: ['analytics-summary', analyticsDays],
    queryFn: () =>
      fetch(`/api/analytics/summary${analyticsDays > 0 ? `?days=${analyticsDays}` : ''}`, {
        headers: { Authorization: `Bearer ${localStorage.getItem('tt_token') ?? ''}` },
      }).then((r) => r.json()),
    staleTime: 60_000,
  })

  const commentMutation = useMutation({
    mutationFn: ({ id, comment }: { id: number; comment: string }) => updateHistoryComment(id, comment),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['history'] }),
  })

  const entries: HistoryEntry[] = data?.data ?? []
  const total = data?.total ?? 0
  const totalPages = Math.ceil(total / LIMIT)
  const page = Math.floor(offset / LIMIT) + 1

  if (isLoading) return <Spinner />

  return (
    <div className="flex gap-0">
      {/* Main table */}
      <div className="flex-1 min-w-0">
        <div className="overflow-x-auto">
          <table className="w-full text-sm">
            <thead>
              <tr className="text-xs text-slate-400 border-b border-slate-700">
                {['Symbol', 'Exchange', 'Side', 'Qty', 'Entry', 'Exit', 'PnL', 'Date', 'Comment'].map((h) => (
                  <th key={h} className="px-4 py-3 text-left font-medium whitespace-nowrap">{h}</th>
                ))}
              </tr>
            </thead>
            <tbody>
              {entries.length === 0 ? (
                <tr>
                  <td colSpan={9} className="px-4 py-6 text-slate-500 text-center text-sm">
                    No trade history found.
                  </td>
                </tr>
              ) : entries.map((e) => (
                <tr key={e.id} className="border-b border-slate-700/50 hover:bg-slate-700/40 transition-colors">
                  <td className="px-4 py-3 font-medium text-white">{e.symbol}</td>
                  <td className="px-4 py-3 text-slate-300 capitalize">{e.exchange}</td>
                  <td className="px-4 py-3"><SideBadge side={e.side} /></td>
                  <td className="px-4 py-3 text-slate-300">{e.quantity}</td>
                  <td className="px-4 py-3 text-slate-300">{fmt$(e.entry_price)}</td>
                  <td className="px-4 py-3 text-slate-300">{fmt$(e.exit_price)}</td>
                  <td className="px-4 py-3"><PnlCell value={e.realized_pnl} /></td>
                  <td className="px-4 py-3 text-slate-300 whitespace-nowrap">{fmtDate(e.closed_at)}</td>
                  <td className="px-4 py-3">
                    <EditableComment
                      value={e.comment ?? ''}
                      onSave={(comment) =>
                        commentMutation.mutateAsync({ id: e.id, comment }).then(() => undefined)
                      }
                    />
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>

        {totalPages > 1 && (
          <div className="flex items-center justify-between px-4 py-3 border-t border-slate-700">
            <p className="text-xs text-slate-400">Page {page} of {totalPages} ({total} trades)</p>
            <div className="flex gap-2">
              <button disabled={offset === 0} onClick={() => setOffset((o) => Math.max(0, o - LIMIT))}
                className="p-1.5 rounded bg-slate-700 hover:bg-slate-600 disabled:opacity-40 text-slate-300">
                <ChevronLeft size={16} />
              </button>
              <button disabled={offset + LIMIT >= total} onClick={() => setOffset((o) => o + LIMIT)}
                className="p-1.5 rounded bg-slate-700 hover:bg-slate-600 disabled:opacity-40 text-slate-300">
                <ChevronRight size={16} />
              </button>
            </div>
          </div>
        )}
      </div>

      {/* Analytics panel */}
      <div className="w-56 flex-shrink-0 border-l border-slate-700 p-4 space-y-4">
        <div>
          <p className="text-xs text-slate-500 mb-2 uppercase tracking-wide">PnL Analytics</p>
          <div className="flex gap-1 flex-wrap">
            {PRESETS.map((p) => (
              <button key={p.label} onClick={() => setAnalyticsDays(p.days)}
                className={`px-2 py-0.5 text-xs rounded-full transition-colors ${
                  analyticsDays === p.days
                    ? 'bg-blue-600 text-white'
                    : 'bg-slate-700 text-slate-400 hover:text-white'
                }`}>
                {p.label}
              </button>
            ))}
          </div>
        </div>

        {summary ? (
          <div className="space-y-3 text-sm">
            <div>
              <p className="text-xs text-slate-500">Total PnL</p>
              <p className={`text-lg font-bold ${(summary.total_realized_pnl ?? 0) >= 0 ? 'text-green-400' : 'text-red-400'}`}>
                {(summary.total_realized_pnl ?? 0) >= 0 ? '+' : ''}{fmt$(summary.total_realized_pnl ?? 0)}
              </p>
            </div>
            <div className="grid grid-cols-2 gap-2 text-xs">
              <div>
                <p className="text-slate-500">Trades</p>
                <p className="text-white font-medium">{summary.total_trades ?? 0}</p>
              </div>
              <div>
                <p className="text-slate-500">Win Rate</p>
                <p className={`font-medium ${(summary.winrate ?? 0) >= 50 ? 'text-green-400' : 'text-red-400'}`}>
                  {(summary.winrate ?? 0).toFixed(1)}%
                </p>
              </div>
              <div>
                <p className="text-slate-500">Best</p>
                <p className="text-green-400 font-medium">{fmt$(summary.best_trade ?? 0)}</p>
              </div>
              <div>
                <p className="text-slate-500">Worst</p>
                <p className="text-red-400 font-medium">{fmt$(summary.worst_trade ?? 0)}</p>
              </div>
            </div>
            {/* Win rate bar */}
            <div>
              <div className="flex justify-between text-xs text-slate-500 mb-1">
                <span>{summary.winning_trades ?? 0}W</span>
                <span>{(summary.total_trades ?? 0) - (summary.winning_trades ?? 0)}L</span>
              </div>
              <div className="h-1.5 bg-slate-700 rounded-full overflow-hidden">
                <div
                  className="h-full bg-green-500 rounded-full"
                  style={{ width: `${Math.min(100, summary.winrate ?? 0)}%` }}
                />
              </div>
            </div>
            <div className="text-xs">
              <p className="text-slate-500">Profit Factor</p>
              <p className={`font-medium ${(summary.profit_factor ?? 0) >= 1.5 ? 'text-green-400' : (summary.profit_factor ?? 0) >= 1 ? 'text-yellow-400' : 'text-red-400'}`}>
                {(summary.profit_factor ?? 0).toFixed(2)}
              </p>
            </div>
          </div>
        ) : (
          <div className="text-xs text-slate-600">Loading stats…</div>
        )}
      </div>
    </div>
  )
}

// ─── Credentials tab ──────────────────────────────────────────────────────────

function CredentialsTab() {
  const qc = useQueryClient()
  const [showForm, setShowForm] = useState(false)
  const [formError, setFormError] = useState('')
  const formRef = useRef<HTMLFormElement>(null)

  const { data: creds = [], isLoading } = useQuery({ queryKey: ['credentials'], queryFn: getCredentials })

  const addMutation = useMutation({
    mutationFn: addCredential,
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: ['credentials'] })
      setShowForm(false)
      formRef.current?.reset()
      setFormError('')
    },
    onError: () => setFormError('Failed to add credential.'),
  })

  const deleteMutation = useMutation({
    mutationFn: deleteCredential,
    onSuccess: () => qc.invalidateQueries({ queryKey: ['credentials'] }),
  })

  const syncMutation = useMutation({
    mutationFn: fullSync,
    onSuccess: () => qc.invalidateQueries({ queryKey: ['portfolio'] }),
  })

  const [exchange, setExchange] = useState('binance')
  const needsPassphrase = exchange === 'okx' || exchange === 'kucoin'

  const handleAdd = (e: FormEvent<HTMLFormElement>) => {
    e.preventDefault()
    const fd = new FormData(e.currentTarget)
    const payload: AddCredentialPayload = {
      exchange: fd.get('exchange') as string,
      label: fd.get('label') as string,
      api_key: fd.get('api_key') as string,
      api_secret: fd.get('api_secret') as string,
    }
    const passphrase = fd.get('passphrase') as string
    if (passphrase) payload.passphrase = passphrase
    addMutation.mutate(payload)
  }

  if (isLoading) return <Spinner />

  return (
    <div className="space-y-4">
      {creds.length === 0 ? (
        <p className="text-slate-500 text-sm py-4">No API credentials added yet.</p>
      ) : (
        <div className="space-y-2">
          {creds.map((c: Credential) => (
            <div key={c.id} className="flex items-center justify-between p-4 bg-slate-700/50 rounded-lg border border-slate-700">
              <div>
                <p className="text-sm font-medium text-white capitalize">
                  {c.exchange}
                  {c.label && <span className="text-slate-400 font-normal"> — {c.label}</span>}
                </p>
                <p className="text-xs text-slate-500 mt-0.5 font-mono">{c.api_key_hint || '••••••••'}</p>
                <p className="text-xs text-slate-600 mt-0.5">
                  Added {fmtDate(c.created_at)}
                  {c.last_sync_at && ` · Last sync ${fmtDate(c.last_sync_at)}`}
                </p>
                {c.last_sync_error && (
                  <p className="text-xs text-red-400 mt-0.5 truncate max-w-xs" title={c.last_sync_error}>
                    ⚠ {c.last_sync_error}
                  </p>
                )}
              </div>
              <button
                onClick={() => deleteMutation.mutate(c.id)}
                disabled={deleteMutation.isPending}
                className="text-slate-400 hover:text-red-400 transition-colors p-1.5"
              >
                <Trash2 size={16} />
              </button>
            </div>
          ))}
        </div>
      )}

      <div className="flex gap-2">
        {!showForm && (
          <button
            onClick={() => setShowForm(true)}
            className="flex items-center gap-2 px-4 py-2 bg-blue-600 hover:bg-blue-500 text-white text-sm font-medium rounded-lg transition-colors"
          >
            <Plus size={15} /> Add API Key
          </button>
        )}
        {creds.length > 0 && (
          <button
            onClick={() => syncMutation.mutate()}
            disabled={syncMutation.isPending}
            className="flex items-center gap-2 px-4 py-2 bg-slate-700 hover:bg-slate-600 disabled:opacity-50 text-slate-300 text-sm rounded-lg transition-colors"
          >
            {syncMutation.isPending ? (
              <div className="w-4 h-4 border-2 border-slate-400 border-t-transparent rounded-full animate-spin" />
            ) : (
              <TrendingUp size={15} />
            )}
            Sync Now
          </button>
        )}
      </div>

      {showForm && (
        <form ref={formRef} onSubmit={handleAdd}
          className="bg-slate-700/40 rounded-xl border border-slate-700 p-5 space-y-3">
          <h3 className="font-medium text-white text-sm">Add API Credential</h3>
          {formError && <p className="text-xs text-red-400 bg-red-900/30 px-3 py-2 rounded">{formError}</p>}
          <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
            <div>
              <label className="label-sm">Exchange</label>
              <select name="exchange" value={exchange} onChange={(e) => setExchange(e.target.value)} className="input-field">
                {KNOWN_EXCHANGES.map((ex) => (
                  <option key={ex} value={ex}>{ex.toUpperCase()}</option>
                ))}
              </select>
            </div>
            <div>
              <label className="label-sm">Label</label>
              <input name="label" required placeholder="My Binance Key" className="input-field" />
            </div>
            <div>
              <label className="label-sm">API Key</label>
              <input name="api_key" required placeholder="API Key" className="input-field" />
            </div>
            <div>
              <label className="label-sm">API Secret</label>
              <input name="api_secret" required type="password" placeholder="API Secret" className="input-field" />
            </div>
            {needsPassphrase && (
              <div>
                <label className="label-sm">Passphrase</label>
                <input name="passphrase" type="password" placeholder="Passphrase" className="input-field" />
              </div>
            )}
          </div>
          <div className="flex gap-2 pt-1">
            <button type="submit" disabled={addMutation.isPending}
              className="px-4 py-2 bg-blue-600 hover:bg-blue-500 disabled:opacity-50 text-white text-sm font-medium rounded-lg transition-colors">
              {addMutation.isPending ? 'Saving…' : 'Save'}
            </button>
            <button type="button" onClick={() => setShowForm(false)}
              className="px-4 py-2 bg-slate-700 hover:bg-slate-600 text-slate-300 text-sm rounded-lg transition-colors">
              Cancel
            </button>
          </div>
        </form>
      )}
    </div>
  )
}

// ─── Spot Trades tab ─────────────────────────────────────────────────────────

function SpotTradesTab() {
  const { data, isLoading } = useQuery({
    queryKey: ['spot-trades'],
    queryFn: () => getSpotTrades(),
    staleTime: 60_000,
  })
  const trades: SpotTrade[] = data?.trades ?? []

  const exchanges = [...new Set(trades.map((t) => t.exchange).filter(Boolean))]
  const [exFilter, setExFilter] = useState(ALL)

  const filtered = exFilter === ALL ? trades : trades.filter((t) => t.exchange === exFilter)

  const fmtTs = (ms: number) => {
    if (!ms) return '—'
    return new Date(ms).toLocaleString('en-US', { month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit' })
  }

  if (isLoading) return <Spinner />

  return (
    <div>
      <div className="flex items-center justify-between px-4 py-3 border-b border-slate-700">
        <ExchangeTabs exchanges={exchanges} active={exFilter} onChange={setExFilter} />
        <span className="text-xs text-slate-500">{filtered.length} trades</span>
      </div>

      {filtered.length === 0 ? (
        <p className="text-slate-500 text-sm p-6 text-center">
          No spot trades found. Run a full sync to import recent trades.
        </p>
      ) : (
        <div className="overflow-x-auto">
          <table className="w-full text-sm">
            <thead>
              <tr className="text-xs text-slate-400 border-b border-slate-700">
                {['Symbol', 'Exchange', 'Side', 'Qty', 'Price', 'Value', 'Fee', 'Date'].map((h) => (
                  <th key={h} className="px-4 py-3 text-left font-medium whitespace-nowrap">{h}</th>
                ))}
              </tr>
            </thead>
            <tbody>
              {filtered.map((t) => (
                <tr key={t.id} className="border-b border-slate-700/50 hover:bg-slate-700/40 transition-colors">
                  <td className="px-4 py-3 font-medium text-white">{t.symbol}</td>
                  <td className="px-4 py-3 text-slate-300 capitalize">{t.exchange}</td>
                  <td className="px-4 py-3"><SideBadge side={t.side} /></td>
                  <td className="px-4 py-3 text-slate-300">{t.quantity}</td>
                  <td className="px-4 py-3 text-slate-300">{fmt$(t.price)}</td>
                  <td className="px-4 py-3 text-slate-200 font-medium">{fmt$(t.quantity * t.price)}</td>
                  <td className="px-4 py-3 text-slate-400">
                    {t.fee > 0 ? `${t.fee.toFixed(6)} ${t.fee_asset}` : '—'}
                  </td>
                  <td className="px-4 py-3 text-slate-400 whitespace-nowrap">{fmtTs(t.traded_at)}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  )
}

// ─── Portfolio page ────────────────────────────────────────────────────────────

const TABS = ['Balances', 'Positions', 'History', 'Spot Trades', 'Credentials'] as const
type Tab = (typeof TABS)[number]

export default function Portfolio() {
  const [tab, setTab] = useState<Tab>('Balances')
  const { data } = useQuery({ queryKey: ['portfolio'], queryFn: getPortfolio })
  const totalValue = data?.total_value ?? 0

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold text-white">Portfolio</h1>
          {totalValue > 0 && (
            <p className="text-slate-400 text-sm mt-0.5">
              Total value: <span className="text-white font-semibold">{fmt$(totalValue)}</span>
            </p>
          )}
        </div>
      </div>

      {/* Tab bar */}
      <div className="flex gap-1 bg-slate-800/50 p-1 rounded-lg border border-slate-700 w-fit">
        {TABS.map((t) => (
          <button key={t} onClick={() => setTab(t)}
            className={`px-4 py-1.5 text-sm font-medium rounded-md transition-colors ${
              tab === t ? 'bg-blue-600 text-white' : 'text-slate-400 hover:text-white'
            }`}>
            {t}
          </button>
        ))}
      </div>

      {/* Content */}
      <div className="bg-slate-800 rounded-xl border border-slate-700 overflow-hidden">
        {tab === 'Balances' && <BalancesTab />}
        {tab === 'Positions' && <PositionsTab />}
        {tab === 'History' && <HistoryTab />}
        {tab === 'Spot Trades' && <SpotTradesTab />}
        {tab === 'Credentials' && <div className="p-5"><CredentialsTab /></div>}
      </div>
    </div>
  )
}
