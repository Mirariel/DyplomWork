import { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { TrendingUp, TrendingDown, RefreshCw, AlertCircle } from 'lucide-react'
import { getFuturesPositions } from '../api'
import ExchangeSelector from '../components/ExchangeSelector'

function fmt(n: number, decimals = 2) {
  return n.toLocaleString('en-US', { minimumFractionDigits: decimals, maximumFractionDigits: decimals })
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

export default function FuturesPositions() {
  const [exchange, setExchange] = useState('')

  const { data, isLoading, isError, refetch, isFetching } = useQuery({
    queryKey: ['futures-positions', exchange],
    queryFn: () => getFuturesPositions(exchange || undefined),
    refetchInterval: 30_000,
  })

  const positions = data?.positions ?? []
  const totalPnl = data?.total_unrealized_pnl ?? 0

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold text-white">Futures Positions</h1>
          <p className="text-slate-400 text-sm mt-1">Open leveraged positions across connected exchanges</p>
        </div>
        <button
          onClick={() => void refetch()}
          disabled={isFetching}
          className="flex items-center gap-2 px-3 py-2 bg-slate-700 hover:bg-slate-600 text-slate-300 rounded-lg text-sm transition-colors disabled:opacity-50"
        >
          <RefreshCw size={15} className={isFetching ? 'animate-spin' : ''} />
          Refresh
        </button>
      </div>

      {/* Summary + Exchange filter */}
      <div className="flex flex-col sm:flex-row gap-4">
        {/* Total PnL card */}
        <div className="flex-1 bg-slate-800 rounded-xl p-5 border border-slate-700">
          <p className="text-slate-400 text-sm mb-1">Total Unrealized PnL</p>
          <p className={`text-3xl font-bold ${totalPnl >= 0 ? 'text-green-400' : 'text-red-400'}`}>
            {totalPnl >= 0 ? '+' : ''}{fmt(totalPnl)} USDT
          </p>
          <p className="text-slate-500 text-xs mt-1">{positions.length} open position{positions.length !== 1 ? 's' : ''}</p>
        </div>

        {/* Exchange filter */}
        <div className="flex items-start gap-3">
          <div>
            <p className="text-slate-400 text-xs mb-1.5">Exchange</p>
            <ExchangeSelector value={exchange} onChange={setExchange} />
          </div>
        </div>
      </div>

      {/* Table */}
      {isError ? (
        <div className="flex items-center gap-3 text-red-400 bg-red-400/10 border border-red-400/20 rounded-xl p-4">
          <AlertCircle size={18} />
          <span className="text-sm">Failed to load futures positions. Check your API keys in Portfolio settings.</span>
        </div>
      ) : isLoading ? (
        <div className="flex items-center justify-center py-20">
          <div className="w-8 h-8 border-2 border-blue-500 border-t-transparent rounded-full animate-spin" />
        </div>
      ) : positions.length === 0 ? (
        <div className="text-center py-20 text-slate-500">
          <TrendingUp size={40} className="mx-auto mb-3 opacity-30" />
          <p className="text-lg font-medium">No open futures positions</p>
          <p className="text-sm mt-1">
            {exchange
              ? `No positions found on ${exchange.charAt(0).toUpperCase() + exchange.slice(1)}`
              : 'Add API keys in Portfolio to see your positions'}
          </p>
        </div>
      ) : (
        <div className="bg-slate-800 rounded-xl border border-slate-700 overflow-hidden">
          <div className="overflow-x-auto">
            <table className="w-full text-sm">
              <thead>
                <tr className="border-b border-slate-700 text-slate-400 text-xs uppercase tracking-wider">
                  <th className="text-left px-4 py-3">Symbol</th>
                  <th className="text-left px-4 py-3">Exchange</th>
                  <th className="text-left px-4 py-3">Side</th>
                  <th className="text-right px-4 py-3">Size</th>
                  <th className="text-right px-4 py-3">Entry Price</th>
                  <th className="text-right px-4 py-3">Mark Price</th>
                  <th className="text-right px-4 py-3">Unrealized PnL</th>
                  <th className="text-center px-4 py-3">Leverage</th>
                  <th className="text-center px-4 py-3">Margin</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-slate-700/50">
                {positions.map((p) => (
                  <tr key={p.id} className="hover:bg-slate-700/30 transition-colors">
                    <td className="px-4 py-3 font-medium text-white">{p.symbol}</td>
                    <td className="px-4 py-3 text-slate-400">
                      {p.exchange.charAt(0).toUpperCase() + p.exchange.slice(1)}
                    </td>
                    <td className="px-4 py-3">
                      <span
                        className={`inline-block px-2 py-0.5 rounded text-xs font-semibold ${
                          p.side === 'LONG'
                            ? 'bg-green-500/20 text-green-400'
                            : 'bg-red-500/20 text-red-400'
                        }`}
                      >
                        {p.side}
                      </span>
                    </td>
                    <td className="px-4 py-3 text-right text-slate-200">{fmt(p.size, 4)}</td>
                    <td className="px-4 py-3 text-right text-slate-200">{fmt(p.entry_price)}</td>
                    <td className="px-4 py-3 text-right text-slate-200">{fmt(p.mark_price)}</td>
                    <td className="px-4 py-3 text-right">
                      <PnlBadge value={p.unrealized_pnl} />
                    </td>
                    <td className="px-4 py-3 text-center text-slate-300 font-medium">{p.leverage}x</td>
                    <td className="px-4 py-3 text-center">
                      <span className="text-xs text-slate-400 capitalize">{p.margin_type}</span>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      )}
    </div>
  )
}
