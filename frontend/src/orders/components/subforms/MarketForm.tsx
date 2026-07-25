import { useFormContext } from 'react-hook-form'
import { Toggle } from '../fields'
import type { MarketInput } from '../../schemas'

export function MarketForm({ bestPrice }: { bestPrice?: number }) {
  const { watch, setValue } = useFormContext<MarketInput>()

  return (
    <>
      {bestPrice !== undefined && (
        <div className="col-span-full text-xs text-slate-400 bg-slate-700/50 rounded-lg px-3 py-2">
          Est. fill price: <span className="text-white font-medium">${bestPrice.toLocaleString()}</span>
          <span className="ml-2 text-yellow-400">⚠ Slippage possible</span>
        </div>
      )}
      <div className="col-span-full">
        <Toggle
          checked={!!watch('reduceOnly')}
          onChange={(v) => setValue('reduceOnly', v)}
          label="Reduce Only"
        />
      </div>
    </>
  )
}
