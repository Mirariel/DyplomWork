import { useState, type FormEvent } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { Play, Square, Trash2, ShoppingBag } from 'lucide-react'
import {
  createDCA,
  listDCA,
  startDCA,
  stopDCA,
  deleteDCA,
  type CreateDCAPayload,
  type DCABot,
} from '../api'

// ─── Helpers ──────────────────────────────────────────────────────────────────

const fmt$ = (v: number) =>
  `$${v.toLocaleString('en-US', { minimumFractionDigits: 2, maximumFractionDigits: 2 })}`

const fmtDate = (iso: string) => {
  if (!iso || iso === '0001-01-01T00:00:00Z') return '—'
  return new Date(iso).toLocaleString('en-US', {
    month: 'short',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
  })
}

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

function CreateDCAForm() {
  const qc = useQueryClient()
  const [error, setError] = useState('')
  const [success, setSuccess] = useState('')

  const mutation = useMutation({
    mutationFn: createDCA,
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: ['dca'] })
      setSuccess('DCA bot created.')
      setError('')
      setTimeout(() => setSuccess(''), 3000)
    },
    onError: () => {
      setError('Failed to create DCA bot.')
      setSuccess('')
    },
  })

  const handleSubmit = (e: FormEvent<HTMLFormElement>) => {
    e.preventDefault()
    const fd = new FormData(e.currentTarget)
    const payload: CreateDCAPayload = {
      exchange: fd.get('exchange') as string,
      symbol: (fd.get('symbol') as string).toUpperCase(),
      category: fd.get('category') as string,
      amount_usd: parseFloat(fd.get('amount_usd') as string),
      interval_hours: parseFloat(fd.get('interval_hours') as string),
    }
    mutation.mutate(payload)
  }

  return (
    <div className="bg-slate-800 rounded-xl border border-slate-700 p-5">
      <h2 className="font-semibold text-white mb-4">Create DCA Bot</h2>

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
        className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-3"
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
          <label className="label-sm">Amount per Buy (USD)</label>
          <input
            name="amount_usd"
            type="number"
            step="any"
            min="1"
            required
            placeholder="100"
            className="input-field"
          />
        </div>

        <div>
          <label className="label-sm">Interval (hours)</label>
          <input
            name="interval_hours"
            type="number"
            step="0.5"
            min="0.5"
            required
            defaultValue={24}
            className="input-field"
          />
        </div>

        <div className="flex items-end">
          <button
            type="submit"
            disabled={mutation.isPending}
            className="w-full px-4 py-2.5 bg-blue-600 hover:bg-blue-500 disabled:opacity-50 text-white text-sm font-medium rounded-lg transition-colors"
          >
            {mutation.isPending ? 'Creating…' : 'Create DCA Bot'}
          </button>
        </div>
      </form>
    </div>
  )
}

// ─── DCA Bot card ─────────────────────────────────────────────────────────────

function DCABotCard({ bot }: { bot: DCABot }) {
  const qc = useQueryClient()

  const startMutation = useMutation({
    mutationFn: ({ id, buyNow }: { id: number; buyNow: boolean }) =>
      startDCA(id, buyNow),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['dca'] }),
  })

  const stopMutation = useMutation({
    mutationFn: stopDCA,
    onSuccess: () => qc.invalidateQueries({ queryKey: ['dca'] }),
  })

  const deleteMutation = useMutation({
    mutationFn: deleteDCA,
    onSuccess: () => qc.invalidateQueries({ queryKey: ['dca'] }),
  })

  const isRunning = bot.status === 'running'

  return (
    <div className="bg-slate-800 rounded-xl border border-slate-700 p-4">
      <div className="flex items-start justify-between gap-4">
        {/* Info */}
        <div className="flex-1 min-w-0">
          <div className="flex items-center gap-2 flex-wrap">
            <span className="font-semibold text-white">{bot.symbol}</span>
            <span className="text-xs text-slate-400 capitalize">({bot.exchange})</span>
            <StatusBadge status={bot.status} />
          </div>

          <div className="mt-2 grid grid-cols-2 sm:grid-cols-4 gap-x-4 gap-y-1 text-xs text-slate-400">
            <div>
              <span className="text-slate-500">Per buy:</span>{' '}
              <span className="text-slate-300">{fmt$(bot.amount_usd)}</span>
            </div>
            <div>
              <span className="text-slate-500">Interval:</span>{' '}
              <span className="text-slate-300">{bot.interval_hours}h</span>
            </div>
            <div>
              <span className="text-slate-500">Invested:</span>{' '}
              <span className="text-slate-300">{fmt$(bot.total_invested)}</span>
            </div>
            <div>
              <span className="text-slate-500">Qty:</span>{' '}
              <span className="text-slate-300">{bot.total_quantity}</span>
            </div>
            <div className="col-span-2">
              <span className="text-slate-500">Next buy:</span>{' '}
              <span className="text-slate-300">{fmtDate(bot.next_buy_at)}</span>
            </div>
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
            <>
              <button
                onClick={() => startMutation.mutate({ id: bot.id, buyNow: false })}
                disabled={startMutation.isPending}
                title="Start"
                className="p-1.5 rounded-lg bg-green-800/60 hover:bg-green-700/60 text-green-400 disabled:opacity-50 transition-colors"
              >
                <Play size={15} />
              </button>
              <button
                onClick={() => startMutation.mutate({ id: bot.id, buyNow: true })}
                disabled={startMutation.isPending}
                title="Start & Buy Now"
                className="p-1.5 rounded-lg bg-blue-800/60 hover:bg-blue-700/60 text-blue-400 disabled:opacity-50 transition-colors"
              >
                <ShoppingBag size={15} />
              </button>
            </>
          )}
          <button
            onClick={() => deleteMutation.mutate(bot.id)}
            disabled={deleteMutation.isPending || isRunning}
            title={isRunning ? 'Stop bot first' : 'Delete'}
            className="p-1.5 rounded-lg bg-slate-700 hover:bg-red-900/50 text-slate-400 hover:text-red-400 disabled:opacity-40 transition-colors"
          >
            <Trash2 size={15} />
          </button>
        </div>
      </div>
    </div>
  )
}

// ─── DCA list ─────────────────────────────────────────────────────────────────

function DCAList() {
  const { data: bots = [], isLoading } = useQuery({
    queryKey: ['dca'],
    queryFn: listDCA,
    refetchInterval: 15_000,
  })

  if (isLoading) return <Spinner />

  if (bots.length === 0)
    return (
      <div className="bg-slate-800 rounded-xl border border-slate-700 px-5 py-8 text-center text-sm text-slate-500">
        No DCA bots yet. Create one above.
      </div>
    )

  return (
    <div className="space-y-3">
      {bots.map((bot) => (
        <DCABotCard key={bot.id} bot={bot} />
      ))}
    </div>
  )
}

// ─── Page ─────────────────────────────────────────────────────────────────────

export default function DCABots() {
  return (
    <div className="space-y-6">
      <h1 className="text-2xl font-bold text-white">DCA Bots</h1>
      <p className="text-sm text-slate-400 -mt-4">
        Dollar Cost Averaging — automatically buy at fixed intervals.{' '}
        <span className="text-blue-400">Play</span> = schedule start,{' '}
        <span className="text-blue-400">Bag icon</span> = start and buy immediately.
      </p>
      <CreateDCAForm />
      <DCAList />
    </div>
  )
}
