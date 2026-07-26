import type { TradeSummary } from '../api'
import { fmt$ } from '../lib/format'

interface Props {
  summary: TradeSummary
  aggRR: number | null
  title?: string
}

export default function StatsPanel({ summary, aggRR, title }: Props) {
  return (
    <div className="space-y-4">
      <p className="text-xs text-slate-500 uppercase tracking-wide font-semibold">
        {title ?? 'Статистика'}
      </p>

      {/* PnL */}
      <div>
        <p className="text-xs text-slate-500 mb-0.5">Загальний PnL</p>
        <p className={`text-xl font-bold ${summary.total_realized_pnl >= 0 ? 'text-green-400' : 'text-red-400'}`}>
          {summary.total_realized_pnl >= 0 ? '+' : ''}{fmt$(summary.total_realized_pnl)}
        </p>
      </div>

      {/* Trade counts */}
      <div className="space-y-1.5 text-xs">
        <div className="flex justify-between">
          <span className="text-slate-400">Угод</span>
          <span className="text-white font-semibold">{summary.total_trades}</span>
        </div>
        <div className="flex justify-between">
          <span className="text-slate-400">Прибуткових</span>
          <span className="text-green-400 font-semibold">{summary.winning_trades}</span>
        </div>
        <div className="flex justify-between">
          <span className="text-slate-400">Збиткових</span>
          <span className="text-red-400 font-semibold">{summary.losing_trades}</span>
        </div>
      </div>

      {/* Win rate bar */}
      <div>
        <div className="flex justify-between text-xs text-slate-500 mb-1">
          <span>Win Rate</span>
          <span className={`font-semibold ${summary.winrate >= 50 ? 'text-green-400' : 'text-red-400'}`}>
            {summary.winrate.toFixed(1)}%
          </span>
        </div>
        <div className="h-1.5 bg-slate-700 rounded-full overflow-hidden">
          <div className="h-full bg-green-500 rounded-full" style={{ width: `${Math.min(100, summary.winrate)}%` }} />
        </div>
      </div>

      {/* Best / Worst / Avg */}
      <div className="space-y-1.5 text-xs">
        <div className="flex justify-between">
          <span className="text-slate-400">Найкраща</span>
          <span className="text-green-400 font-medium">+{fmt$(summary.best_trade)}</span>
        </div>
        <div className="flex justify-between">
          <span className="text-slate-400">Найгірша</span>
          <span className="text-red-400 font-medium">{fmt$(summary.worst_trade)}</span>
        </div>
        <div className="flex justify-between">
          <span className="text-slate-400">Середня</span>
          <span className={`font-medium ${summary.avg_pnl >= 0 ? 'text-green-400' : 'text-red-400'}`}>
            {summary.avg_pnl >= 0 ? '+' : ''}{fmt$(summary.avg_pnl)}
          </span>
        </div>
      </div>

      {/* R:R */}
      <div className="bg-slate-700/50 rounded-lg p-3 space-y-1.5">
        <p className="text-xs text-slate-400 font-medium">Risk / Reward</p>
        {aggRR !== null ? (
          <>
            <div className={`text-sm font-bold ${aggRR >= 1.5 ? 'text-green-400' : aggRR >= 1.0 ? 'text-yellow-400' : 'text-red-400'}`}>
              1 : {aggRR.toFixed(2)}
            </div>
            <p className={`text-xs ${aggRR >= 1.5 ? 'text-green-500' : aggRR >= 1.0 ? 'text-yellow-500' : 'text-red-400'}`}>
              {aggRR >= 1.5 ? 'Відмінне співвідношення' : aggRR >= 1.0 ? 'Прийнятне співвідношення' : 'Ризик перевищує прибуток'}
            </p>
          </>
        ) : (
          <span className="text-slate-600 text-xs">—</span>
        )}
      </div>

      {/* Fees + Profit Factor */}
      <div className="space-y-1.5 text-xs">
        {summary.total_fees > 0 && (
          <div className="flex justify-between">
            <span className="text-slate-400">Комісії</span>
            <span className="text-slate-300 font-medium">-{fmt$(summary.total_fees)}</span>
          </div>
        )}
        <div className="flex justify-between">
          <span className="text-slate-400">Profit Factor</span>
          <span className={`font-medium ${summary.profit_factor >= 1.5 ? 'text-green-400' : summary.profit_factor >= 1 ? 'text-yellow-400' : 'text-red-400'}`}>
            {summary.profit_factor.toFixed(2)}
          </span>
        </div>
      </div>
    </div>
  )
}
