import { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { RefreshCw } from 'lucide-react'
import { getSummary, getCoins, getArbitrage } from '../api'

// ─── Helpers ──────────────────────────────────────────────────────────────────

const fmt$ = (v: number) =>
  `$${v.toLocaleString('en-US', { minimumFractionDigits: 2, maximumFractionDigits: 2 })}`

const fmtPct = (v: number) => `${v.toFixed(2)}%`

// ─── Stat card ────────────────────────────────────────────────────────────────

function StatCard({
  label,
  value,
  positive,
}: {
  label: string
  value: string
  positive?: boolean
}) {
  return (
    <div className="bg-slate-700/50 rounded-xl p-4 border border-slate-700">
      <p className="text-xs font-medium text-slate-400 uppercase tracking-wider">{label}</p>
      <p
        className={`mt-1.5 text-xl font-bold ${
          positive === undefined
            ? 'text-white'
            : positive
            ? 'text-green-400'
            : 'text-red-400'
        }`}
      >
        {value}
      </p>
    </div>
  )
}

function Spinner() {
  return (
    <div className="flex justify-center py-10">
      <div className="w-6 h-6 border-2 border-blue-500 border-t-transparent rounded-full animate-spin" />
    </div>
  )
}

// ─── Trade summary ────────────────────────────────────────────────────────────

function TradeSummarySection() {
  const { data, isLoading } = useQuery({
    queryKey: ['summary'],
    queryFn: getSummary,
  })

  if (isLoading) return <Spinner />
  if (!data)
    return <p className="text-slate-500 text-sm">No summary data available.</p>

  return (
    <div className="grid grid-cols-2 sm:grid-cols-3 lg:grid-cols-4 gap-3">
      <StatCard label="Total Trades" value={String(data.total_trades)} />
      <StatCard label="Win Rate" value={fmtPct(data.winrate)} />
      <StatCard label="Total PnL" value={fmt$(data.total_realized_pnl)} positive={data.total_realized_pnl >= 0} />
      <StatCard label="Avg PnL" value={fmt$(data.avg_pnl)} positive={data.avg_pnl >= 0} />
      <StatCard label="Best Trade" value={fmt$(data.best_trade)} positive />
      <StatCard label="Worst Trade" value={fmt$(data.worst_trade)} positive={false} />
      <StatCard
        label="Profit Factor"
        value={data.profit_factor.toFixed(2)}
        positive={data.profit_factor >= 1}
      />
      <StatCard label="Winners / Losers" value={`${data.winning_trades} / ${data.losing_trades}`} />
    </div>
  )
}

// ─── Coin performance ──────────────────────────────────────────────────────────

function CoinPerformanceSection() {
  const { data: coins = [], isLoading } = useQuery({
    queryKey: ['coins'],
    queryFn: getCoins,
  })

  if (isLoading) return <Spinner />

  // Sort by total PnL desc
  const sorted = [...coins].sort((a, b) => b.total_pnl - a.total_pnl)

  return (
    <div className="overflow-x-auto">
      <table className="w-full text-sm">
        <thead>
          <tr className="text-xs text-slate-400 border-b border-slate-700">
            {['Symbol', 'Exchange', 'Trades', 'Win Rate', 'Total PnL', 'Avg PnL', 'Best', 'Worst'].map(
              (h) => (
                <th key={h} className="px-4 py-3 text-left font-medium whitespace-nowrap">
                  {h}
                </th>
              ),
            )}
          </tr>
        </thead>
        <tbody>
          {sorted.length === 0 ? (
            <tr>
              <td colSpan={8} className="px-4 py-6 text-slate-500 text-center">
                No coin performance data yet.
              </td>
            </tr>
          ) : (
            sorted.map((c) => (
              <tr
                key={`${c.symbol}-${c.exchange}`}
                className="border-b border-slate-700/50 hover:bg-slate-700/40 transition-colors"
              >
                <td className="px-4 py-3 font-medium text-white">{c.symbol}</td>
                <td className="px-4 py-3 text-slate-300 capitalize">{c.exchange}</td>
                <td className="px-4 py-3 text-slate-300">{c.total_trades}</td>
                <td className="px-4 py-3 text-slate-300">{fmtPct(c.winrate)}</td>
                <td
                  className={`px-4 py-3 font-medium ${
                    c.total_pnl >= 0 ? 'text-green-400' : 'text-red-400'
                  }`}
                >
                  {c.total_pnl >= 0 ? '+' : ''}
                  {fmt$(c.total_pnl)}
                </td>
                <td
                  className={`px-4 py-3 ${
                    c.avg_pnl >= 0 ? 'text-green-400' : 'text-red-400'
                  }`}
                >
                  {c.avg_pnl >= 0 ? '+' : ''}
                  {fmt$(c.avg_pnl)}
                </td>
                <td className="px-4 py-3 text-green-400">+{fmt$(c.best_trade)}</td>
                <td className="px-4 py-3 text-red-400">{fmt$(c.worst_trade)}</td>
              </tr>
            ))
          )}
        </tbody>
      </table>
    </div>
  )
}

// ─── Arbitrage scanner ────────────────────────────────────────────────────────

function ArbitrageSection() {
  const [minSpread, setMinSpread] = useState<number | undefined>(undefined)

  const {
    data: opps = [],
    isLoading,
    refetch,
    isFetching,
  } = useQuery({
    queryKey: ['arbitrage', minSpread],
    queryFn: () => getArbitrage(minSpread),
    staleTime: 0,
  })

  const spreadColor = (pct: number) => {
    if (pct >= 1) return 'text-green-400'
    if (pct >= 0.5) return 'text-yellow-400'
    return 'text-slate-400'
  }

  return (
    <div className="space-y-3">
      <div className="flex items-center gap-3 flex-wrap">
        <div className="flex items-center gap-2">
          <label className="text-xs text-slate-400">Min spread %:</label>
          <input
            type="number"
            step="0.1"
            min="0"
            placeholder="0"
            value={minSpread ?? ''}
            onChange={(e) =>
              setMinSpread(e.target.value ? parseFloat(e.target.value) : undefined)
            }
            className="w-20 px-2 py-1 bg-slate-700 border border-slate-600 rounded text-sm text-white focus:outline-none focus:border-blue-500"
          />
        </div>
        <button
          onClick={() => void refetch()}
          disabled={isFetching}
          className="flex items-center gap-2 px-3 py-1.5 bg-slate-700 hover:bg-slate-600 disabled:opacity-50 text-slate-300 text-sm rounded-lg transition-colors"
        >
          <RefreshCw size={13} className={isFetching ? 'animate-spin' : ''} />
          Refresh
        </button>
      </div>

      {isLoading ? (
        <Spinner />
      ) : (
        <div className="overflow-x-auto">
          <table className="w-full text-sm">
            <thead>
              <tr className="text-xs text-slate-400 border-b border-slate-700">
                {['Symbol', 'Buy On', 'Buy Price', 'Sell On', 'Sell Price', 'Spread $', 'Spread %'].map(
                  (h) => (
                    <th key={h} className="px-4 py-3 text-left font-medium whitespace-nowrap">
                      {h}
                    </th>
                  ),
                )}
              </tr>
            </thead>
            <tbody>
              {opps.length === 0 ? (
                <tr>
                  <td colSpan={7} className="px-4 py-6 text-slate-500 text-center">
                    No arbitrage opportunities found.{' '}
                    {minSpread !== undefined &&
                      'Try lowering the minimum spread filter.'}
                  </td>
                </tr>
              ) : (
                opps.map((opp) => (
                  <tr
                    key={`${opp.symbol}-${opp.buy_exchange}-${opp.sell_exchange}`}
                    className="border-b border-slate-700/50 hover:bg-slate-700/40 transition-colors"
                  >
                    <td className="px-4 py-3 font-medium text-white">{opp.symbol}</td>
                    <td className="px-4 py-3 text-slate-300 capitalize">{opp.buy_exchange}</td>
                    <td className="px-4 py-3 text-slate-300">{fmt$(opp.buy_price)}</td>
                    <td className="px-4 py-3 text-slate-300 capitalize">{opp.sell_exchange}</td>
                    <td className="px-4 py-3 text-slate-300">{fmt$(opp.sell_price)}</td>
                    <td className="px-4 py-3 text-green-400">{fmt$(opp.spread_usd)}</td>
                    <td
                      className={`px-4 py-3 font-medium ${spreadColor(opp.spread_pct)}`}
                    >
                      {fmtPct(opp.spread_pct)}
                    </td>
                  </tr>
                ))
              )}
            </tbody>
          </table>
        </div>
      )}
    </div>
  )
}

// ─── Page ─────────────────────────────────────────────────────────────────────

export default function Analytics() {
  return (
    <div className="space-y-8">
      <h1 className="text-2xl font-bold text-white">Analytics</h1>

      {/* Trade Summary */}
      <section className="bg-slate-800 rounded-xl border border-slate-700 p-5">
        <h2 className="font-semibold text-white mb-4">Trade Summary</h2>
        <TradeSummarySection />
      </section>

      {/* Coin Performance */}
      <section className="bg-slate-800 rounded-xl border border-slate-700 overflow-hidden">
        <div className="px-5 py-4 border-b border-slate-700">
          <h2 className="font-semibold text-white">Coin Performance</h2>
          <p className="text-xs text-slate-400 mt-0.5">Sorted by total PnL descending</p>
        </div>
        <CoinPerformanceSection />
      </section>

      {/* Arbitrage Scanner */}
      <section className="bg-slate-800 rounded-xl border border-slate-700 overflow-hidden">
        <div className="px-5 py-4 border-b border-slate-700">
          <h2 className="font-semibold text-white">Arbitrage Scanner</h2>
          <p className="text-xs text-slate-400 mt-0.5">
            Price differences across exchanges
          </p>
        </div>
        <div className="p-5">
          <ArbitrageSection />
        </div>
      </section>
    </div>
  )
}
