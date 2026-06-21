import { useState, useRef, type FormEvent } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { Trash2, Plus, ChevronLeft, ChevronRight, Check, X } from 'lucide-react'
import {
  getPortfolio,
  getHistory,
  getCredentials,
  addCredential,
  deleteCredential,
  updatePositionComment,
  updateHistoryComment,
  type AddCredentialPayload,
  type Position,
  type HistoryEntry,
  type Credential,
} from '../api'

// ─── Helpers ──────────────────────────────────────────────────────────────────

const fmt$ = (v: number) =>
  `$${v.toLocaleString('en-US', { minimumFractionDigits: 2, maximumFractionDigits: 2 })}`

const fmtDate = (iso: string) =>
  new Date(iso).toLocaleDateString('en-US', {
    year: 'numeric',
    month: 'short',
    day: 'numeric',
  })

const EXCHANGES = ['binance', 'okx', 'bybit', 'kucoin']

// ─── Inline editable comment ──────────────────────────────────────────────────

function EditableComment({
  value,
  onSave,
}: {
  value: string
  onSave: (v: string) => Promise<void>
}) {
  const [editing, setEditing] = useState(false)
  const [text, setText] = useState(value)
  const [saving, setSaving] = useState(false)

  const save = async () => {
    setSaving(true)
    try {
      await onSave(text)
      setEditing(false)
    } finally {
      setSaving(false)
    }
  }

  if (!editing) {
    return (
      <span
        className="text-slate-400 cursor-pointer hover:text-slate-200 text-xs italic"
        onClick={() => setEditing(true)}
      >
        {text || 'Add comment…'}
      </span>
    )
  }

  return (
    <div className="flex items-center gap-1">
      <input
        autoFocus
        value={text}
        onChange={(e) => setText(e.target.value)}
        className="bg-slate-700 border border-slate-600 rounded px-2 py-0.5 text-xs text-white focus:outline-none focus:border-blue-500 w-32"
        onKeyDown={(e) => {
          if (e.key === 'Enter') void save()
          if (e.key === 'Escape') setEditing(false)
        }}
      />
      <button
        onClick={() => void save()}
        disabled={saving}
        className="text-green-400 hover:text-green-300 disabled:opacity-50"
      >
        <Check size={13} />
      </button>
      <button onClick={() => setEditing(false)} className="text-slate-400 hover:text-slate-200">
        <X size={13} />
      </button>
    </div>
  )
}

// ─── Positions tab ────────────────────────────────────────────────────────────

function PositionsTab() {
  const qc = useQueryClient()
  const { data, isLoading } = useQuery({
    queryKey: ['portfolio'],
    queryFn: getPortfolio,
  })

  const commentMutation = useMutation({
    mutationFn: ({ id, comment }: { id: number; comment: string }) =>
      updatePositionComment(id, comment),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['portfolio'] }),
  })

  const positions: Position[] = data?.positions ?? []

  if (isLoading) return <Spinner />

  if (positions.length === 0)
    return <p className="text-slate-500 text-sm py-6">No open positions found.</p>

  return (
    <div className="overflow-x-auto">
      <table className="w-full text-sm">
        <thead>
          <tr className="text-xs text-slate-400 border-b border-slate-700">
            {['Symbol', 'Exchange', 'Side', 'Qty', 'Avg Price', 'Mark Price', 'Unr. PnL', 'Comment'].map(
              (h) => (
                <th key={h} className="px-4 py-3 text-left font-medium whitespace-nowrap">
                  {h}
                </th>
              ),
            )}
          </tr>
        </thead>
        <tbody>
          {positions.map((p) => (
            <tr
              key={p.id}
              className="border-b border-slate-700/50 hover:bg-slate-700/40 transition-colors"
            >
              <td className="px-4 py-3 font-medium text-white">{p.symbol}</td>
              <td className="px-4 py-3 text-slate-300 capitalize">{p.exchange}</td>
              <td className="px-4 py-3">
                <SideBadge side={p.side} />
              </td>
              <td className="px-4 py-3 text-slate-300">{p.quantity}</td>
              <td className="px-4 py-3 text-slate-300">{fmt$(p.avg_price)}</td>
              <td className="px-4 py-3 text-slate-300">{fmt$(p.mark_price)}</td>
              <td
                className={`px-4 py-3 font-medium ${
                  p.unrealized_pnl >= 0 ? 'text-green-400' : 'text-red-400'
                }`}
              >
                {p.unrealized_pnl >= 0 ? '+' : ''}
                {fmt$(p.unrealized_pnl)}
              </td>
              <td className="px-4 py-3">
                <EditableComment
                  value={p.comment}
                  onSave={(comment) =>
                    commentMutation.mutateAsync({ id: p.id, comment }).then(() => undefined)
                  }
                />
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  )
}

// ─── History tab ──────────────────────────────────────────────────────────────

function HistoryTab() {
  const qc = useQueryClient()
  const [offset, setOffset] = useState(0)
  const LIMIT = 15

  const { data, isLoading } = useQuery({
    queryKey: ['history', LIMIT, offset],
    queryFn: () => getHistory(LIMIT, offset),
  })

  const commentMutation = useMutation({
    mutationFn: ({ id, comment }: { id: number; comment: string }) =>
      updateHistoryComment(id, comment),
    onSuccess: () => qc.invalidateQueries({ queryKey: ['history'] }),
  })

  const entries: HistoryEntry[] = data?.data ?? []
  const total = data?.total ?? 0
  const totalPages = Math.ceil(total / LIMIT)
  const page = Math.floor(offset / LIMIT) + 1

  if (isLoading) return <Spinner />

  return (
    <div>
      <div className="overflow-x-auto">
        <table className="w-full text-sm">
          <thead>
            <tr className="text-xs text-slate-400 border-b border-slate-700">
              {[
                'Symbol',
                'Exchange',
                'Side',
                'Qty',
                'Entry',
                'Exit',
                'Realized PnL',
                'Date',
                'Comment',
              ].map((h) => (
                <th key={h} className="px-4 py-3 text-left font-medium whitespace-nowrap">
                  {h}
                </th>
              ))}
            </tr>
          </thead>
          <tbody>
            {entries.length === 0 ? (
              <tr>
                <td colSpan={9} className="px-4 py-6 text-slate-500 text-center text-sm">
                  No trade history found.
                </td>
              </tr>
            ) : (
              entries.map((e) => (
                <tr
                  key={e.id}
                  className="border-b border-slate-700/50 hover:bg-slate-700/40 transition-colors"
                >
                  <td className="px-4 py-3 font-medium text-white">{e.symbol}</td>
                  <td className="px-4 py-3 text-slate-300 capitalize">{e.exchange}</td>
                  <td className="px-4 py-3">
                    <SideBadge side={e.side} />
                  </td>
                  <td className="px-4 py-3 text-slate-300">{e.quantity}</td>
                  <td className="px-4 py-3 text-slate-300">{fmt$(e.entry_price)}</td>
                  <td className="px-4 py-3 text-slate-300">{fmt$(e.exit_price)}</td>
                  <td
                    className={`px-4 py-3 font-medium ${
                      e.realized_pnl >= 0 ? 'text-green-400' : 'text-red-400'
                    }`}
                  >
                    {e.realized_pnl >= 0 ? '+' : ''}
                    {fmt$(e.realized_pnl)}
                  </td>
                  <td className="px-4 py-3 text-slate-300 whitespace-nowrap">
                    {fmtDate(e.closed_at)}
                  </td>
                  <td className="px-4 py-3">
                    <EditableComment
                      value={e.comment}
                      onSave={(comment) =>
                        commentMutation.mutateAsync({ id: e.id, comment }).then(() => undefined)
                      }
                    />
                  </td>
                </tr>
              ))
            )}
          </tbody>
        </table>
      </div>

      {/* Pagination */}
      {totalPages > 1 && (
        <div className="flex items-center justify-between px-4 py-3 border-t border-slate-700">
          <p className="text-xs text-slate-400">
            Page {page} of {totalPages} ({total} trades)
          </p>
          <div className="flex gap-2">
            <button
              disabled={offset === 0}
              onClick={() => setOffset((o) => Math.max(0, o - LIMIT))}
              className="p-1.5 rounded bg-slate-700 hover:bg-slate-600 disabled:opacity-40 disabled:cursor-not-allowed text-slate-300"
            >
              <ChevronLeft size={16} />
            </button>
            <button
              disabled={offset + LIMIT >= total}
              onClick={() => setOffset((o) => o + LIMIT)}
              className="p-1.5 rounded bg-slate-700 hover:bg-slate-600 disabled:opacity-40 disabled:cursor-not-allowed text-slate-300"
            >
              <ChevronRight size={16} />
            </button>
          </div>
        </div>
      )}
    </div>
  )
}

// ─── Credentials tab ──────────────────────────────────────────────────────────

function CredentialsTab() {
  const qc = useQueryClient()
  const [showForm, setShowForm] = useState(false)
  const [formError, setFormError] = useState('')
  const formRef = useRef<HTMLFormElement>(null)

  const { data: creds = [], isLoading } = useQuery({
    queryKey: ['credentials'],
    queryFn: getCredentials,
  })

  const addMutation = useMutation({
    mutationFn: addCredential,
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: ['credentials'] })
      setShowForm(false)
      formRef.current?.reset()
      setFormError('')
    },
    onError: () => setFormError('Failed to add credential.'),
  })

  const deleteMutation = useMutation({
    mutationFn: deleteCredential,
    onSuccess: () => qc.invalidateQueries({ queryKey: ['credentials'] }),
  })

  const [exchange, setExchange] = useState('binance')

  const handleAdd = (e: FormEvent<HTMLFormElement>) => {
    e.preventDefault()
    const fd = new FormData(e.currentTarget)
    const payload: AddCredentialPayload = {
      exchange: fd.get('exchange') as string,
      label: fd.get('label') as string,
      api_key: fd.get('api_key') as string,
      api_secret: fd.get('api_secret') as string,
    }
    const passphrase = fd.get('passphrase') as string
    if (passphrase) payload.passphrase = passphrase
    addMutation.mutate(payload)
  }

  const needsPassphrase = exchange === 'okx' || exchange === 'kucoin'

  const maskKey = (key: string) =>
    key.length > 8 ? key.slice(0, 4) + '••••••••' + key.slice(-4) : '••••••••'

  if (isLoading) return <Spinner />

  return (
    <div className="space-y-4">
      {/* List */}
      {creds.length === 0 ? (
        <p className="text-slate-500 text-sm py-4">
          No API credentials added yet.
        </p>
      ) : (
        <div className="space-y-2">
          {creds.map((c: Credential) => (
            <div
              key={c.id}
              className="flex items-center justify-between p-4 bg-slate-700/50 rounded-lg border border-slate-700"
            >
              <div>
                <p className="text-sm font-medium text-white capitalize">
                  {c.exchange}{' '}
                  <span className="text-slate-400 font-normal">— {c.label}</span>
                </p>
                <p className="text-xs text-slate-500 mt-0.5 font-mono">
                  {maskKey(c.api_key)}
                </p>
                <p className="text-xs text-slate-600 mt-0.5">
                  Added {fmtDate(c.created_at)}
                </p>
              </div>
              <button
                onClick={() => deleteMutation.mutate(c.id)}
                disabled={deleteMutation.isPending}
                className="text-slate-400 hover:text-red-400 transition-colors p-1.5"
              >
                <Trash2 size={16} />
              </button>
            </div>
          ))}
        </div>
      )}

      {/* Add button */}
      {!showForm && (
        <button
          onClick={() => setShowForm(true)}
          className="flex items-center gap-2 px-4 py-2 bg-blue-600 hover:bg-blue-500 text-white text-sm font-medium rounded-lg transition-colors"
        >
          <Plus size={15} /> Add API Key
        </button>
      )}

      {/* Add form */}
      {showForm && (
        <form
          ref={formRef}
          onSubmit={handleAdd}
          className="bg-slate-700/40 rounded-xl border border-slate-700 p-5 space-y-3"
        >
          <h3 className="font-medium text-white text-sm">Add API Credential</h3>

          {formError && (
            <p className="text-xs text-red-400 bg-red-900/30 px-3 py-2 rounded">{formError}</p>
          )}

          <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
            <div>
              <label className="label-sm">Exchange</label>
              <select
                name="exchange"
                value={exchange}
                onChange={(e) => setExchange(e.target.value)}
                className="input-field"
              >
                {EXCHANGES.map((ex) => (
                  <option key={ex} value={ex}>
                    {ex.toUpperCase()}
                  </option>
                ))}
              </select>
            </div>
            <div>
              <label className="label-sm">Label</label>
              <input name="label" required placeholder="My Binance Key" className="input-field" />
            </div>
            <div>
              <label className="label-sm">API Key</label>
              <input name="api_key" required placeholder="API Key" className="input-field" />
            </div>
            <div>
              <label className="label-sm">API Secret</label>
              <input
                name="api_secret"
                required
                type="password"
                placeholder="API Secret"
                className="input-field"
              />
            </div>
            {needsPassphrase && (
              <div>
                <label className="label-sm">Passphrase</label>
                <input
                  name="passphrase"
                  type="password"
                  placeholder="Passphrase"
                  className="input-field"
                />
              </div>
            )}
          </div>

          <div className="flex gap-2 pt-1">
            <button
              type="submit"
              disabled={addMutation.isPending}
              className="px-4 py-2 bg-blue-600 hover:bg-blue-500 disabled:opacity-50 text-white text-sm font-medium rounded-lg transition-colors"
            >
              {addMutation.isPending ? 'Saving…' : 'Save'}
            </button>
            <button
              type="button"
              onClick={() => setShowForm(false)}
              className="px-4 py-2 bg-slate-700 hover:bg-slate-600 text-slate-300 text-sm rounded-lg transition-colors"
            >
              Cancel
            </button>
          </div>
        </form>
      )}
    </div>
  )
}

// ─── Shared components ────────────────────────────────────────────────────────

function SideBadge({ side }: { side: string }) {
  return (
    <span
      className={`text-xs font-medium px-2 py-0.5 rounded-full ${
        side === 'long' || side === 'buy'
          ? 'bg-green-900/50 text-green-400'
          : 'bg-red-900/50 text-red-400'
      }`}
    >
      {side.toUpperCase()}
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

// ─── Portfolio page ───────────────────────────────────────────────────────────

const TABS = ['Positions', 'History', 'Credentials'] as const
type Tab = (typeof TABS)[number]

export default function Portfolio() {
  const [tab, setTab] = useState<Tab>('Positions')

  return (
    <div className="space-y-6">
      <h1 className="text-2xl font-bold text-white">Portfolio</h1>

      {/* Tab bar */}
      <div className="flex gap-1 bg-slate-800/50 p-1 rounded-lg border border-slate-700 w-fit">
        {TABS.map((t) => (
          <button
            key={t}
            onClick={() => setTab(t)}
            className={`px-4 py-1.5 text-sm font-medium rounded-md transition-colors ${
              tab === t
                ? 'bg-blue-600 text-white'
                : 'text-slate-400 hover:text-white'
            }`}
          >
            {t}
          </button>
        ))}
      </div>

      {/* Content */}
      <div className="bg-slate-800 rounded-xl border border-slate-700 overflow-hidden">
        {tab === 'Positions' && <PositionsTab />}
        {tab === 'History' && <HistoryTab />}
        {tab === 'Credentials' && (
          <div className="p-5">
            <CredentialsTab />
          </div>
        )}
      </div>
    </div>
  )
}
