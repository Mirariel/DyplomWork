import { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import {
  AreaChart,
  Area,
  XAxis,
  YAxis,
  Tooltip,
  ResponsiveContainer,
  CartesianGrid,
} from 'recharts'

// ─── Types ────────────────────────────────────────────────────────────────────

interface ChartPoint {
  time: string
  price: number
  open: number
  high: number
  low: number
}

type Interval = '15m' | '1h' | '4h' | '1d'

const INTERVALS: { key: Interval; label: string; limit: number }[] = [
  { key: '15m', label: '15m', limit: 48 },
  { key: '1h',  label: '1h',  limit: 48 },
  { key: '4h',  label: '4h',  limit: 42 },
  { key: '1d',  label: '1d',  limit: 30 },
]

// ─── Fetch helper ─────────────────────────────────────────────────────────────

async function fetchKlines(symbol: string, interval: Interval, limit: number): Promise<ChartPoint[]> {
  const res = await fetch(
    `https://api.binance.com/api/v3/klines?symbol=${symbol}USDT&interval=${interval}&limit=${limit}`,
  )
  if (!res.ok) throw new Error('Failed to fetch klines')

  // Binance kline: [openTime, open, high, low, close, volume, ...]
  const data: [number, string, string, string, string, ...unknown[]][] = await res.json()

  return data.map((k) => {
    const ts = new Date(k[0])
    const timeLabel =
      interval === '1d'
        ? ts.toLocaleDateString('en-US', { month: 'short', day: 'numeric' })
        : ts.toLocaleTimeString('en-US', { hour: '2-digit', minute: '2-digit', hour12: false })
    return {
      time:  timeLabel,
      price: parseFloat(k[4]),  // close
      open:  parseFloat(k[1]),
      high:  parseFloat(k[2]),
      low:   parseFloat(k[3]),
    }
  })
}

// ─── Custom Tooltip ───────────────────────────────────────────────────────────

function CustomTooltip({ active, payload, label }: {
  active?: boolean
  payload?: Array<{ value: number }>
  label?: string
}) {
  if (!active || !payload?.length) return null
  const price = payload[0].value
  return (
    <div className="bg-slate-900 border border-slate-700 rounded-lg px-3 py-2 text-xs shadow-lg">
      <p className="text-slate-400 mb-0.5">{label}</p>
      <p className="text-white font-semibold">
        ${price.toLocaleString('en-US', { minimumFractionDigits: 2, maximumFractionDigits: price < 1 ? 6 : 2 })}
      </p>
    </div>
  )
}

// ─── PriceChart ───────────────────────────────────────────────────────────────

interface PriceChartProps {
  symbol: string           // base symbol, e.g. "BTC"
  height?: number          // chart height px (default 200)
  showIntervalSelector?: boolean
  defaultInterval?: Interval
}

export default function PriceChart({
  symbol,
  height = 200,
  showIntervalSelector = true,
  defaultInterval = '1h',
}: PriceChartProps) {
  const [interval, setInterval] = useState<Interval>(defaultInterval)
  const cfg = INTERVALS.find((i) => i.key === interval) ?? INTERVALS[1]

  const { data = [], isLoading, isError } = useQuery({
    queryKey: ['klines', symbol, interval],
    queryFn:  () => fetchKlines(symbol, interval, cfg.limit),
    staleTime: 60_000,   // 1 minute
    retry: 1,
  })

  // Color: green if last price >= first price
  const positive = data.length >= 2 ? data[data.length - 1].price >= data[0].price : true
  const lineColor = positive ? '#34d399' : '#f87171'
  const gradId    = `grad-${symbol}-${interval}`

  // Y-axis domain with a little padding
  const prices   = data.map((d) => d.price)
  const minPrice = prices.length ? Math.min(...prices) : 0
  const maxPrice = prices.length ? Math.max(...prices) : 1
  const pad      = (maxPrice - minPrice) * 0.05 || maxPrice * 0.01

  const fmtY = (v: number) =>
    v >= 1000
      ? `$${(v / 1000).toFixed(1)}k`
      : `$${v.toFixed(v < 1 ? 4 : 2)}`

  return (
    <div className="w-full flex flex-col gap-2">
      {/* Interval selector */}
      {showIntervalSelector && (
        <div className="flex gap-1 justify-end px-1">
          {INTERVALS.map((opt) => (
            <button
              key={opt.key}
              onClick={() => setInterval(opt.key)}
              className={`px-2.5 py-1 text-xs rounded transition-colors ${
                interval === opt.key
                  ? 'bg-blue-600 text-white'
                  : 'text-slate-400 hover:text-white hover:bg-slate-700'
              }`}
            >
              {opt.label}
            </button>
          ))}
        </div>
      )}

      {/* Chart area */}
      {isLoading ? (
        <div
          className="flex items-center justify-center text-slate-500 text-xs"
          style={{ height }}
        >
          Loading chart…
        </div>
      ) : isError || data.length === 0 ? (
        <div
          className="flex items-center justify-center text-slate-500 text-xs"
          style={{ height }}
        >
          Chart unavailable
        </div>
      ) : (
        <ResponsiveContainer width="100%" height={height}>
          <AreaChart data={data} margin={{ top: 4, right: 8, bottom: 0, left: 0 }}>
            <defs>
              <linearGradient id={gradId} x1="0" y1="0" x2="0" y2="1">
                <stop offset="5%"  stopColor={lineColor} stopOpacity={0.25} />
                <stop offset="95%" stopColor={lineColor} stopOpacity={0} />
              </linearGradient>
            </defs>
            <CartesianGrid strokeDasharray="3 3" stroke="#1e293b" vertical={false} />
            <XAxis
              dataKey="time"
              tick={{ fill: '#64748b', fontSize: 10 }}
              tickLine={false}
              axisLine={false}
              interval="preserveStartEnd"
            />
            <YAxis
              domain={[minPrice - pad, maxPrice + pad]}
              tick={{ fill: '#64748b', fontSize: 10 }}
              tickLine={false}
              axisLine={false}
              tickFormatter={fmtY}
              width={60}
            />
            <Tooltip content={<CustomTooltip />} />
            <Area
              type="monotone"
              dataKey="price"
              stroke={lineColor}
              strokeWidth={1.5}
              fill={`url(#${gradId})`}
              dot={false}
              activeDot={{ r: 3, fill: lineColor }}
            />
          </AreaChart>
        </ResponsiveContainer>
      )}
    </div>
  )
}
