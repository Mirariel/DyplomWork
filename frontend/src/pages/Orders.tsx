import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { XCircle } from 'lucide-react'
import {
  listOrders, cancelOrder,
  listSmartOrders, cancelSmartOrder,
  getCredentials, listCredentialGroups,
  type Credential, type Order, type SmartOrder,
} from '../api'
import { OrderForm } from '../orders/components/OrderForm'
import { SymbolPanel } from '../components/SymbolPanel'

// ─── Helpers ──────────────────────────────────────────────────────────────────

const fmt$ = (v: number) =>
  `$${v.toLocaleString('en-US', { minimumFractionDigits: 2, maximumFractionDigits: 2 })}`

const fmtDate = (iso: string) =>
  new Date(iso).toLocaleString('en-US', { month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit' })

const STATUS_COLORS: Record<string, string> = {
  open: 'bg-blue-900/50 text-blue-400',
  new: 'bg-blue-900/50 text-blue-400',
  active: 'bg-blue-900/50 text-blue-400',
  filled: 'bg-green-900/50 text-green-400',
  triggered: 'bg-purple-900/50 text-purple-400',
  cancelled: 'bg-red-900/50 text-red-400',
  failed: 'bg-red-900/50 text-red-400',
  rejected: 'bg-red-900/50 text-red-400',
  partial: 'bg-yellow-900/50 text-yellow-400',
}

function StatusBadge({ status }: { status: string }) {
  return (
    <span className={`text-xs font-medium px-2 py-0.5 rounded-full capitalize ${STATUS_COLORS[status] ?? 'bg-slate-700 text-slate-400'}`}>
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

function credLabel(c: Credential) {
  return c.label || `${c.exchange.toUpperCase()} (${c.api_key_hint})`
}

// ─── Unified row type ─────────────────────────────────────────────────────────

interface UnifiedRow {
  kind: 'order' | 'smart'
  id: number
  credentialId?: number
  exchange: string
  symbol: string
  side: string
  category: string
  leverage: string
  type: string
  quantity: number
  price: number
  priceLabel: string
  status: string
  createdAt: string
  cancellable: boolean
}

function mergeRows(orders: Order[], smartOrders: SmartOrder[]): UnifiedRow[] {
  const rows: UnifiedRow[] = []
  for (const o of orders) {
    rows.push({
      kind: 'order', id: o.id, credentialId: o.credential_id, exchange: o.exchange,
      symbol: o.symbol, side: o.side, category: o.category, leverage: o.leverage,
      type: o.type, quantity: o.quantity, price: o.price,
      priceLabel: o.price > 0 ? fmt$(o.price) : '—',
      status: o.status, createdAt: o.created_at,
      cancellable: o.status === 'open' || o.status === 'new',
    })
  }
  for (const o of smartOrders) {
    const isTrailing = o.order_type === 'trailing_stop'
    rows.push({
      kind: 'smart', id: o.id, credentialId: o.credential_id, exchange: o.exchange,
      symbol: o.symbol, side: o.side, category: o.category, leverage: o.leverage,
      type: o.order_type.replace(/_/g, ' '), quantity: o.quantity,
      price: isTrailing ? o.callback_rate : o.trigger_price,
      priceLabel: isTrailing ? `${o.callback_rate}%` : o.trigger_price > 0 ? fmt$(o.trigger_price) : '—',
      status: o.status, createdAt: o.created_at, cancellable: o.status === 'active',
    })
  }
  rows.sort((a, b) => new Date(b.createdAt).getTime() - new Date(a.createdAt).getTime())
  return rows
}

// ─── Orders List ──────────────────────────────────────────────────────────────

function OrdersList() {
  const qc = useQueryClient()
  const [filterCredId, setFilterCredId] = useState('')

  const { data: credentials = [] } = useQuery({ queryKey: ['credentials'], queryFn: getCredentials, staleTime: 30_000 })
  const { data: groups = [] } = useQuery({ queryKey: ['credential-groups'], queryFn: listCredentialGroups, staleTime: 30_000 })
  const activeCreds = credentials.filter((c) => c.is_active)

  let filterCredIDs: number[] | null = null
  if (filterCredId.startsWith('cred:')) filterCredIDs = [parseInt(filterCredId.slice(5))]
  else if (filterCredId.startsWith('group:')) {
    const g = groups.find((x) => x.id === parseInt(filterCredId.slice(6)))
    if (g) filterCredIDs = g.member_ids
  }

  const { data: orders = [], isLoading: ordersLoading } = useQuery({
    queryKey: ['orders', filterCredIDs],
    queryFn: () => listOrders(filterCredIDs ? { credential_ids: filterCredIDs!.join(',') } : undefined),
    refetchInterval: 10_000,
  })
  const { data: smartOrders = [], isLoading: smartLoading } = useQuery({
    queryKey: ['smart-orders'],
    queryFn: listSmartOrders,
    refetchInterval: 10_000,
  })

  const cancelOrderMut = useMutation({ mutationFn: cancelOrder, onSuccess: () => qc.invalidateQueries({ queryKey: ['orders'] }) })
  const cancelSmartMut = useMutation({ mutationFn: cancelSmartOrder, onSuccess: () => qc.invalidateQueries({ queryKey: ['smart-orders'] }) })

  const filteredSmartOrders = filterCredIDs
    ? smartOrders.filter((so) => so.credential_id && filterCredIDs!.includes(so.credential_id))
    : smartOrders

  const rows = mergeRows(orders, filteredSmartOrders)
  const credMap = new Map(activeCreds.map((c) => [c.id, credLabel(c)]))

  if (ordersLoading || smartLoading) return <Spinner />

  return (
    <div className="bg-slate-800 rounded-xl border border-slate-700 overflow-hidden">
      <div className="px-5 py-4 border-b border-slate-700 flex items-center justify-between gap-3">
        <select
          value={filterCredId}
          onChange={(e) => setFilterCredId(e.target.value)}
          className="bg-slate-700 border border-slate-600 text-slate-200 text-sm rounded-lg px-3 py-1.5 focus:outline-none focus:ring-1 focus:ring-blue-500"
        >
          <option value="">All orders</option>
          {activeCreds.length > 0 && (
            <optgroup label="API Keys">
              {activeCreds.map((c) => <option key={`cred:${c.id}`} value={`cred:${c.id}`}>{credLabel(c)}</option>)}
            </optgroup>
          )}
          {groups.length > 0 && (
            <optgroup label="Groups">
              {groups.map((g) => <option key={`group:${g.id}`} value={`group:${g.id}`}>{g.name}</option>)}
            </optgroup>
          )}
        </select>
        <h2 className="font-semibold text-white text-sm">Orders ({rows.length})</h2>
      </div>

      {rows.length === 0 ? (
        <p className="px-5 py-6 text-sm text-slate-500">No orders found.</p>
      ) : (
        <div className="overflow-x-auto">
          <table className="w-full text-sm">
            <thead>
              <tr className="text-xs text-slate-400 border-b border-slate-700">
                {['ID', 'API Key', 'Symbol', 'Side', 'Cat.', 'Lev.', 'Type', 'Qty', 'Price / Trigger', 'Status', 'Created', ''].map(
                  (h) => <th key={h} className="px-4 py-3 text-left font-medium whitespace-nowrap">{h}</th>,
                )}
              </tr>
            </thead>
            <tbody>
              {rows.map((r) => (
                <tr key={`${r.kind}-${r.id}`} className="border-b border-slate-700/50 hover:bg-slate-700/40 transition-colors">
                  <td className="px-4 py-3 text-slate-500 font-mono text-xs">{r.kind === 'smart' ? 'S' : ''}#{r.id}</td>
                  <td className="px-4 py-3 text-slate-300 text-xs">{r.credentialId ? (credMap.get(r.credentialId) ?? r.exchange) : r.exchange}</td>
                  <td className="px-4 py-3 font-medium text-white">{r.symbol}</td>
                  <td className="px-4 py-3">
                    <span className={`text-xs font-medium px-2 py-0.5 rounded-full ${r.side === 'buy' ? 'bg-green-900/50 text-green-400' : 'bg-red-900/50 text-red-400'}`}>
                      {r.side.toUpperCase()}
                    </span>
                  </td>
                  <td className="px-4 py-3 text-slate-400 text-xs capitalize">{r.category}</td>
                  <td className="px-4 py-3 text-slate-400 text-xs">{r.leverage || '—'}</td>
                  <td className="px-4 py-3 text-slate-300 capitalize text-xs">{r.type}</td>
                  <td className="px-4 py-3 text-slate-300">{r.quantity}</td>
                  <td className="px-4 py-3 text-slate-300">{r.priceLabel}</td>
                  <td className="px-4 py-3"><StatusBadge status={r.status} /></td>
                  <td className="px-4 py-3 text-slate-400 whitespace-nowrap text-xs">{fmtDate(r.createdAt)}</td>
                  <td className="px-4 py-3">
                    {r.cancellable && (
                      <button
                        onClick={() => r.kind === 'order' ? cancelOrderMut.mutate(r.id) : cancelSmartMut.mutate(r.id)}
                        disabled={cancelOrderMut.isPending || cancelSmartMut.isPending}
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

export default function Orders() {
  const [symbol, setSymbol] = useState('BTC')

  return (
    <div className="space-y-6">
      <h1 className="text-2xl font-bold text-white">Orders</h1>
      <OrdersList />
      <SymbolPanel symbol={symbol} onSymbolChange={setSymbol} />
      <OrderForm symbol={`${symbol}-USDT-SWAP`} />
    </div>
  )
}
