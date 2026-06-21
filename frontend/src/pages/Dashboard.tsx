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
import { RefreshCw, Wifi, WifiOff } from 'lucide-react'
import { getSummary, getSnapshots, fullSync } from '../api'
import { useWebSocket } from '../ws'

// ─── Helpers ──────────────────────────────────────────────────────────────────

const fmt$ = (v: number) =>
  `$${v.toLocaleString('en-US', { minimumFractionDigits: 2, maximumFractionDigits: 2 })}`

const fmtPct = (v: number) => `${v.toFixed(2)}%`

const fmtDate = (iso: string) =>
  new Date(iso).toLocaleDateString('en-US', { month: 'short', day: 'numeric' })

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

// ─── Dashboard ────────────────────────────────────────────────────────────────

export default function Dashboard() {
  const qc = useQueryClient()
  const { positions, spotPrices, connected } = useWebSocket()
  const [syncError, setSyncError] = useState('')

  const { data: summary } = useQuery({
    queryKey: ['summary'],
    queryFn: getSummary,
  })

  const { data: snapshots = [] } = useQuery({
    queryKey: ['snapshots', 30],
    queryFn: () => getSnapshots(30),
  })

  const syncMutation = useMutation({
    mutationFn: fullSync,
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: ['summary'] })
      void qc.invalidateQueries({ queryKey: ['snapshots'] })
      setSyncError('')
    },
    onError: () => setSyncError('Sync failed. Make sure your API credentials are configured.'),
  })

  // Compute total portfolio value from WS positions
  const totalValue = positions.reduce(
    (acc, p) => acc + p.quantity * (spotPrices[p.symbol] ?? p.mark_price),
    0,
  )

  const openCount = positions.length
  const totalPnl = summary?.total_realized_pnl ?? 0
  const winRate = summary?.winrate ?? 0

  // Top 10 spot prices
  const topPrices = Object.entries(spotPrices).slice(0, 10)

  // Chart data
  const chartData = snapshots.map((s) => ({
    date: fmtDate(s.snapshot_date),
    value: s.total_value,
  }))

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
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
        <div className="flex items-center gap-3">
          {syncError && <p className="text-xs text-red-400">{syncError}</p>}
          <button
            onClick={() => syncMutation.mutate()}
            disabled={syncMutation.isPending}
            className="flex items-center gap-2 px-4 py-2 bg-blue-600 hover:bg-blue-500 disabled:opacity-50 text-white text-sm font-medium rounded-lg transition-colors"
          >
            <RefreshCw
              size={15}
              className={syncMutation.isPending ? 'animate-spin' : ''}
            />
            Sync All
          </button>
        </div>
      </div>

      {/* Stats */}
      <div className="grid grid-cols-2 lg:grid-cols-4 gap-4">
        <StatCard label="Portfolio Value" value={fmt$(totalValue)} />
        <StatCard
          label="Total Realized PnL"
          value={fmt$(totalPnl)}
          positive={totalPnl >= 0}
        />
        <StatCard label="Win Rate" value={fmtPct(winRate)} />
        <StatCard label="Open Positions" value={String(openCount)} />
      </div>

      {/* Two-column grid: prices + chart */}
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
        {/* Live spot prices */}
        <div className="bg-slate-800 rounded-xl border border-slate-700 overflow-hidden">
          <div className="px-5 py-4 border-b border-slate-700">
            <h2 className="font-semibold text-white text-sm">Live Spot Prices</h2>
          </div>
          <div className="overflow-x-auto">
            {topPrices.length === 0 ? (
              <p className="px-5 py-6 text-sm text-slate-500">
                No price data yet. Connect to WebSocket or sync.
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
                  {topPrices.map(([symbol, price]) => (
                    <tr
                      key={symbol}
                      className="border-b border-slate-700/50 hover:bg-slate-700/40 transition-colors"
                    >
                      <td className="px-5 py-3 font-medium text-white">{symbol}</td>
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
          <h2 className="font-semibold text-white text-sm mb-4">Portfolio Value (30d)</h2>
          {chartData.length === 0 ? (
            <div className="flex items-center justify-center h-48 text-slate-500 text-sm">
              No snapshot data yet.
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
                  tickFormatter={(v: number | string) => `$${(Number(v) / 1000).toFixed(0)}k`}
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
          </h2>
        </div>
        {positions.length === 0 ? (
          <p className="px-5 py-6 text-sm text-slate-500">No open positions.</p>
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full text-sm">
              <thead>
                <tr className="text-xs text-slate-400 border-b border-slate-700">
                  <th className="px-5 py-3 text-left font-medium">Symbol</th>
                  <th className="px-5 py-3 text-left font-medium">Side</th>
                  <th className="px-5 py-3 text-right font-medium">Entry Price</th>
                  <th className="px-5 py-3 text-right font-medium">Mark Price</th>
                  <th className="px-5 py-3 text-right font-medium">PnL</th>
                  <th className="px-5 py-3 text-right font-medium">PnL %</th>
                </tr>
              </thead>
              <tbody>
                {positions.map((p, i) => {
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
                            p.side === 'long'
                              ? 'bg-green-900/50 text-green-400'
                              : 'bg-red-900/50 text-red-400'
                          }`}
                        >
                          {p.side.toUpperCase()}
                        </span>
                      </td>
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
    </div>
  )
}
