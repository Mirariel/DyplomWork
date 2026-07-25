import { useFormContext } from 'react-hook-form'
import { FieldWrap, Field, SelectField, Toggle } from '../fields'
import { calcScaledSubOrders } from '../../types'
import type { ScaledInput } from '../../schemas'
import type { ScaledOrder } from '../../types'

export function ScaledForm() {
  const { register, watch, setValue, formState: { errors } } = useFormContext<ScaledInput>()
  const values = watch()

  const preview = values.priceLow > 0 && values.priceHigh > values.priceLow && values.subOrderCount >= 2 && values.totalSz > 0
    ? calcScaledSubOrders(values as ScaledOrder)
    : []

  return (
    <>
      <FieldWrap label="Price Low" error={errors.priceLow}>
        <Field type="number" step="any" placeholder="60000" {...register('priceLow', { valueAsNumber: true })} />
      </FieldWrap>

      <FieldWrap label="Price High" error={errors.priceHigh}>
        <Field type="number" step="any" placeholder="70000" {...register('priceHigh', { valueAsNumber: true })} />
      </FieldWrap>

      <FieldWrap label="Sub-order Count (2–100)" error={errors.subOrderCount}>
        <Field type="number" min="2" max="100" step="1" placeholder="5" {...register('subOrderCount', { valueAsNumber: true })} />
      </FieldWrap>

      <FieldWrap label="Total Size" error={errors.totalSz}>
        <Field type="number" step="any" placeholder="0.05" {...register('totalSz', { valueAsNumber: true })} />
      </FieldWrap>

      <FieldWrap label="Distribution" tooltip="How to distribute size across sub-orders">
        <SelectField {...register('distribution')}>
          <option value="flat">Flat (equal size)</option>
          <option value="ascending">Ascending (more at top)</option>
          <option value="descending">Descending (more at bottom)</option>
        </SelectField>
      </FieldWrap>

      <div className="col-span-full">
        <Toggle checked={!!watch('reduceOnly')} onChange={(v) => setValue('reduceOnly', v)} label="Reduce Only" />
      </div>

      {/* Preview table */}
      {preview.length > 0 && (
        <div className="col-span-full">
          <p className="text-xs font-medium text-slate-400 mb-2">Sub-order Preview</p>
          <div className="rounded-lg border border-slate-600 overflow-hidden max-h-48 overflow-y-auto">
            <table className="w-full text-xs">
              <thead className="bg-slate-700">
                <tr>
                  <th className="px-3 py-2 text-left text-slate-400">#</th>
                  <th className="px-3 py-2 text-left text-slate-400">Price</th>
                  <th className="px-3 py-2 text-left text-slate-400">Size</th>
                </tr>
              </thead>
              <tbody>
                {preview.map((row, i) => (
                  <tr key={i} className="border-t border-slate-700 hover:bg-slate-700/30">
                    <td className="px-3 py-1.5 text-slate-500">{i + 1}</td>
                    <td className="px-3 py-1.5 text-white">${row.price.toFixed(2)}</td>
                    <td className="px-3 py-1.5 text-slate-300">{row.size.toFixed(6)}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      )}
    </>
  )
}
