import { useState, type FormEvent } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { XCircle } from 'lucide-react'
import {
  createSmartOrder,
  listSmartOrders,
  cancelSmartOrder,
  type CreateSmartOrderPayload,
} from '../api'

// ─── Helpers ──────────────────────────────────────────────────────────────────

const fmt$ = (v: number) =>
  `$${v.toLocaleString('en-US', { minimumFractionDigits: 2, maximumFractionDigits: 2 })}`

const fmtDate = (iso: string) =>
  new Date(iso).toLocaleString('en-US', {
    month: 'short',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
  })

const STATUS_COLORS: Record<string, string> = {
  active: 'bg-blue-900/50 text-blue-400',
  triggered: 'bg-purple-900/50 text-purple-400',
  cancelled: 'bg-red-900/50 text-red-400',
  filled: 'bg-green-900/50 text-green-400',
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

function CreateSmartOrderForm() {
  const qc = useQueryClient()
  const [orderType, setOrderType] = useState('stop_loss')
  const [error, setError] = useState('')
  const [success, setSuccess] = useState('')

  const mutation = useMutation({
    mutationFn: createSmartOrder,
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: ['smart-orders'] })
      setSuccess('Smart order created.')
      setError('')
      setTimeout(() => setSuccess(''), 3000)
    },
    onError: () => {
      setError('Failed to create smart order.')
      setSuccess('')
    },
  })

  const handleSubmit = (e: FormEvent<HTMLFormElement>) => {
    e.preventDefault()
    const fd = new FormData(e.currentTarget)
    const payload: CreateSmartOrderPayload = {
      exchange: fd.get('exchange') as string,
      symbol: (fd.get('symbol') as string).toUpperCase(),
      side: fd.get('side') as string,
      category: fd.get('category') as string,
      quantity: parseFloat(fd.get('quantity') as string),
      order_type: fd.get('order_type') as string,
    }
    if (orderType !== 'trailing_stop') {
      payload.trigger_price = parseFloat(fd.get('trigger_price') as string)
    } else {
      payload.callback_rate = parseFloat(fd.get('callback_rate') as string)
    }
    mutation.mutate(payload)
  }

  const showTriggerPrice = orderType === 'stop_loss' || orderType === 'take_profit'
  const showCallbackRate = orderType === 'trailing_stop'

  return (
    <div className="bg-slate-800 rounded-xl border border-slate-700 p-5">
      <h2 className="font-semibold text-white mb-4">Create Smart Order</h2>

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
          <label className="label-sm">Side</label>
          <select name="side" className="input-field">
            <option value="sell">Sell</option>
            <option value="buy">Buy</option>
          </select>
        </div>

        <div>
          <label className="label-sm">Category</label>
          <select name="category" className="input-field">
            <option value="futures">Futures</option>
            <option value="spot">Spot</option>
          </select>
        </div>

        <div>
          <label className="label-sm">Order Type</label>
          <select
            name="order_type"
            className="input-field"
            value={orderType}
            onChange={(e) => setOrderType(e.target.value)}
          >
            <option value="stop_loss">Stop Loss</option>
            <option value="take_profit">Take Profit</option>
            <option value="trailing_stop">Trailing Stop</option>
          </select>
        </div>

        <div>
          <label className="label-sm">Quantity</label>
          <input
            name="quantity"
            type="number"
            step="any"
            min="0"
            required
            placeholder="0.01"
            className="input-field"
          />
        </div>

        {showTriggerPrice && (
          <div>
            <label className="label-sm">Trigger Price (USD)</label>
            <input
              name="trigger_price"
              type="number"
              step="any"
              min="0"
              required
              placeholder="60000"
              className="input-field"
            />
          </div>
        )}

        {showCallbackRate && (
          <div>
            <label className="label-sm">Callback Rate (%)</label>
            <input
              name="callback_rate"
              type="number"
              step="0.1"
              min="0.1"
              max="10"
              required
              placeholder="1.5"
              className="input-field"
            />
          </div>
        )}

        <div className="sm:col-span-2 lg:col-span-3 pt-1">
          <button
            type="submit"
            disabled={mutation.isPending}
            className="px-6 py-2 bg-blue-600 hover:bg-blue-500 disabled:opacity-50 text-white text-sm font-medium rounded-lg transition-colors"
          >
            {mutation.isPending ? 'Creating…' : 'Create Smart Order'}
          </button>
        </div>
      </form>
    </div>
  )
}

// ─── Smart orders list ────────────────────────────────────────────────────────

function SmartOrdersList() {
  const qc = useQueryClient()

  const { data: orders = [], isLoading } = useQuery({
    queryKey: ['smart-orders'],
    queryFn: listSmartOrders,
    refetchInterval: 10_000,
  })

  const cancelMutation = useMutation({
    mutationFn: cancelSmartOrder,
    onSuccess: () => qc.invalidateQueries({ queryKey: ['smart-orders'] }),
  })

  if (isLoading) return <Spinner />

  return (
    <div className="bg-slate-800 rounded-xl border border-slate-700 overflow-hidden">
      <div className="px-5 py-4 border-b border-slate-700">
        <h2 className="font-semibold text-white text-sm">
          Smart Orders ({orders.length})
        </h2>
      </div>
      {orders.length === 0 ? (
        <p className="px-5 py-6 text-sm text-slate-500">No smart orders found.</p>
      ) : (
        <div className="overflow-x-auto">
          <table className="w-full text-sm">
            <thead>
              <tr className="text-xs text-slate-400 border-b border-slate-700">
                {[
                  'ID',
                  'Exchange',
                  'Symbol',
                  'Side',
                  'Type',
                  'Qty',
                  'Trigger / Callback',
                  'Status',
                  'Created',
                  '',
                ].map((h) => (
                  <th key={h} className="px-4 py-3 text-left font-medium whitespace-nowrap">
                    {h}
                  </th>
                ))}
              </tr>
            </thead>
            <tbody>
              {orders.map((o) => (
                <tr
                  key={o.id}
                  className="border-b border-slate-700/50 hover:bg-slate-700/40 transition-colors"
                >
                  <td className="px-4 py-3 text-slate-500 font-mono text-xs">#{o.id}</td>
                  <td className="px-4 py-3 text-slate-300 capitalize">{o.exchange}</td>
                  <td className="px-4 py-3 font-medium text-white">{o.symbol}</td>
                  <td className="px-4 py-3">
                    <span
                      className={`text-xs font-medium px-2 py-0.5 rounded-full ${
                        o.side === 'buy'
                          ? 'bg-green-900/50 text-green-400'
                          : 'bg-red-900/50 text-red-400'
                      }`}
                    >
                      {o.side.toUpperCase()}
                    </span>
                  </td>
                  <td className="px-4 py-3 text-slate-300 capitalize">
                    {o.order_type.replace('_', ' ')}
                  </td>
                  <td className="px-4 py-3 text-slate-300">{o.quantity}</td>
                  <td className="px-4 py-3 text-slate-300">
                    {o.order_type === 'trailing_stop'
                      ? `${o.callback_rate}%`
                      : o.trigger_price > 0
                      ? fmt$(o.trigger_price)
                      : '—'}
                  </td>
                  <td className="px-4 py-3">
                    <StatusBadge status={o.status} />
                  </td>
                  <td className="px-4 py-3 text-slate-400 whitespace-nowrap text-xs">
                    {fmtDate(o.created_at)}
                  </td>
                  <td className="px-4 py-3">
                    {o.status === 'active' && (
                      <button
                        onClick={() => cancelMutation.mutate(o.id)}
                        disabled={cancelMutation.isPending}
                        title="Cancel"
                        className="text-slate-400 hover:text-red-400 transition-colors disabled:opacity-50"
                      >
                        <XCircle size={16} />
                      </button>
                    )}
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  )
}

// ─── Page ─────────────────────────────────────────────────────────────────────

export default function SmartOrders() {
  return (
    <div className="space-y-6">
      <h1 className="text-2xl font-bold text-white">Smart Orders</h1>
      <CreateSmartOrderForm />
      <SmartOrdersList />
    </div>
  )
}
