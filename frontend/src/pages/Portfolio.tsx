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
  fullSync, getSpotTrades, getSummary,
  type AddCredentialPayload, type Position, type HistoryEntry,
  type Credential, type UserAsset, type SpotTrade,
} from '../api'
import PriceChart from '../components/PriceChart'
import ExchangeSelector from '../components/ExchangeSelector'
import PositionsTable, { type PosRow } from '../components/PositionsTable'
import StatsPanel from '../components/StatsPanel'
import { isLong as isLongSide } from '../lib/side'
import { fmt$ as fmt$Lib, fmtDate as fmtDateLib, fmtPct as fmtPctLib, parseLeverage, calcRoi } from '../lib/format'

// ─── Helpers ──────────────────────────────────────────────────────────────────

const fmt$ = fmt$Lib
const fmtDate = fmtDateLib
const fmtPct = fmtPctLib

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
  return (
    <span className={`text-xs font-semibold px-2 py-0.5 rounded-full ${
      isLongSide(side) ? 'bg-green-900/50 text-green-400' : 'bg-red-900/50 text-red-400'
    }`}>
      {side.toUpperCase()}
    </span>
  )
}

function PnlCell({ pnl, margin }: { pnl: number; margin: number }) {
  const roi = calcRoi(pnl, margin)
  const positive = pnl >= 0
  return (
    <div>
      <div className={`font-medium ${positive ? 'text-green-400' : 'text-red-400'}`}>
        {positive ? '+' : ''}{fmt$(pnl)}
      </div>
      {roi != null && (
        <div className={`text-[10px] ${positive ? 'text-green-500/70' : 'text-red-500/70'}`}>
          {fmtPct(roi)}
        </div>
      )}
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

function BalancesTab({ selectedExchange }: { selectedExchange: string }) {
  const qc = useQueryClient()
  const { data, isLoading } = useQuery({ queryKey: ['portfolio'], queryFn: getPortfolio })
  const assets: UserAsset[] = data?.assets ?? []

  const [detail, setDetail] = useState<UserAsset | null>(null)
  const [editPrice, setEditPrice] = useState<UserAsset | null>(null)

  const filtered = selectedExchange === ALL || !selectedExchange
    ? assets
    : assets.filter((a) => a.exchange === selectedExchange)

  const totalValue = filtered.reduce((s, a) => s + a.quantity * a.current_price, 0)

  if (isLoading) return <Spinner />

  return (
    <div>
      {/* Header row */}
      <div className="flex items-center justify-end px-4 py-3 border-b border-slate-700">
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

function PositionsTab({ selectedExchange }: { selectedExchange: string }) {
  const qc = useQueryClient()
  const { data, isLoading } = useQuery({ queryKey: ['portfolio'], queryFn: getPortfolio, refetchInterval: 10_000 })
  const positions: Position[] = data?.positions ?? []

  const [detail, setDetail] = useState<Position | null>(null)
  const [draftExchange, setDraftExchange] = useState(selectedExchange)
  const [appliedExchange, setAppliedExchange] = useState(selectedExchange)

  const exchangeChanged = draftExchange !== appliedExchange

  const applyExchange = () => setAppliedExchange(draftExchange)
  const clearExchange = () => { setDraftExchange(''); setAppliedExchange('') }

  const filtered = appliedExchange === ALL || !appliedExchange
    ? positions
    : positions.filter((p) => p.exchange === appliedExchange)

  const totalPnl = filtered.reduce((s, p) => s + p.pnl, 0)

  const commentMutation = useMutation({
    mutationFn: ({ id, comment }: { id: number; comment: string }) => updatePositionComment(id, comment),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['portfolio'] }),
  })

  if (isLoading) return <Spinner />

  return (
    <div>
      {/* Header with GO filter */}
      <div className="flex items-center justify-between px-4 py-3 border-b border-slate-700">
        <div className="flex items-center gap-2">
          <ExchangeSelector value={draftExchange} onChange={setDraftExchange} className="!py-1.5 !text-xs" />
          <button onClick={applyExchange} disabled={!exchangeChanged}
            className="px-3 py-1.5 text-xs font-semibold rounded-lg bg-blue-600 text-white hover:bg-blue-500 disabled:opacity-40 disabled:cursor-not-allowed transition-colors">
            GO
          </button>
          {appliedExchange && (
            <button onClick={clearExchange}
              className="text-xs text-slate-500 hover:text-slate-300 p-1 rounded bg-slate-700">
              <X size={11} />
            </button>
          )}
        </div>
        <span className="text-sm text-slate-400">
          Total PnL:{' '}
          <span className={`font-semibold ${totalPnl >= 0 ? 'text-green-400' : 'text-red-400'}`}>
            {totalPnl >= 0 ? '+' : ''}{fmt$(totalPnl)}
          </span>
        </span>
      </div>

      <PositionsTable
        rows={filtered.map((p) => ({
          key: String(p.id),
          symbol: p.symbol,
          leverage: parseLeverage(p.leverage),
          exchange: p.exchange,
          side: p.side,
          tradeSize: p.trade_size,
          margin: p.margin,
          marginMode: p.margin_mode,
          entryPrice: p.entry_price,
          markPrice: p.mark_price,
          pnl: p.pnl,
          comment: p.comment,
          _position: p,
        } as PosRow & { _position: Position }))}
        showComment
        onRowClick={(r) => setDetail((r as PosRow & { _position: Position })._position)}
        commentRenderer={(r) => {
          const p = (r as PosRow & { _position: Position })._position
          return (
            <EditableComment
              value={p.comment ?? ''}
              onSave={(comment) =>
                commentMutation.mutateAsync({ id: p.id, comment }).then(() => undefined)
              }
            />
          )
        }}
        emptyText="No open positions found."
      />

      {detail && (
        <DetailModal item={detail} exchange={detail.exchange} onClose={() => setDetail(null)} />
      )}
    </div>
  )
}

// ─── History tab ──────────────────────────────────────────────────────────────

function HistoryTab() {
  const qc = useQueryClient()
  const [offset, setOffset] = useState(0)
  const [fromDate, setFromDate] = useState('')
  const [toDate, setToDate]   = useState('')
  const LIMIT = 15

  const resetOffset = () => setOffset(0)

  const applyPreset = (days: number) => {
    const to   = new Date()
    const from = new Date()
    from.setDate(from.getDate() - (days - 1))
    const fmt = (d: Date) => d.toISOString().slice(0, 10)
    setFromDate(fmt(from))
    setToDate(fmt(to))
    resetOffset()
  }

  const { data, isLoading } = useQuery({
    queryKey: ['history', LIMIT, offset, fromDate, toDate],
    queryFn: () => getHistory(LIMIT, offset, fromDate || undefined, toDate || undefined),
  })

  const commentMutation = useMutation({
    mutationFn: ({ id, comment }: { id: number; comment: string }) => updateHistoryComment(id, comment),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['history'] }),
  })

  // analytics panel — same date range as table
  const { data: summary } = useQuery({
    queryKey: ['analytics-summary', fromDate, toDate],
    queryFn: () => getSummary(0, '', fromDate || undefined, toDate || undefined),
    staleTime: 60_000,
  })

  const entries: HistoryEntry[] = data?.data ?? []
  const total = data?.total ?? 0
  const totalPages = Math.ceil(total / LIMIT)
  const page = Math.floor(offset / LIMIT) + 1

  if (isLoading) return <Spinner />

  // Per-row computed values
  const rowData = entries.map((e) => {
    const lev = parseLeverage(e.leverage)
    const levLabel = lev > 0
      ? e.leverage
      : e.margin_mode === 'cross' ? 'Cross' : (e.margin_mode || '—')
    const marginModeLabel = e.margin_mode === 'isolated' ? 'Isolated' : 'Cross'
    return { levLabel, marginModeLabel }
  })

  // Aggregate R:R: avgWin / |avgLoss| — matches old project formula
  const aggRR = summary && summary.avg_loss < 0
    ? summary.avg_win / Math.abs(summary.avg_loss)
    : null

  return (
    <div className="flex gap-0">
      {/* Main table */}
      <div className="flex-1 min-w-0">

        {/* Period filter */}
        <div className="flex items-center gap-2 px-4 py-3 border-b border-slate-700 flex-wrap">
          {[{ label: '1d', days: 1 }, { label: '7d', days: 7 }, { label: '1m', days: 30 }, { label: '3m', days: 90 }].map((p) => (
            <button
              key={p.label}
              onClick={() => applyPreset(p.days)}
              className="px-2.5 py-1 text-xs font-medium rounded-lg bg-slate-700 text-slate-400 hover:bg-slate-600 hover:text-white transition-colors"
            >
              {p.label}
            </button>
          ))}
          <span className="text-slate-700 text-xs">|</span>
          <input
            type="date"
            value={fromDate}
            onChange={(e) => { setFromDate(e.target.value); resetOffset() }}
            className="text-xs bg-slate-700 border border-slate-600 text-slate-300 rounded-lg px-2 py-1.5 focus:outline-none focus:border-blue-500"
          />
          <span className="text-slate-600 text-xs">—</span>
          <input
            type="date"
            value={toDate}
            onChange={(e) => { setToDate(e.target.value); resetOffset() }}
            className="text-xs bg-slate-700 border border-slate-600 text-slate-300 rounded-lg px-2 py-1.5 focus:outline-none focus:border-blue-500"
          />
          {(fromDate || toDate) && (
            <button
              onClick={() => { setFromDate(''); setToDate(''); resetOffset() }}
              className="text-xs text-slate-500 hover:text-slate-300 px-2 py-1 rounded bg-slate-700"
            >
              <X size={11} />
            </button>
          )}
          <span className="ml-auto text-xs text-slate-500">{total} угод</span>
        </div>

        <div className="overflow-x-auto">
          <table className="w-full text-sm">
            <thead>
              <tr className="text-xs text-slate-400 border-b border-slate-700">
                {['Символ / Плече', 'Біржа', 'Side', 'Size', 'Тип маржі', 'Вхід', 'Вихід', 'PnL', 'Закрито', 'Коментар'].map((h) => (
                  <th key={h} className="px-4 py-3 text-left font-medium whitespace-nowrap">{h}</th>
                ))}
              </tr>
            </thead>
            <tbody>
              {entries.length === 0 ? (
                <tr>
                  <td colSpan={10} className="px-4 py-6 text-slate-500 text-center text-sm">
                    Немає закритих угод. Запустіть Sync History для імпорту.
                  </td>
                </tr>
              ) : entries.map((e, i) => {
                const { levLabel, marginModeLabel } = rowData[i]
                return (
                  <tr key={e.id} className="border-b border-slate-700/50 hover:bg-slate-700/40 transition-colors">
                    <td className="px-4 py-3">
                      <div className="font-medium text-white">{e.symbol}</div>
                      <div className="text-xs text-slate-500 mt-0.5">{levLabel}</div>
                    </td>
                    <td className="px-4 py-3 text-slate-300 capitalize">{e.exchange}</td>
                    <td className="px-4 py-3"><SideBadge side={e.side} /></td>
                    <td className="px-4 py-3">
                      <div className="text-slate-200">{e.max_size > 0 ? fmt$(e.max_size) : <span className="text-slate-600">—</span>}</div>
                      {e.margin > 0 && <div className="text-xs text-slate-500 mt-0.5">{fmt$(e.margin)}</div>}
                    </td>
                    <td className="px-4 py-3 text-slate-300">{marginModeLabel}</td>
                    <td className="px-4 py-3 text-slate-300">{fmt$(e.entry_price)}</td>
                    <td className="px-4 py-3 text-slate-300">{fmt$(e.exit_price)}</td>
                    <td className="px-4 py-3"><PnlCell pnl={e.realized_pnl} margin={e.margin} /></td>
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
                )
              })}
            </tbody>
          </table>
        </div>

        {totalPages > 1 && (
          <div className="flex items-center justify-between px-4 py-3 border-t border-slate-700">
            <p className="text-xs text-slate-400">Сторінка {page} з {totalPages} ({total} угод)</p>
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

      {/* Analytics sidebar */}
      <div className="w-56 flex-shrink-0 border-l border-slate-700 p-4">
        {summary ? (
          <StatsPanel
            summary={summary}
            aggRR={aggRR}
            title={fromDate || toDate ? 'Статистика за період' : 'Загальна статистика'}
          />
        ) : (
          <div className="text-xs text-slate-600">Завантаження…</div>
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

function SpotTradesTab({ selectedExchange }: { selectedExchange: string }) {
  const { data, isLoading } = useQuery({
    queryKey: ['spot-trades'],
    queryFn: () => getSpotTrades(),
    staleTime: 60_000,
  })
  const trades: SpotTrade[] = data?.trades ?? []

  const filtered = selectedExchange === ALL || !selectedExchange
    ? trades
    : trades.filter((t) => t.exchange === selectedExchange)

  const fmtTs = (ms: number) => {
    if (!ms) return '—'
    return new Date(ms).toLocaleString('en-US', { month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit' })
  }

  if (isLoading) return <Spinner />

  return (
    <div>
      <div className="flex items-center justify-end px-4 py-3 border-b border-slate-700">
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
  const [selectedExchange, setSelectedExchange] = useState('')
  const { data } = useQuery({ queryKey: ['portfolio'], queryFn: getPortfolio })
  const totalValue = data?.total_value ?? 0

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between flex-wrap gap-3">
        <div>
          <h1 className="text-2xl font-bold text-white">Portfolio</h1>
          {totalValue > 0 && (
            <p className="text-slate-400 text-sm mt-0.5">
              Total value: <span className="text-white font-semibold">{fmt$(totalValue)}</span>
            </p>
          )}
        </div>
        {/* Account selector — один для всіх табів */}
        {tab !== 'Credentials' && (
          <ExchangeSelector value={selectedExchange} onChange={setSelectedExchange} />
        )}
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
        {tab === 'Balances'    && <BalancesTab    selectedExchange={selectedExchange} />}
        {tab === 'Positions'   && <PositionsTab   selectedExchange={selectedExchange} />}
        {tab === 'History'     && <HistoryTab />}
        {tab === 'Spot Trades' && <SpotTradesTab  selectedExchange={selectedExchange} />}
        {tab === 'Credentials' && <div className="p-5"><CredentialsTab /></div>}
      </div>
    </div>
  )
}
