import { useState, type FormEvent } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { Play, Square, Trash2, ChevronDown, ChevronUp } from 'lucide-react'
import { createBot, listBots, getBot, startBot, stopBot, deleteBot, type CreateBotPayload, type Bot } from '../api'

// ─── Helpers ──────────────────────────────────────────────────────────────────

const fmt$ = (v: number) =>
  `$${v.toLocaleString('en-US', { minimumFractionDigits: 2, maximumFractionDigits: 2 })}`

const STATUS_COLORS: Record<string, string> = {
  running: 'bg-green-900/50 text-green-400',
  stopped: 'bg-slate-700 text-slate-400',
  error: 'bg-red-900/50 text-red-400',
}

function StatusBadge({ status }: { status: string }) {
  return (
    <span
      className={`text-xs font-medium px-2 py-0.5 rounded-full capitalize ${
        STATUS_COLORS[status] ?? 'bg-slate-700 text-slate-400'
      }`}
    >
      {status}
    </span>
  )
}

function Spinner() {
  return (
    <div className="flex justify-center py-10">
      <div className="w-6 h-6 border-2 border-blue-500 border-t-transparent rounded-full animate-spin" />
    </div>
  )
}

// ─── Create form ──────────────────────────────────────────────────────────────

function CreateBotForm() {
  const qc = useQueryClient()
  const [error, setError] = useState('')
  const [success, setSuccess] = useState('')

  const mutation = useMutation({
    mutationFn: createBot,
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: ['bots'] })
      setSuccess('Bot created successfully.')
      setError('')
      setTimeout(() => setSuccess(''), 3000)
    },
    onError: () => {
      setError('Failed to create bot.')
      setSuccess('')
    },
  })

  const handleSubmit = (e: FormEvent<HTMLFormElement>) => {
    e.preventDefault()
    const fd = new FormData(e.currentTarget)
    const payload: CreateBotPayload = {
      exchange: fd.get('exchange') as string,
      symbol: (fd.get('symbol') as string).toUpperCase(),
      category: fd.get('category') as string,
      price_low: parseFloat(fd.get('price_low') as string),
      price_high: parseFloat(fd.get('price_high') as string),
      grids: parseInt(fd.get('grids') as string, 10),
      qty_per_grid: parseFloat(fd.get('qty_per_grid') as string),
    }
    mutation.mutate(payload)
  }

  return (
    <div className="bg-slate-800 rounded-xl border border-slate-700 p-5">
      <h2 className="font-semibold text-white mb-4">Create Grid Bot</h2>

      {error && (
        <div className="mb-3 px-3 py-2 rounded bg-red-900/40 border border-red-700 text-red-300 text-sm">
          {error}
        </div>
      )}
      {success && (
        <div className="mb-3 px-3 py-2 rounded bg-green-900/40 border border-green-700 text-green-300 text-sm">
          {success}
        </div>
      )}

      <form
        onSubmit={handleSubmit}
        className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-3"
      >
        <div>
          <label className="label-sm">Exchange</label>
          <select name="exchange" className="input-field">
            {['binance', 'okx', 'bybit'].map((ex) => (
              <option key={ex} value={ex}>
                {ex.toUpperCase()}
              </option>
            ))}
          </select>
        </div>

        <div>
          <label className="label-sm">Symbol</label>
          <input
            name="symbol"
            required
            placeholder="BTCUSDT"
            className="input-field uppercase"
          />
        </div>

        <div>
          <label className="label-sm">Category</label>
          <select name="category" className="input-field">
            <option value="spot">Spot</option>
            <option value="futures">Futures</option>
          </select>
        </div>

        <div>
          <label className="label-sm">Price Low ($)</label>
          <input
            name="price_low"
            type="number"
            step="any"
            min="0"
            required
            placeholder="55000"
            className="input-field"
          />
        </div>

        <div>
          <label className="label-sm">Price High ($)</label>
          <input
            name="price_high"
            type="number"
            step="any"
            min="0"
            required
            placeholder="75000"
            className="input-field"
          />
        </div>

        <div>
          <label className="label-sm">Grids (2–100)</label>
          <input
            name="grids"
            type="number"
            min="2"
            max="100"
            required
            defaultValue={10}
            className="input-field"
          />
        </div>

        <div>
          <label className="label-sm">Qty per Grid</label>
          <input
            name="qty_per_grid"
            type="number"
            step="any"
            min="0"
            required
            placeholder="0.001"
            className="input-field"
          />
        </div>

        <div className="flex items-end">
          <button
            type="submit"
            disabled={mutation.isPending}
            className="w-full px-4 py-2.5 bg-blue-600 hover:bg-blue-500 disabled:opacity-50 text-white text-sm font-medium rounded-lg transition-colors"
          >
            {mutation.isPending ? 'Creating…' : 'Create Bot'}
          </button>
        </div>
      </form>
    </div>
  )
}

// ─── Bot card (expandable) ────────────────────────────────────────────────────

function BotCard({ bot }: { bot: Bot }) {
  const qc = useQueryClient()
  const [expanded, setExpanded] = useState(false)

  const { data: botDetail } = useQuery({
    queryKey: ['bot', bot.id],
    queryFn: () => getBot(bot.id),
    enabled: expanded,
  })

  const startMutation = useMutation({
    mutationFn: startBot,
    onSuccess: () => qc.invalidateQueries({ queryKey: ['bots'] }),
  })

  const stopMutation = useMutation({
    mutationFn: stopBot,
    onSuccess: () => qc.invalidateQueries({ queryKey: ['bots'] }),
  })

  const deleteMutation = useMutation({
    mutationFn: deleteBot,
    onSuccess: () => qc.invalidateQueries({ queryKey: ['bots'] }),
  })

  const isRunning = bot.status === 'running'

  return (
    <div className="bg-slate-800 rounded-xl border border-slate-700 overflow-hidden">
      {/* Header row */}
      <div className="flex items-center gap-4 p-4">
        <div className="flex-1 min-w-0">
          <div className="flex items-center gap-2 flex-wrap">
            <span className="font-semibold text-white">
              {bot.symbol}
            </span>
            <span className="text-xs text-slate-400 capitalize">({bot.exchange})</span>
            <StatusBadge status={bot.status} />
          </div>
          <div className="mt-1 flex flex-wrap gap-x-4 gap-y-1 text-xs text-slate-400">
            <span>
              Range: {fmt$(bot.price_low)} – {fmt$(bot.price_high)}
            </span>
            <span>{bot.grids} grids</span>
            <span
              className={`font-medium ${
                bot.total_profit >= 0 ? 'text-green-400' : 'text-red-400'
              }`}
            >
              Profit: {bot.total_profit >= 0 ? '+' : ''}
              {fmt$(bot.total_profit)}
            </span>
          </div>
        </div>

        {/* Actions */}
        <div className="flex items-center gap-2 flex-shrink-0">
          {isRunning ? (
            <button
              onClick={() => stopMutation.mutate(bot.id)}
              disabled={stopMutation.isPending}
              title="Stop"
              className="p-1.5 rounded-lg bg-slate-700 hover:bg-slate-600 text-slate-300 disabled:opacity-50 transition-colors"
            >
              <Square size={15} />
            </button>
          ) : (
            <button
              onClick={() => startMutation.mutate(bot.id)}
              disabled={startMutation.isPending}
              title="Start"
              className="p-1.5 rounded-lg bg-green-800/60 hover:bg-green-700/60 text-green-400 disabled:opacity-50 transition-colors"
            >
              <Play size={15} />
            </button>
          )}
          <button
            onClick={() => deleteMutation.mutate(bot.id)}
            disabled={deleteMutation.isPending || isRunning}
            title={isRunning ? 'Stop bot first' : 'Delete'}
            className="p-1.5 rounded-lg bg-slate-700 hover:bg-red-900/50 text-slate-400 hover:text-red-400 disabled:opacity-40 transition-colors"
          >
            <Trash2 size={15} />
          </button>
          <button
            onClick={() => setExpanded((v) => !v)}
            className="p-1.5 rounded-lg bg-slate-700 hover:bg-slate-600 text-slate-300 transition-colors"
            title={expanded ? 'Collapse' : 'Expand grid levels'}
          >
            {expanded ? <ChevronUp size={15} /> : <ChevronDown size={15} />}
          </button>
        </div>
      </div>

      {/* Grid levels */}
      {expanded && (
        <div className="border-t border-slate-700">
          {!botDetail ? (
            <div className="py-4">
              <Spinner />
            </div>
          ) : !botDetail.grid_levels || botDetail.grid_levels.length === 0 ? (
            <p className="px-5 py-4 text-sm text-slate-500">No grid levels yet.</p>
          ) : (
            <div className="overflow-x-auto">
              <table className="w-full text-xs">
                <thead>
                  <tr className="text-slate-400 border-b border-slate-700">
                    <th className="px-4 py-2 text-left font-medium">Level</th>
                    <th className="px-4 py-2 text-left font-medium">Price</th>
                    <th className="px-4 py-2 text-left font-medium">Side</th>
                    <th className="px-4 py-2 text-left font-medium">Status</th>
                  </tr>
                </thead>
                <tbody>
                  {botDetail.grid_levels.map((gl, i) => (
                    <tr
                      key={gl.id}
                      className="border-b border-slate-700/50 hover:bg-slate-700/30 transition-colors"
                    >
                      <td className="px-4 py-2 text-slate-500">#{i + 1}</td>
                      <td className="px-4 py-2 text-slate-300">{fmt$(gl.price)}</td>
                      <td className="px-4 py-2">
                        <span
                          className={`px-1.5 py-0.5 rounded font-medium ${
                            gl.side === 'buy'
                              ? 'bg-green-900/40 text-green-400'
                              : 'bg-red-900/40 text-red-400'
                          }`}
                        >
                          {gl.side.toUpperCase()}
                        </span>
                      </td>
                      <td className="px-4 py-2 text-slate-400 capitalize">{gl.status}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </div>
      )}
    </div>
  )
}

// ─── Bots list ────────────────────────────────────────────────────────────────

function BotsList() {
  const { data: bots = [], isLoading } = useQuery({
    queryKey: ['bots'],
    queryFn: listBots,
    refetchInterval: 15_000,
  })

  if (isLoading) return <Spinner />

  if (bots.length === 0)
    return (
      <div className="bg-slate-800 rounded-xl border border-slate-700 px-5 py-8 text-center text-sm text-slate-500">
        No grid bots yet. Create one above.
      </div>
    )

  return (
    <div className="space-y-3">
      {bots.map((bot) => (
        <BotCard key={bot.id} bot={bot} />
      ))}
    </div>
  )
}

// ─── Page ─────────────────────────────────────────────────────────────────────

export default function GridBots() {
  return (
    <div className="space-y-6">
      <h1 className="text-2xl font-bold text-white">Grid Bots</h1>
      <CreateBotForm />
      <BotsList />
    </div>
  )
}
