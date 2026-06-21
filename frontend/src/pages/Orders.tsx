import { useState, type FormEvent } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { XCircle } from 'lucide-react'
import { placeOrder, listOrders, cancelOrder, type PlaceOrderPayload } from '../api'

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
  open: 'bg-blue-900/50 text-blue-400',
  filled: 'bg-green-900/50 text-green-400',
  cancelled: 'bg-red-900/50 text-red-400',
  partial: 'bg-yellow-900/50 text-yellow-400',
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

// ─── Place Order form ─────────────────────────────────────────────────────────

function PlaceOrderForm() {
  const qc = useQueryClient()
  const [orderType, setOrderType] = useState<'market' | 'limit'>('limit')
  const [error, setError] = useState('')
  const [success, setSuccess] = useState('')

  const mutation = useMutation({
    mutationFn: placeOrder,
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: ['orders'] })
      setSuccess('Order placed successfully.')
      setError('')
      setTimeout(() => setSuccess(''), 3000)
    },
    onError: () => {
      setError('Failed to place order. Check your credentials and parameters.')
      setSuccess('')
    },
  })

  const handleSubmit = (e: FormEvent<HTMLFormElement>) => {
    e.preventDefault()
    const fd = new FormData(e.currentTarget)
    const payload: PlaceOrderPayload = {
      exchange: fd.get('exchange') as string,
      symbol: (fd.get('symbol') as string).toUpperCase(),
      side: fd.get('side') as string,
      category: fd.get('category') as string,
      type: fd.get('type') as string,
      quantity: parseFloat(fd.get('quantity') as string),
    }
    if (orderType === 'limit') {
      payload.price = parseFloat(fd.get('price') as string)
    }
    mutation.mutate(payload)
  }

  return (
    <div className="bg-slate-800 rounded-xl border border-slate-700 p-5">
      <h2 className="font-semibold text-white mb-4">Place Order</h2>

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
            <option value="buy">Buy</option>
            <option value="sell">Sell</option>
          </select>
        </div>

        <div>
          <label className="label-sm">Category</label>
          <select name="category" className="input-field">
            <option value="spot">Spot</option>
            <option value="futures">Futures</option>
          </select>
        </div>

        <div>
          <label className="label-sm">Order Type</label>
          <select
            name="type"
            className="input-field"
            onChange={(e) => setOrderType(e.target.value as 'market' | 'limit')}
            value={orderType}
          >
            <option value="limit">Limit</option>
            <option value="market">Market</option>
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
            placeholder="0.001"
            className="input-field"
          />
        </div>

        {orderType === 'limit' && (
          <div>
            <label className="label-sm">Price (USD)</label>
            <input
              name="price"
              type="number"
              step="any"
              min="0"
              required={orderType === 'limit'}
              placeholder="65000"
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
            {mutation.isPending ? 'Placing…' : 'Place Order'}
          </button>
        </div>
      </form>
    </div>
  )
}

// ─── Orders list ──────────────────────────────────────────────────────────────

function OrdersList() {
  const qc = useQueryClient()

  const { data: orders = [], isLoading } = useQuery({
    queryKey: ['orders'],
    queryFn: () => listOrders(),
    refetchInterval: 10_000,
  })

  const cancelMutation = useMutation({
    mutationFn: cancelOrder,
    onSuccess: () => qc.invalidateQueries({ queryKey: ['orders'] }),
  })

  if (isLoading) return <Spinner />

  return (
    <div className="bg-slate-800 rounded-xl border border-slate-700 overflow-hidden">
      <div className="px-5 py-4 border-b border-slate-700">
        <h2 className="font-semibold text-white text-sm">Orders ({orders.length})</h2>
      </div>
      {orders.length === 0 ? (
        <p className="px-5 py-6 text-sm text-slate-500">No orders found.</p>
      ) : (
        <div className="overflow-x-auto">
          <table className="w-full text-sm">
            <thead>
              <tr className="text-xs text-slate-400 border-b border-slate-700">
                {['ID', 'Exchange', 'Symbol', 'Side', 'Type', 'Qty', 'Price', 'Status', 'Created', ''].map(
                  (h) => (
                    <th key={h} className="px-4 py-3 text-left font-medium whitespace-nowrap">
                      {h}
                    </th>
                  ),
                )}
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
                  <td className="px-4 py-3 text-slate-300 capitalize">{o.type}</td>
                  <td className="px-4 py-3 text-slate-300">{o.quantity}</td>
                  <td className="px-4 py-3 text-slate-300">
                    {o.price > 0 ? fmt$(o.price) : '—'}
                  </td>
                  <td className="px-4 py-3">
                    <StatusBadge status={o.status} />
                  </td>
                  <td className="px-4 py-3 text-slate-400 whitespace-nowrap text-xs">
                    {fmtDate(o.created_at)}
                  </td>
                  <td className="px-4 py-3">
                    {o.status === 'open' && (
                      <button
                        onClick={() => cancelMutation.mutate(o.id)}
                        disabled={cancelMutation.isPending}
                        title="Cancel order"
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

// ─── Orders page ──────────────────────────────────────────────────────────────

export default function Orders() {
  return (
    <div className="space-y-6">
      <h1 className="text-2xl font-bold text-white">Orders</h1>
      <PlaceOrderForm />
      <OrdersList />
    </div>
  )
}
