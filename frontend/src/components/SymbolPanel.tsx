import { useState, useEffect, useRef } from 'react'
import { useQuery } from '@tanstack/react-query'
import { createChart, ColorType, CrosshairMode } from 'lightweight-charts'
import type { IChartApi, ISeriesApi, UTCTimestamp } from 'lightweight-charts'
import { TrendingUp, TrendingDown, Star, Search } from 'lucide-react'

type Interval = '1m' | '5m' | '15m' | '1h' | '4h' | '1d'

const INTERVALS: { key: Interval; label: string; limit: number }[] = [
  { key: '1m',  label: '1m',  limit: 120 },
  { key: '5m',  label: '5m',  limit: 100 },
  { key: '15m', label: '15m', limit: 96 },
  { key: '1h',  label: '1h',  limit: 72 },
  { key: '4h',  label: '4h',  limit: 60 },
  { key: '1d',  label: '1d',  limit: 60 },
]

// Top coins for search dropdown
const ALL_SYMBOLS = [
  'BTC', 'ETH', 'SOL', 'BNB', 'XRP', 'DOGE', 'ADA', 'AVAX', 'DOT', 'MATIC',
  'LINK', 'UNI', 'ATOM', 'LTC', 'FIL', 'APT', 'ARB', 'OP', 'SUI', 'NEAR',
  'INJ', 'TIA', 'SEI', 'JUP', 'WIF', 'PEPE', 'BONK', 'SHIB', 'FTM', 'ALGO',
  'AAVE', 'MKR', 'CRV', 'LDO', 'RUNE', 'SAND', 'MANA', 'AXS', 'IMX', 'GALA',
  'RENDER', 'FET', 'AGIX', 'TAO', 'WLD', 'ONDO', 'ENA', 'STX', 'TRX', 'TON',
]

const FAVORITES_KEY = 'tradetracker_fav_symbols'

function loadFavorites(): string[] {
  try {
    const raw = localStorage.getItem(FAVORITES_KEY)
    return raw ? JSON.parse(raw) : ['BTC', 'ETH', 'SOL']
  } catch { return ['BTC', 'ETH', 'SOL'] }
}

function saveFavorites(favs: string[]) {
  localStorage.setItem(FAVORITES_KEY, JSON.stringify(favs))
}

interface Kline {
  time: number
  open: number
  high: number
  low: number
  close: number
  volume: number
}

async function fetchKlines(symbol: string, interval: Interval, limit: number): Promise<Kline[]> {
  const res = await fetch(
    `https://api.binance.com/api/v3/klines?symbol=${symbol}USDT&interval=${interval}&limit=${limit}`,
  )
  if (!res.ok) throw new Error('Failed to fetch klines')
  const data: [number, string, string, string, string, string][] = await res.json()
  return data.map((k) => ({
    time: Math.floor(k[0] / 1000),
    open: parseFloat(k[1]),
    high: parseFloat(k[2]),
    low: parseFloat(k[3]),
    close: parseFloat(k[4]),
    volume: parseFloat(k[5]),
  }))
}

interface Props {
  symbol: string
  onSymbolChange: (s: string) => void
}

export function SymbolPanel({ symbol, onSymbolChange }: Props) {
  const [interval, setInterval] = useState<Interval>('1h')
  const [inputVal, setInputVal] = useState(symbol)
  const [showDropdown, setShowDropdown] = useState(false)
  const [favorites, setFavorites] = useState(loadFavorites)
  const dropdownRef = useRef<HTMLDivElement>(null)
  const chartRef = useRef<HTMLDivElement>(null)
  const chartApi = useRef<IChartApi | null>(null)
  const candleSeries = useRef<ISeriesApi<'Candlestick'> | null>(null)
  const volSeries = useRef<ISeriesApi<'Histogram'> | null>(null)

  const cfg = INTERVALS.find((i) => i.key === interval) ?? INTERVALS[3]

  const { data = [], isLoading, isError } = useQuery({
    queryKey: ['klines-candle', symbol, interval],
    queryFn: () => fetchKlines(symbol, interval, cfg.limit),
    staleTime: 30_000,
    retry: 1,
  })

  // Close dropdown on outside click
  useEffect(() => {
    const handler = (e: MouseEvent) => {
      if (dropdownRef.current && !dropdownRef.current.contains(e.target as Node)) {
        setShowDropdown(false)
      }
    }
    document.addEventListener('mousedown', handler)
    return () => document.removeEventListener('mousedown', handler)
  }, [])

  // Create chart once
  useEffect(() => {
    if (!chartRef.current) return
    const chart = createChart(chartRef.current, {
      layout: {
        background: { type: ColorType.Solid, color: '#0f172a' },
        textColor: '#94a3b8',
      },
      grid: {
        vertLines: { color: '#1e293b' },
        horzLines: { color: '#1e293b' },
      },
      crosshair: { mode: CrosshairMode.Normal },
      rightPriceScale: { borderColor: '#334155' },
      timeScale: { borderColor: '#334155', timeVisible: true, secondsVisible: false },
      width: chartRef.current.clientWidth,
      height: 340,
    })

    candleSeries.current = chart.addCandlestickSeries({
      upColor: '#34d399',
      downColor: '#f87171',
      borderUpColor: '#34d399',
      borderDownColor: '#f87171',
      wickUpColor: '#34d399',
      wickDownColor: '#f87171',
    })

    volSeries.current = chart.addHistogramSeries({
      color: '#3b82f680',
      priceFormat: { type: 'volume' },
      priceScaleId: 'vol',
    })
    chart.priceScale('vol').applyOptions({ scaleMargins: { top: 0.85, bottom: 0 } })

    chartApi.current = chart

    const ro = new ResizeObserver(() => {
      if (chartRef.current) chart.applyOptions({ width: chartRef.current.clientWidth })
    })
    ro.observe(chartRef.current)

    return () => {
      ro.disconnect()
      chart.remove()
      chartApi.current = null
    }
  }, [])

  // Update data
  useEffect(() => {
    if (!candleSeries.current || !volSeries.current || data.length === 0) return
    const candles = data.map((k) => ({
      time: k.time as UTCTimestamp,
      open: k.open, high: k.high, low: k.low, close: k.close,
    }))
    const vols = data.map((k) => ({
      time: k.time as UTCTimestamp,
      value: k.volume,
      color: k.close >= k.open ? '#34d39940' : '#f8717140',
    }))
    candleSeries.current.setData(candles)
    volSeries.current.setData(vols)
    chartApi.current?.timeScale().fitContent()
  }, [data])

  const lastCandle = data.at(-1)
  const prevCandle = data.at(-2)
  const priceChange = lastCandle && prevCandle
    ? ((lastCandle.close - prevCandle.close) / prevCandle.close) * 100
    : 0
  const isUp = priceChange >= 0

  const handleSymbolSelect = (s: string) => {
    const sym = s.trim().toUpperCase().replace(/USDT$/, '')
    if (sym) {
      setInputVal(sym)
      onSymbolChange(sym)
      setShowDropdown(false)
    }
  }

  const toggleFavorite = (s: string) => {
    const updated = favorites.includes(s)
      ? favorites.filter((f) => f !== s)
      : [...favorites, s]
    setFavorites(updated)
    saveFavorites(updated)
  }

  // Filter dropdown results
  const filtered = inputVal
    ? ALL_SYMBOLS.filter((s) => s.includes(inputVal.toUpperCase()))
    : ALL_SYMBOLS

  return (
    <div className="bg-slate-800 rounded-xl border border-slate-700 overflow-hidden">
      {/* Header */}
      <div className="px-4 py-3 border-b border-slate-700 flex items-center gap-3 flex-wrap">
        {/* Symbol input with dropdown */}
        <div className="relative" ref={dropdownRef}>
          <div className="flex items-center gap-1">
            <div className="relative">
              <Search size={14} className="absolute left-2.5 top-1/2 -translate-y-1/2 text-slate-500" />
              <input
                value={inputVal}
                onChange={(e) => { setInputVal(e.target.value.toUpperCase()); setShowDropdown(true) }}
                onFocus={() => setShowDropdown(true)}
                onKeyDown={(e) => { if (e.key === 'Enter') { e.preventDefault(); handleSymbolSelect(inputVal) } }}
                className="bg-slate-700 border border-slate-600 rounded-lg pl-8 pr-3 py-1.5 text-sm text-white font-mono w-32 focus:outline-none focus:ring-1 focus:ring-blue-500 uppercase"
                placeholder="Search..."
              />
            </div>
            {/* Star to add/remove current from favorites */}
            <button
              type="button"
              onClick={() => toggleFavorite(symbol)}
              className={`p-1.5 rounded transition-colors ${
                favorites.includes(symbol) ? 'text-yellow-400 hover:text-yellow-300' : 'text-slate-500 hover:text-slate-300'
              }`}
              title={favorites.includes(symbol) ? 'Remove from favorites' : 'Add to favorites'}
            >
              <Star size={16} fill={favorites.includes(symbol) ? 'currentColor' : 'none'} />
            </button>
          </div>

          {/* Dropdown */}
          {showDropdown && (
            <div className="absolute top-full left-0 mt-1 w-56 bg-slate-900 border border-slate-700 rounded-lg shadow-xl z-50 max-h-64 overflow-y-auto">
              {filtered.length === 0 ? (
                <p className="px-3 py-2 text-xs text-slate-500">No results</p>
              ) : (
                filtered.map((s) => (
                  <button
                    key={s}
                    type="button"
                    onClick={() => handleSymbolSelect(s)}
                    className={`w-full flex items-center justify-between px-3 py-2 text-sm hover:bg-slate-700 transition-colors ${
                      s === symbol ? 'bg-slate-800 text-blue-400' : 'text-slate-300'
                    }`}
                  >
                    <span className="font-mono">{s}<span className="text-slate-500">/USDT</span></span>
                    <button
                      type="button"
                      onClick={(e) => { e.stopPropagation(); toggleFavorite(s) }}
                      className={`p-0.5 ${favorites.includes(s) ? 'text-yellow-400' : 'text-slate-600 hover:text-slate-400'}`}
                    >
                      <Star size={12} fill={favorites.includes(s) ? 'currentColor' : 'none'} />
                    </button>
                  </button>
                ))
              )}
            </div>
          )}
        </div>

        {/* Favorites */}
        <div className="flex gap-1 flex-wrap">
          {favorites.map((s) => (
            <button
              key={s}
              onClick={() => handleSymbolSelect(s)}
              className={`px-2 py-1 rounded text-xs transition-colors ${
                symbol === s ? 'bg-blue-600 text-white' : 'bg-slate-700 text-slate-400 hover:bg-slate-600'
              }`}
            >
              {s}
            </button>
          ))}
        </div>

        {/* Price */}
        {lastCandle && (
          <div className="ml-auto flex items-center gap-2">
            <span className="text-white font-semibold font-mono">
              ${lastCandle.close.toLocaleString('en-US', { maximumFractionDigits: lastCandle.close < 1 ? 6 : 2 })}
            </span>
            <span className={`flex items-center gap-0.5 text-xs font-medium ${isUp ? 'text-green-400' : 'text-red-400'}`}>
              {isUp ? <TrendingUp size={12} /> : <TrendingDown size={12} />}
              {priceChange >= 0 ? '+' : ''}{priceChange.toFixed(2)}%
            </span>
          </div>
        )}

        {/* Intervals */}
        <div className="flex gap-0.5">
          {INTERVALS.map((opt) => (
            <button
              key={opt.key}
              onClick={() => setInterval(opt.key)}
              className={`px-2 py-1 text-xs rounded transition-colors ${
                interval === opt.key
                  ? 'bg-blue-600 text-white'
                  : 'text-slate-400 hover:text-white hover:bg-slate-700'
              }`}
            >
              {opt.label}
            </button>
          ))}
        </div>
      </div>

      {/* Chart */}
      <div className="relative">
        {isLoading && (
          <div className="absolute inset-0 flex items-center justify-center bg-slate-900/60 z-10 text-slate-400 text-sm">
            Loading…
          </div>
        )}
        {isError && (
          <div className="absolute inset-0 flex items-center justify-center bg-slate-900/60 z-10 text-slate-400 text-sm">
            Chart unavailable
          </div>
        )}
        <div ref={chartRef} className="w-full" />
      </div>
    </div>
  )
}
