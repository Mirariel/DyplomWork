import { useState } from 'react'
import { useQuery, useMutation } from '@tanstack/react-query'
import { X, ExternalLink } from 'lucide-react'
import { getCredentials, placeOrder, createBot, createDCA } from '../api'
import type { PlaceOrderPayload, CreateBotPayload, CreateDCAPayload } from '../api'
import PriceChart from './PriceChart'

// ─── Types ────────────────────────────────────────────────────────────────────

export interface CoinModalProps {
  symbol: string   // base symbol, e.g. "BTC"
  price: number    // current price from WS
  onClose: () => void
}

type Tab = 'info' | 'news' | 'trade' | 'bot'
type GridPreset = 'conservative' | 'moderate' | 'aggressive'
type BotType = 'grid' | 'dca'

const TABS: { key: Tab; label: string }[] = [
  { key: 'info', label: 'Info' },
  { key: 'news', label: 'News' },
  { key: 'trade', label: 'Trade' },
  { key: 'bot', label: 'Bot' },
]

const GRID_PRESETS: Record<GridPreset, { grids: number; rangePercent: number; label: string; color: string; desc: string }> = {
  conservative: { grids: 10, rangePercent: 10, label: 'Conservative', color: 'text-blue-400',   desc: 'Narrow range, steady profits' },
  moderate:     { grids: 15, rangePercent: 20, label: 'Moderate',     color: 'text-yellow-400', desc: 'Balanced risk & reward' },
  aggressive:   { grids: 20, rangePercent: 40, label: 'Aggressive',   color: 'text-red-400',    desc: 'Wide range, high potential' },
}

const NEWS_SOURCES = [
  { name: 'CoinDesk',      url: (s: string) => `https://www.coindesk.com/search/?s=${s}` },
  { name: 'CoinTelegraph', url: (s: string) => `https://cointelegraph.com/search?query=${s}` },
  { name: 'Decrypt',       url: (s: string) => `https://decrypt.co/search?q=${s}` },
  { name: 'The Block',     url: (s: string) => `https://www.theblock.co/search?q=${s}` },
  { name: 'CryptoPanic',   url: (s: string) => `https://cryptopanic.com/?currencies=${s}` },
  { name: 'Google News',   url: (s: string) => `https://news.google.com/search?q=${s}+crypto` },
]

const fmt$ = (v: number) =>
  `$${v.toLocaleString('en-US', { minimumFractionDigits: 2, maximumFractionDigits: 2 })}`

// ─── Info Tab ─────────────────────────────────────────────────────────────────

interface Ticker24h {
  priceChangePercent: string
  highPrice: string
  lowPrice: string
  volume: string
  quoteVolume: string
  openPrice: string
  weightedAvgPrice: string
  count: number
}

function InfoTab({ symbol, currentPrice }: { symbol: string; currentPrice: number }) {
  const { data: ticker, isLoading } = useQuery({
    queryKey: ['ticker24h', symbol],
    queryFn: async () => {
      const res = await fetch(`https://api.binance.com/api/v3/ticker/24hr?symbol=${symbol}USDT`)
      if (!res.ok) throw new Error('Failed')
      return res.json() as Promise<Ticker24h>
    },
    staleTime: 30_000,
  })

  if (isLoading) return <p className="text-slate-500 text-sm p-5">Loading market data…</p>
  if (!ticker)   return <p className="text-slate-500 text-sm p-5">No data available.</p>

  const changePct = parseFloat(ticker.priceChangePercent)
  const positive  = changePct >= 0

  const stats = [
    { label: '24h Change',    value: `${positive ? '+' : ''}${changePct.toFixed(2)}%`, color: positive ? 'text-green-400' : 'text-red-400' },
    { label: 'Current Price', value: fmt$(currentPrice) },
    { label: '24h High',      value: fmt$(parseFloat(ticker.highPrice)) },
    { label: '24h Low',       value: fmt$(parseFloat(ticker.lowPrice)) },
    { label: 'Open Price',    value: fmt$(parseFloat(ticker.openPrice)) },
    { label: 'Avg Price',     value: fmt$(parseFloat(ticker.weightedAvgPrice)) },
    { label: 'Volume (24h)',  value: `${parseFloat(ticker.volume).toLocaleString('en-US', { maximumFractionDigits: 2 })} ${symbol}` },
    { label: 'Turnover',      value: `$${(parseFloat(ticker.quoteVolume) / 1_000_000).toFixed(1)}M` },
    { label: 'Trades (24h)',  value: ticker.count.toLocaleString() },
  ]

  return (
    <div className="p-4">
      <div className="grid grid-cols-2 sm:grid-cols-3 gap-2">
        {stats.map((s) => (
          <div key={s.label} className="bg-slate-700/40 rounded-lg p-3">
            <p className="text-xs text-slate-400 mb-1">{s.label}</p>
            <p className={`text-sm font-semibold ${s.color ?? 'text-white'}`}>{s.value}</p>
          </div>
        ))}
      </div>
    </div>
  )
}

// ─── News Tab ─────────────────────────────────────────────────────────────────

function NewsTab({ symbol }: { symbol: string }) {
  return (
    <div className="p-4 space-y-3">
      <p className="text-xs text-slate-400">Latest {symbol} news from popular sources:</p>
      <div className="grid grid-cols-2 gap-2">
        {NEWS_SOURCES.map((src) => (
          <a
            key={src.name}
            href={src.url(symbol)}
            target="_blank"
            rel="noopener noreferrer"
            className="flex items-center gap-2 bg-slate-700/40 hover:bg-slate-700 rounded-lg p-3 transition-colors group"
          >
            <ExternalLink size={13} className="text-slate-400 group-hover:text-blue-400 transition-colors flex-shrink-0" />
            <span className="text-sm text-slate-300 group-hover:text-white transition-colors">{src.name}</span>
          </a>
        ))}
      </div>
    </div>
  )
}

// ─── Trade Tab ────────────────────────────────────────────────────────────────

function TradeTab({ symbol, currentPrice }: { symbol: string; currentPrice: number }) {
  const { data: creds = [] } = useQuery({ queryKey: ['credentials'], queryFn: getCredentials })
  const [side, setSide]         = useState<'buy' | 'sell'>('buy')
  const [orderType, setOrderType] = useState<'market' | 'limit'>('market')
  const [exchange, setExchange] = useState('')
  const [quantity, setQuantity] = useState('')
  const [price, setPrice]       = useState(currentPrice.toFixed(2))
  const [msg, setMsg]           = useState('')

  const exchanges    = [...new Set(creds.map((c) => c.exchange))]
  const activeEx     = exchange || exchanges[0] || ''
  const limitPrice   = parseFloat(price || '0')
  const qty          = parseFloat(quantity || '0')
  const estTotal     = qty * (orderType === 'limit' ? limitPrice : currentPrice)

  const mut = useMutation({
    mutationFn: placeOrder,
    onSuccess: () => { setMsg('Order placed!'); setQuantity('') },
    onError: (e: Error) => setMsg(`Error: ${e.message}`),
  })

  const submit = () => {
    if (!qty || !activeEx) return
    const payload: PlaceOrderPayload = {
      exchange: activeEx,
      symbol: `${symbol}USDT`,
      side,
      category: 'spot',
      type: orderType,
      quantity: qty,
      ...(orderType === 'limit' ? { price: limitPrice } : {}),
    }
    mut.mutate(payload)
  }

  return (
    <div className="p-4 space-y-4">
      {exchanges.length > 0 ? (
        <>
          {/* Exchange */}
          <div>
            <label className="block text-xs text-slate-400 mb-1.5">Exchange</label>
            <select
              value={activeEx}
              onChange={(e) => setExchange(e.target.value)}
              className="w-full bg-slate-700 border border-slate-600 text-slate-200 text-sm rounded-lg px-3 py-2 focus:outline-none focus:border-blue-500"
            >
              {exchanges.map((ex) => (
                <option key={ex} value={ex}>{ex.charAt(0).toUpperCase() + ex.slice(1)}</option>
              ))}
            </select>
          </div>

          {/* Side */}
          <div className="flex rounded-lg overflow-hidden border border-slate-600">
            {(['buy', 'sell'] as const).map((s) => (
              <button
                key={s}
                onClick={() => setSide(s)}
                className={`flex-1 py-2.5 text-sm font-semibold capitalize transition-colors ${
                  side === s
                    ? s === 'buy' ? 'bg-green-600 text-white' : 'bg-red-600 text-white'
                    : 'bg-slate-700 text-slate-400 hover:text-white'
                }`}
              >{s}</button>
            ))}
          </div>

          {/* Order type */}
          <div className="flex gap-2">
            {(['market', 'limit'] as const).map((t) => (
              <button
                key={t}
                onClick={() => setOrderType(t)}
                className={`px-3 py-1.5 text-xs rounded-lg border capitalize transition-colors ${
                  orderType === t ? 'bg-blue-600 border-blue-600 text-white' : 'border-slate-600 text-slate-400 hover:text-white'
                }`}
              >{t}</button>
            ))}
          </div>

          {/* Quantity */}
          <div>
            <label className="block text-xs text-slate-400 mb-1.5">Quantity ({symbol})</label>
            <input
              type="number" step="any" min="0"
              value={quantity}
              onChange={(e) => setQuantity(e.target.value)}
              placeholder="0.00"
              className="w-full bg-slate-700 border border-slate-600 text-slate-200 text-sm rounded-lg px-3 py-2 focus:outline-none focus:border-blue-500"
            />
          </div>

          {/* Limit price */}
          {orderType === 'limit' && (
            <div>
              <label className="block text-xs text-slate-400 mb-1.5">Price (USDT)</label>
              <input
                type="number" step="any" min="0"
                value={price}
                onChange={(e) => setPrice(e.target.value)}
                className="w-full bg-slate-700 border border-slate-600 text-slate-200 text-sm rounded-lg px-3 py-2 focus:outline-none focus:border-blue-500"
              />
            </div>
          )}

          {estTotal > 0 && (
            <p className="text-xs text-slate-400">
              Est. total: <span className="text-white font-medium">${estTotal.toFixed(2)} USDT</span>
            </p>
          )}

          {msg && <p className={`text-xs ${msg.startsWith('Error') ? 'text-red-400' : 'text-green-400'}`}>{msg}</p>}

          <button
            onClick={submit}
            disabled={!qty || !activeEx || mut.isPending}
            className={`w-full py-2.5 text-sm font-semibold rounded-lg transition-colors disabled:opacity-50 ${
              side === 'buy' ? 'bg-green-600 hover:bg-green-500 text-white' : 'bg-red-600 hover:bg-red-500 text-white'
            }`}
          >
            {mut.isPending ? 'Placing…' : `${side === 'buy' ? 'Buy' : 'Sell'} ${symbol}`}
          </button>
        </>
      ) : (
        <p className="text-slate-500 text-sm">No exchange credentials configured. Add them in Portfolio → Credentials.</p>
      )}
    </div>
  )
}

// ─── Bot Tab ──────────────────────────────────────────────────────────────────

function BotTab({ symbol, currentPrice }: { symbol: string; currentPrice: number }) {
  const { data: creds = [] } = useQuery({ queryKey: ['credentials'], queryFn: getCredentials })
  const [botType, setBotType]   = useState<BotType>('grid')
  const [exchange, setExchange] = useState('')
  const [preset, setPreset]     = useState<GridPreset>('moderate')
  const [invest, setInvest]     = useState('100')
  const [dcaAmt, setDcaAmt]     = useState('50')
  const [dcaH, setDcaH]         = useState('24')
  const [msg, setMsg]           = useState('')

  const exchanges = [...new Set(creds.map((c) => c.exchange))]
  const activeEx  = exchange || exchanges[0] || ''

  const p          = GRID_PRESETS[preset]
  const priceLow   = currentPrice * (1 - p.rangePercent / 200)
  const priceHigh  = currentPrice * (1 + p.rangePercent / 200)
  const qtyPerGrid = parseFloat(invest) / p.grids / currentPrice

  const gridMut = useMutation({
    mutationFn: createBot,
    onSuccess: () => setMsg('Grid bot created! See Bots page.'),
    onError:   (e: Error) => setMsg(`Error: ${e.message}`),
  })
  const dcaMut = useMutation({
    mutationFn: createDCA,
    onSuccess: () => setMsg('DCA bot created! See DCA Bots page.'),
    onError:   (e: Error) => setMsg(`Error: ${e.message}`),
  })

  const createGrid = () => {
    if (!activeEx) return
    const payload: CreateBotPayload = {
      exchange: activeEx,
      symbol: `${symbol}USDT`,
      category: 'spot',
      price_low:    parseFloat(priceLow.toFixed(8)),
      price_high:   parseFloat(priceHigh.toFixed(8)),
      grids:        p.grids,
      qty_per_grid: parseFloat(qtyPerGrid.toFixed(8)),
    }
    gridMut.mutate(payload)
  }

  const createDCABot = () => {
    if (!activeEx) return
    const payload: CreateDCAPayload = {
      exchange: activeEx,
      symbol: `${symbol}USDT`,
      category: 'spot',
      amount_usd:     parseFloat(dcaAmt),
      interval_hours: parseInt(dcaH),
    }
    dcaMut.mutate(payload)
  }

  if (exchanges.length === 0) {
    return <p className="text-slate-500 text-sm p-5">No exchange credentials configured.</p>
  }

  return (
    <div className="p-4 space-y-4">
      {/* Exchange */}
      <div>
        <label className="block text-xs text-slate-400 mb-1.5">Exchange</label>
        <select
          value={activeEx}
          onChange={(e) => setExchange(e.target.value)}
          className="w-full bg-slate-700 border border-slate-600 text-slate-200 text-sm rounded-lg px-3 py-2 focus:outline-none focus:border-blue-500"
        >
          {exchanges.map((ex) => (
            <option key={ex} value={ex}>{ex.charAt(0).toUpperCase() + ex.slice(1)}</option>
          ))}
        </select>
      </div>

      {/* Bot type switch */}
      <div className="flex rounded-lg overflow-hidden border border-slate-600">
        {(['grid', 'dca'] as const).map((t) => (
          <button
            key={t}
            onClick={() => { setBotType(t); setMsg('') }}
            className={`flex-1 py-2.5 text-sm font-semibold transition-colors ${
              botType === t ? 'bg-blue-600 text-white' : 'bg-slate-700 text-slate-400 hover:text-white'
            }`}
          >{t === 'grid' ? 'Grid Bot' : 'DCA Bot'}</button>
        ))}
      </div>

      {botType === 'grid' && (
        <>
          {/* Presets */}
          <div>
            <label className="block text-xs text-slate-400 mb-2">Strategy Preset</label>
            <div className="grid grid-cols-3 gap-2">
              {(Object.keys(GRID_PRESETS) as GridPreset[]).map((key) => {
                const pr = GRID_PRESETS[key]
                return (
                  <button
                    key={key}
                    onClick={() => setPreset(key)}
                    className={`rounded-lg p-2.5 border text-left transition-colors ${
                      preset === key ? 'border-blue-500 bg-blue-500/10' : 'border-slate-600 bg-slate-700/40 hover:border-slate-500'
                    }`}
                  >
                    <p className={`text-xs font-semibold ${pr.color}`}>{pr.label}</p>
                    <p className="text-xs text-slate-500 mt-0.5 leading-tight">{pr.desc}</p>
                  </button>
                )
              })}
            </div>
          </div>

          {/* Preset details */}
          <div className="bg-slate-700/30 rounded-lg p-3 grid grid-cols-2 gap-y-1.5 text-xs">
            <div><span className="text-slate-400">Low: </span><span className="text-white">{fmt$(priceLow)}</span></div>
            <div><span className="text-slate-400">High: </span><span className="text-white">{fmt$(priceHigh)}</span></div>
            <div><span className="text-slate-400">Grids: </span><span className="text-white">{p.grids}</span></div>
            <div><span className="text-slate-400">Range: </span><span className="text-white">±{p.rangePercent / 2}%</span></div>
            <div className="col-span-2"><span className="text-slate-400">Qty/grid: </span><span className="text-white">{isFinite(qtyPerGrid) ? qtyPerGrid.toFixed(6) : '—'} {symbol}</span></div>
          </div>

          {/* Investment */}
          <div>
            <label className="block text-xs text-slate-400 mb-1.5">Total Investment (USDT)</label>
            <input
              type="number" step="1" min="1"
              value={invest}
              onChange={(e) => setInvest(e.target.value)}
              className="w-full bg-slate-700 border border-slate-600 text-slate-200 text-sm rounded-lg px-3 py-2 focus:outline-none focus:border-blue-500"
            />
          </div>

          {msg && <p className={`text-xs ${msg.startsWith('Error') ? 'text-red-400' : 'text-green-400'}`}>{msg}</p>}

          <button
            onClick={createGrid}
            disabled={!activeEx || gridMut.isPending}
            className="w-full py-2.5 text-sm font-semibold rounded-lg bg-blue-600 hover:bg-blue-500 text-white transition-colors disabled:opacity-50"
          >
            {gridMut.isPending ? 'Creating…' : 'Create Grid Bot'}
          </button>
        </>
      )}

      {botType === 'dca' && (
        <>
          <div>
            <label className="block text-xs text-slate-400 mb-1.5">Buy Amount per Interval (USDT)</label>
            <input
              type="number" step="1" min="1"
              value={dcaAmt}
              onChange={(e) => setDcaAmt(e.target.value)}
              className="w-full bg-slate-700 border border-slate-600 text-slate-200 text-sm rounded-lg px-3 py-2 focus:outline-none focus:border-blue-500"
            />
          </div>

          <div>
            <label className="block text-xs text-slate-400 mb-2">Interval</label>
            <div className="flex gap-2 flex-wrap">
              {[1, 4, 8, 12, 24, 48, 168].map((h) => (
                <button
                  key={h}
                  onClick={() => setDcaH(String(h))}
                  className={`px-3 py-1.5 text-xs rounded-lg border transition-colors ${
                    dcaH === String(h) ? 'bg-blue-600 border-blue-600 text-white' : 'border-slate-600 text-slate-400 hover:text-white'
                  }`}
                >
                  {h < 24 ? `${h}h` : h < 168 ? `${h / 24}d` : '1w'}
                </button>
              ))}
            </div>
          </div>

          <div className="bg-slate-700/30 rounded-lg p-3 text-xs space-y-1">
            <p className="text-slate-400">
              Buy <span className="text-white">${dcaAmt} USDT</span> of {symbol} every{' '}
              <span className="text-white">{parseInt(dcaH) < 24 ? `${dcaH}h` : `${parseInt(dcaH) / 24}d`}</span>
            </p>
            <p className="text-slate-400">
              ≈ <span className="text-white">{currentPrice > 0 ? (parseFloat(dcaAmt || '0') / currentPrice).toFixed(6) : '—'} {symbol}</span> per buy
            </p>
          </div>

          {msg && <p className={`text-xs ${msg.startsWith('Error') ? 'text-red-400' : 'text-green-400'}`}>{msg}</p>}

          <button
            onClick={createDCABot}
            disabled={!activeEx || dcaMut.isPending}
            className="w-full py-2.5 text-sm font-semibold rounded-lg bg-blue-600 hover:bg-blue-500 text-white transition-colors disabled:opacity-50"
          >
            {dcaMut.isPending ? 'Creating…' : 'Create DCA Bot'}
          </button>
        </>
      )}
    </div>
  )
}

// ─── Main Modal ───────────────────────────────────────────────────────────────

export default function CoinModal({ symbol, price, onClose }: CoinModalProps) {
  const [activeTab, setActiveTab] = useState<Tab>('info')
  const tvSymbol = `BINANCE:${symbol}USDT`

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center bg-black/70 p-4"
      onClick={onClose}
    >
      <div
        className="bg-slate-800 border border-slate-700 rounded-2xl w-full max-w-2xl max-h-[90vh] flex flex-col shadow-2xl overflow-hidden"
        onClick={(e) => e.stopPropagation()}
      >
        {/* Header */}
        <div className="flex items-center justify-between px-5 py-4 border-b border-slate-700 flex-shrink-0">
          <div className="flex items-center gap-3">
            <span className="text-xl font-bold text-white">{symbol}</span>
            <span className="text-lg font-semibold text-slate-300">{fmt$(price)}</span>
            <a
              href={`https://www.tradingview.com/chart/?symbol=${tvSymbol}`}
              target="_blank"
              rel="noopener noreferrer"
              className="text-slate-400 hover:text-blue-400 transition-colors"
              title="Open in TradingView"
            >
              <ExternalLink size={14} />
            </a>
          </div>
          <button onClick={onClose} className="text-slate-400 hover:text-white transition-colors">
            <X size={20} />
          </button>
        </div>

        {/* Chart — fixed height, no flex-shrink */}
        <div className="flex-shrink-0 bg-slate-900 px-2 py-2">
          <PriceChart symbol={symbol} height={208} />
        </div>

        {/* Tab bar */}
        <div className="flex border-b border-slate-700 flex-shrink-0">
          {TABS.map((tab) => (
            <button
              key={tab.key}
              onClick={() => setActiveTab(tab.key)}
              className={`flex-1 py-3 text-sm font-medium transition-colors ${
                activeTab === tab.key
                  ? 'text-white border-b-2 border-blue-500'
                  : 'text-slate-400 hover:text-white'
              }`}
            >
              {tab.label}
            </button>
          ))}
        </div>

        {/* Tab content — scrollable */}
        <div className="flex-1 overflow-y-auto">
          {activeTab === 'info'  && <InfoTab  symbol={symbol} currentPrice={price} />}
          {activeTab === 'news'  && <NewsTab  symbol={symbol} />}
          {activeTab === 'trade' && <TradeTab symbol={symbol} currentPrice={price} />}
          {activeTab === 'bot'   && <BotTab   symbol={symbol} currentPrice={price} />}
        </div>
      </div>
    </div>
  )
}
