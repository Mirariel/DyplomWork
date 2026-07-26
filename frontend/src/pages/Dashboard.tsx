import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import {
  LineChart,
  Line,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip,
  ResponsiveContainer,
} from 'recharts'
import { RefreshCw, Wifi, WifiOff, Search, CalendarDays, X } from 'lucide-react'
import { getSummary, getPortfolioChart, fullSync, getPortfolio, getTopSymbols, type ChartPoint } from '../api'
import ExchangeSelector from '../components/ExchangeSelector'
import { useWebSocket } from '../ws'
import CoinModal from '../components/CoinModal'
import { isLong } from '../lib/side'

// ─── Helpers ──────────────────────────────────────────────────────────────────

const fmt$ = (v: number) =>
  `$${v.toLocaleString('en-US', { minimumFractionDigits: 2, maximumFractionDigits: 2 })}`

const fmtPct = (v: number) => `${v.toFixed(2)}%`

// ─── Stat card ────────────────────────────────────────────────────────────────

function StatCard({
  label,
  value,
  sub,
  positive,
}: {
  label: string
  value: string
  sub?: string
  positive?: boolean
}) {
  return (
    <div className="bg-slate-800 rounded-xl p-5 border border-slate-700">
      <p className="text-xs font-medium text-slate-400 uppercase tracking-wider">{label}</p>
      <p
        className={`mt-2 text-2xl font-bold ${
          positive === undefined
            ? 'text-white'
            : positive
            ? 'text-green-400'
            : 'text-red-400'
        }`}
      >
        {value}
      </p>
      {sub && <p className="mt-1 text-xs text-slate-500">{sub}</p>}
    </div>
  )
}

// ─── Period options ───────────────────────────────────────────────────────────

type PeriodKey = '1d' | '7d' | '30d' | '90d' | 'all' | 'custom'

const PERIOD_OPTIONS: { label: string; key: PeriodKey }[] = [
  { label: '1d',    key: '1d' },
  { label: '7d',    key: '7d' },
  { label: '30d',   key: '30d' },
  { label: '90d',   key: '90d' },
  { label: 'All',   key: 'all' },
  { label: 'Custom',key: 'custom' },
]

// Convert period key to days for summary stats
function periodToDays(key: PeriodKey): number {
  switch (key) {
    case '1d':  return 1
    case '7d':  return 7
    case '30d': return 30
    case '90d': return 90
    default:    return 0
  }
}

// Downsample chart data to at most maxPoints points
function downsample(data: ChartPoint[], maxPoints: number): ChartPoint[] {
  if (data.length <= maxPoints) return data
  const step = Math.ceil(data.length / maxPoints)
  const result = data.filter((_, i) => i % step === 0)
  if (result[result.length - 1] !== data[data.length - 1]) {
    result.push(data[data.length - 1])
  }
  return result
}

// Format recorded_at label based on the selected period
function fmtChartLabel(isoStr: string, key: PeriodKey): string {
  const d = new Date(isoStr)
  if (key === '1d') {
    return d.toLocaleTimeString('en-US', { hour: '2-digit', minute: '2-digit', hour12: false })
  }
  if (key === '7d') {
    return d.toLocaleDateString('en-US', { month: 'short', day: 'numeric' }) +
      ' ' + d.toLocaleTimeString('en-US', { hour: '2-digit', minute: '2-digit', hour12: false })
  }
  return d.toLocaleDateString('en-US', { month: 'short', day: 'numeric' })
}

// ─── Dashboard ────────────────────────────────────────────────────────────────

export default function Dashboard() {
  const qc = useQueryClient()
  const { positions, spotPrices, spotPricesByExchange, topSymbols: wsTopSymbols, connected } = useWebSocket()
  const [syncError, setSyncError] = useState('')
  const [selectedExchange, setSelectedExchange] = useState('')
  const [selectedPeriod, setSelectedPeriod] = useState<PeriodKey>('30d')
  const [customFrom, setCustomFrom] = useState('')
  const [customTo, setCustomTo]   = useState('')
  const [coinSearch, setCoinSearch] = useState('')
  const [selectedCoin, setSelectedCoin] = useState<{ symbol: string; price: number } | null>(null)

  const { data: portfolio } = useQuery({
    queryKey: ['portfolio'],
    queryFn: getPortfolio,
  })

  const summaryDays = periodToDays(selectedPeriod)
  const { data: summary } = useQuery({
    queryKey: ['summary', summaryDays, selectedExchange],
    queryFn: () => getSummary(summaryDays, selectedExchange),
  })

  const { data: chartPoints = [] } = useQuery({
    queryKey: ['chart', selectedPeriod, selectedExchange, customFrom, customTo],
    queryFn: () =>
      selectedPeriod === 'custom'
        ? getPortfolioChart('7d', selectedExchange || 'all', customFrom, customTo)
        : getPortfolioChart(selectedPeriod, selectedExchange || 'all'),
    enabled: selectedPeriod !== 'custom' || (customFrom !== '' && customTo !== ''),
  })

  // HTTP fallback для топ символів (якщо WS ще не підключений)
  const { data: httpTopSymbols = [] } = useQuery({
    queryKey: ['top-symbols'],
    queryFn: getTopSymbols,
    staleTime: 60 * 60 * 1000, // 1h — збігається з TTL на бекенді
  })

  // Пріоритет: WS (fresh, кожні 2с) → HTTP fallback → порожній список
  const topSymbols = wsTopSymbols.length > 0 ? wsTopSymbols : httpTopSymbols

  const syncMutation = useMutation({
    mutationFn: fullSync,
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: ['summary'] })
      void qc.invalidateQueries({ queryKey: ['chart'] })
      void qc.invalidateQueries({ queryKey: ['portfolio'] })
      setSyncError('')
    },
    onError: () => setSyncError('Sync failed. Make sure your API credentials are configured.'),
  })

  // ── Portfolio Value ─────────────────────────────────────────────────────────
  // Spot assets: quantity × current_price (filtered by exchange if selected)
  const allAssets = portfolio?.assets ?? []
  const filteredAssets = selectedExchange
    ? allAssets.filter((a) => a.exchange === selectedExchange)
    : allAssets
  const spotValue = filteredAssets.reduce((acc, a) => acc + a.quantity * a.current_price, 0)

  // Futures: unrealized PnL from WS (filtered by exchange)
  const filteredPositions = selectedExchange
    ? positions.filter((p) => p.exchange === selectedExchange)
    : positions
  const unrealizedPnl = filteredPositions.reduce((acc, p) => acc + p.pnl, 0)

  const totalValue = spotValue
  const openCount = filteredPositions.length
  const totalPnl = summary?.total_realized_pnl ?? 0
  const winRate = summary?.winrate ?? 0

  // ── Live Spot Prices ────────────────────────────────────────────────────────
  const filteredPriceMap =
    selectedExchange && spotPricesByExchange[selectedExchange]
      ? spotPricesByExchange[selectedExchange]
      : spotPrices

  const topSymbolSet = new Set(topSymbols)
  const searchQuery  = coinSearch.toUpperCase().trim()

  // Всі валідні пари (базовий токен, не починається з цифри, є ціна)
  const allPriceEntries = Object.entries(filteredPriceMap).filter(
    ([sym, price]) =>
      price > 0 &&
      sym.length >= 2 &&
      !/^\d/.test(sym) &&
      !sym.endsWith('USDT') &&
      !sym.endsWith('USDC'),
  )

  // Search: фільтр + сортування за ціною
  // Default (no search): топ символи у порядку рейтингу
  const displayedPrices: Array<[string, number, boolean]> = searchQuery
    ? allPriceEntries
        .filter(([sym]) => sym.includes(searchQuery))
        .sort(([, a], [, b]) => b - a)
        .slice(0, 20)
        .map(([sym, price]) => [sym, price, topSymbolSet.has(sym)] as [string, number, boolean])
    : topSymbols.length > 0
      ? topSymbols
          .map((sym) => [sym, filteredPriceMap[sym] ?? 0, true] as [string, number, boolean])
          .filter(([, price]) => price > 0)
      : allPriceEntries
          .sort(([, a], [, b]) => b - a)
          .slice(0, 20)
          .map(([sym, price]) => [sym, price, false] as [string, number, boolean])

  // ── Chart ───────────────────────────────────────────────────────────────────
  // Downsample: 1d→96 pts (15-min), 7d→168 pts (hourly), else→180 pts
  const maxPoints = selectedPeriod === '1d' ? 96 : selectedPeriod === '7d' ? 168 : 180
  const sampledPoints = downsample(chartPoints, maxPoints)
  const chartData = sampledPoints.map((p) => ({
    date: fmtChartLabel(p.recorded_at, selectedPeriod),
    value: p.value,
  }))

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between flex-wrap gap-3">
        <div>
          <h1 className="text-2xl font-bold text-white">Dashboard</h1>
          <p className="text-sm text-slate-400 mt-0.5">
            {connected ? (
              <span className="flex items-center gap-1.5 text-green-400">
                <Wifi size={13} /> Live
              </span>
            ) : (
              <span className="flex items-center gap-1.5 text-slate-500">
                <WifiOff size={13} /> Connecting…
              </span>
            )}
          </p>
        </div>

        {/* Global filters + sync */}
        <div className="flex items-center gap-2 flex-wrap">
          {/* Account / exchange selector */}
          <ExchangeSelector value={selectedExchange} onChange={setSelectedExchange} />

          {/* Period selector */}
          <div className="flex bg-slate-700 border border-slate-600 rounded-lg overflow-hidden">
            {PERIOD_OPTIONS.map((opt) => (
              <button
                key={opt.key}
                onClick={() => setSelectedPeriod(opt.key)}
                className={`px-3 py-2 text-xs font-medium transition-colors ${
                  selectedPeriod === opt.key
                    ? 'bg-blue-600 text-white'
                    : 'text-slate-400 hover:text-white hover:bg-slate-600'
                }`}
              >
                {opt.label}
              </button>
            ))}
          </div>

          {/* Custom date range */}
          {selectedPeriod === 'custom' && (
            <div className="flex items-center gap-2">
              <CalendarDays size={13} className="text-slate-400" />
              <input
                type="date"
                value={customFrom ? customFrom.slice(0, 10) : ''}
                onChange={(e) => setCustomFrom(e.target.value ? e.target.value + 'T00:00:00Z' : '')}
                className="text-xs bg-slate-700 border border-slate-600 text-slate-300 rounded-lg px-2 py-2 focus:outline-none focus:border-blue-500"
              />
              <span className="text-slate-500 text-xs">—</span>
              <input
                type="date"
                value={customTo ? customTo.slice(0, 10) : ''}
                onChange={(e) => setCustomTo(e.target.value ? e.target.value + 'T23:59:59Z' : '')}
                className="text-xs bg-slate-700 border border-slate-600 text-slate-300 rounded-lg px-2 py-2 focus:outline-none focus:border-blue-500"
              />
              {(customFrom || customTo) && (
                <button
                  onClick={() => { setCustomFrom(''); setCustomTo('') }}
                  className="text-slate-400 hover:text-white"
                >
                  <X size={13} />
                </button>
              )}
            </div>
          )}

          {syncError && <p className="text-xs text-red-400">{syncError}</p>}
          <button
            onClick={() => syncMutation.mutate()}
            disabled={syncMutation.isPending}
            className="flex items-center gap-2 px-4 py-2 bg-blue-600 hover:bg-blue-500 disabled:opacity-50 text-white text-sm font-medium rounded-lg transition-colors"
          >
            <RefreshCw size={15} className={syncMutation.isPending ? 'animate-spin' : ''} />
            Refresh now
          </button>
        </div>
      </div>

      {/* Stats */}
      <div className="grid grid-cols-2 lg:grid-cols-4 gap-4">
        <StatCard
          label="Portfolio Value"
          value={fmt$(totalValue)}
          sub={unrealizedPnl !== 0 ? `Futures PnL: ${unrealizedPnl >= 0 ? '+' : ''}${fmt$(unrealizedPnl)}` : undefined}
        />
        <StatCard
          label="Total Realized PnL"
          value={fmt$(totalPnl)}
          positive={totalPnl >= 0}
          sub={summaryDays > 0 ? `Last ${summaryDays}d` : 'All time'}
        />
        <StatCard
          label="Win Rate"
          value={fmtPct(winRate)}
          sub={`${summary?.winning_trades ?? 0}W / ${summary?.losing_trades ?? 0}L`}
        />
        <StatCard label="Open Positions" value={String(openCount)} />
      </div>

      {/* Two-column grid: prices + chart */}
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
        {/* Live spot prices */}
        <div className="bg-slate-800 rounded-xl border border-slate-700 overflow-hidden">
          <div className="px-5 py-4 border-b border-slate-700 space-y-3">
            <div className="flex items-center justify-between gap-2">
              <h2 className="font-semibold text-white text-sm">
                Live Spot Prices
                {selectedExchange && (
                  <span className="ml-2 text-xs font-normal text-slate-400 capitalize">
                    ({selectedExchange})
                  </span>
                )}
              </h2>
              {!coinSearch && topSymbols.length > 0 && (
                <span className="text-xs text-yellow-400/70 flex items-center gap-1">
                  <span>★</span> Top 20 by volume
                </span>
              )}
            </div>
            {/* Search */}
            <div className="relative">
              <Search size={13} className="absolute left-3 top-1/2 -translate-y-1/2 text-slate-400" />
              <input
                type="text"
                value={coinSearch}
                onChange={(e) => setCoinSearch(e.target.value)}
                placeholder="Search coin…"
                className="w-full bg-slate-700 border border-slate-600 text-slate-200 text-xs rounded-lg pl-8 pr-3 py-2 focus:outline-none focus:border-blue-500 placeholder:text-slate-500"
              />
            </div>
          </div>
          <div className="overflow-x-auto">
            {displayedPrices.length === 0 ? (
              <p className="px-5 py-6 text-sm text-slate-500">
                {coinSearch ? `No results for "${coinSearch}"` : 'No price data yet.'}
              </p>
            ) : (
              <table className="w-full text-sm">
                <thead>
                  <tr className="text-xs text-slate-400 border-b border-slate-700">
                    <th className="px-5 py-3 text-left font-medium">Symbol</th>
                    <th className="px-5 py-3 text-right font-medium">Price</th>
                  </tr>
                </thead>
                <tbody>
                  {displayedPrices.map(([symbol, price, isTop]) => (
                    <tr
                      key={symbol}
                      onClick={() => setSelectedCoin({ symbol, price })}
                      className="border-b border-slate-700/50 hover:bg-slate-700/40 transition-colors cursor-pointer"
                    >
                      <td className="px-5 py-3 font-medium text-white">
                        <span className="flex items-center gap-1.5">
                          {isTop && <span className="text-yellow-400 text-xs leading-none">★</span>}
                          {symbol}
                        </span>
                      </td>
                      <td className="px-5 py-3 text-right text-slate-300">{fmt$(price)}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            )}
          </div>
        </div>

        {/* Portfolio value chart */}
        <div className="bg-slate-800 rounded-xl border border-slate-700 p-5">
          <h2 className="font-semibold text-white text-sm mb-4">
            Portfolio Value
            {selectedPeriod === 'custom' && customFrom && customTo
              ? ` (${customFrom.slice(0,10)} – ${customTo.slice(0,10)})`
              : ` (${PERIOD_OPTIONS.find(p => p.key === selectedPeriod)?.label ?? ''})`}
          </h2>
          {chartData.length < 2 ? (
            <div className="flex flex-col items-center justify-center h-48 gap-2 text-center px-4">
              <p className="text-slate-400 text-sm font-medium">Not enough data yet</p>
              {chartData.length === 0
                ? <p className="text-slate-500 text-xs">Click «Refresh now» to record your first snapshot. Chart populates automatically.</p>
                : <p className="text-slate-500 text-xs">One more snapshot needed — chart updates automatically.</p>
              }
            </div>
          ) : (
            <ResponsiveContainer width="100%" height={220}>
              <LineChart data={chartData}>
                <CartesianGrid strokeDasharray="3 3" stroke="#334155" />
                <XAxis
                  dataKey="date"
                  tick={{ fill: '#94a3b8', fontSize: 11 }}
                  tickLine={false}
                  axisLine={false}
                />
                <YAxis
                  tick={{ fill: '#94a3b8', fontSize: 11 }}
                  tickLine={false}
                  axisLine={false}
                  tickFormatter={(v: number | string) => {
                    const n = Number(v)
                    return n >= 1000 ? `$${(n / 1000).toFixed(1)}k` : `$${n.toFixed(0)}`
                  }}
                />
                <Tooltip
                  contentStyle={{
                    backgroundColor: '#1e293b',
                    border: '1px solid #334155',
                    borderRadius: '8px',
                    color: '#e2e8f0',
                  }}
                  formatter={(v: number | string) => [fmt$(Number(v)), 'Value']}
                />
                <Line
                  type="monotone"
                  dataKey="value"
                  stroke="#3b82f6"
                  strokeWidth={2}
                  dot={false}
                  activeDot={{ r: 4, fill: '#3b82f6' }}
                />
              </LineChart>
            </ResponsiveContainer>
          )}
        </div>
      </div>

      {/* Open positions table */}
      <div className="bg-slate-800 rounded-xl border border-slate-700 overflow-hidden">
        <div className="px-5 py-4 border-b border-slate-700">
          <h2 className="font-semibold text-white text-sm">
            Open Positions ({openCount})
            {selectedExchange && (
              <span className="ml-2 text-xs font-normal text-slate-400 capitalize">
                — {selectedExchange}
              </span>
            )}
          </h2>
        </div>
        {filteredPositions.length === 0 ? (
          <p className="px-5 py-6 text-sm text-slate-500">No open positions.</p>
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full text-sm">
              <thead>
                <tr className="text-xs text-slate-400 border-b border-slate-700">
                  <th className="px-5 py-3 text-left font-medium">Symbol</th>
                  <th className="px-5 py-3 text-left font-medium">Side</th>
                  <th className="px-5 py-3 text-left font-medium">Exchange</th>
                  <th className="px-5 py-3 text-right font-medium">Entry Price</th>
                  <th className="px-5 py-3 text-right font-medium">Mark Price</th>
                  <th className="px-5 py-3 text-right font-medium">PnL</th>
                  <th className="px-5 py-3 text-right font-medium">PnL %</th>
                </tr>
              </thead>
              <tbody>
                {filteredPositions.map((p, i) => {
                  const pos = p.pnl >= 0
                  return (
                    <tr
                      key={`${p.symbol}-${p.exchange}-${i}`}
                      className="border-b border-slate-700/50 hover:bg-slate-700/40 transition-colors"
                    >
                      <td className="px-5 py-3 font-medium text-white">{p.symbol}</td>
                      <td className="px-5 py-3">
                        <span
                          className={`text-xs font-medium px-2 py-0.5 rounded-full ${
                            isLong(p.side)
                              ? 'bg-green-900/50 text-green-400'
                              : 'bg-red-900/50 text-red-400'
                          }`}
                        >
                          {p.side.toUpperCase()}
                        </span>
                      </td>
                      <td className="px-5 py-3 text-slate-400 capitalize">{p.exchange}</td>
                      <td className="px-5 py-3 text-right text-slate-300">
                        {fmt$(p.entry_price)}
                      </td>
                      <td className="px-5 py-3 text-right text-slate-300">
                        {fmt$(p.mark_price)}
                      </td>
                      <td
                        className={`px-5 py-3 text-right font-medium ${
                          pos ? 'text-green-400' : 'text-red-400'
                        }`}
                      >
                        {pos ? '+' : ''}
                        {fmt$(p.pnl)}
                      </td>
                      <td
                        className={`px-5 py-3 text-right font-medium ${
                          pos ? 'text-green-400' : 'text-red-400'
                        }`}
                      >
                        {pos ? '+' : ''}
                        {fmtPct(p.pnl_pct)}
                      </td>
                    </tr>
                  )
                })}
              </tbody>
            </table>
          </div>
        )}
      </div>

      {/* Coin detail modal */}
      {selectedCoin && (
        <CoinModal
          symbol={selectedCoin.symbol}
          price={selectedCoin.price}
          onClose={() => setSelectedCoin(null)}
        />
      )}
    </div>
  )
}
