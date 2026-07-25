import { useState } from 'react'
import { X, Check, TrendingUp, TrendingDown, Activity, Target, ChevronDown, Zap, BarChart2, Layers } from 'lucide-react'
import type { OrderType } from '../types'

interface TypeMeta {
  type: OrderType
  icon: React.ReactNode
  label: string
  desc: string
  group: 'basic' | 'advanced'
}

const ORDER_TYPES: TypeMeta[] = [
  { type: 'limit', icon: <TrendingUp size={18} />, label: 'Limit', desc: 'Buy/sell at specified price or better', group: 'basic' },
  { type: 'market', icon: <Zap size={18} />, label: 'Market', desc: 'Immediate execution at best available price', group: 'basic' },
  { type: 'tpsl', icon: <Target size={18} />, label: 'TP/SL', desc: 'Auto-place order when target price is reached (OCO)', group: 'basic' },
  { type: 'chase', icon: <Activity size={18} />, label: 'Chase', desc: 'Limit order that tracks best bid/ask automatically', group: 'advanced' },
  { type: 'advanced_limit', icon: <TrendingUp size={18} />, label: 'Advanced Limit', desc: 'Limit order with Post-Only / FOK / IOC execution', group: 'advanced' },
  { type: 'trailing_stop', icon: <TrendingDown size={18} />, label: 'Trailing Stop', desc: 'Market order when price pulls back by callback amount', group: 'advanced' },
  { type: 'trigger', icon: <ChevronDown size={18} />, label: 'Trigger', desc: 'Place limit or market order when trigger price hit', group: 'advanced' },
  { type: 'scaled', icon: <Layers size={18} />, label: 'Scaled (TWAP)', desc: 'Place multiple limit sub-orders across a price range', group: 'advanced' },
]

interface Props {
  value: OrderType
  onChange: (t: OrderType) => void
}

export function OrderTypeSelector({ value, onChange }: Props) {
  const [open, setOpen] = useState(false)
  const current = ORDER_TYPES.find((t) => t.type === value)!

  return (
    <>
      <button
        type="button"
        onClick={() => setOpen(true)}
        className="flex items-center gap-2 px-3 py-2 rounded-lg bg-slate-700 border border-slate-600 text-sm text-white hover:bg-slate-600 transition-colors"
      >
        <span className="text-blue-400">{current.icon}</span>
        <span className="font-medium">{current.label}</span>
        <BarChart2 size={14} className="text-slate-400 ml-auto" />
      </button>

      {open && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm p-4">
          <div className="bg-slate-900 border border-slate-700 rounded-2xl shadow-2xl w-full max-w-md">
            <div className="flex items-center justify-between px-5 py-4 border-b border-slate-700">
              <span className="font-semibold text-white">Select Order Type</span>
              <button onClick={() => setOpen(false)} className="text-slate-400 hover:text-white">
                <X size={18} />
              </button>
            </div>

            <div className="p-4 space-y-4 max-h-[70vh] overflow-y-auto">
              {(['basic', 'advanced'] as const).map((group) => (
                <div key={group}>
                  <p className="text-xs font-semibold uppercase text-slate-500 mb-2 px-1">
                    {group === 'basic' ? 'Basic' : 'Advanced'}
                  </p>
                  <div className="space-y-1">
                    {ORDER_TYPES.filter((t) => t.group === group).map((t) => (
                      <button
                        key={t.type}
                        type="button"
                        onClick={() => { onChange(t.type); setOpen(false) }}
                        className={`w-full flex items-center gap-3 px-3 py-3 rounded-xl transition-colors text-left ${
                          t.type === value
                            ? 'bg-blue-600/20 border border-blue-600/40'
                            : 'hover:bg-slate-800 border border-transparent'
                        }`}
                      >
                        <span className={`flex-shrink-0 ${t.type === value ? 'text-blue-400' : 'text-slate-400'}`}>
                          {t.icon}
                        </span>
                        <span className="flex-1 min-w-0">
                          <span className="block text-sm font-medium text-white">{t.label}</span>
                          <span className="block text-xs text-slate-400 mt-0.5 leading-snug">{t.desc}</span>
                        </span>
                        {t.type === value && <Check size={16} className="text-blue-400 flex-shrink-0" />}
                      </button>
                    ))}
                  </div>
                </div>
              ))}
            </div>
          </div>
        </div>
      )}
    </>
  )
}
