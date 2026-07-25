import { useFormContext } from 'react-hook-form'
import { FieldWrap, Field, SelectField, Toggle } from '../fields'
import type { TriggerInput } from '../../schemas'

export function TriggerForm() {
  const { register, watch, setValue, formState: { errors } } = useFormContext<TriggerInput>()
  const orderPx = watch('orderPx')
  const isMarket = orderPx === -1

  return (
    <>
      <FieldWrap label="Trigger Price" error={errors.triggerPx}>
        <Field type="number" step="any" placeholder="65000" {...register('triggerPx', { valueAsNumber: true })} />
      </FieldWrap>

      <FieldWrap label="Trigger Type">
        <SelectField {...register('triggerPxType')}>
          <option value="last">Last</option>
          <option value="mark">Mark</option>
          <option value="index">Index</option>
        </SelectField>
      </FieldWrap>

      <FieldWrap label="Order Price (-1 = market)" error={errors.orderPx}>
        <div className="flex gap-2">
          <Field
            type="number"
            step="any"
            placeholder="-1 for market"
            disabled={isMarket}
            {...register('orderPx', { valueAsNumber: true })}
            className={isMarket ? 'opacity-50' : ''}
          />
          <button
            type="button"
            onClick={() => setValue('orderPx', isMarket ? 65000 : -1)}
            className="flex-shrink-0 px-2 py-1 text-xs rounded bg-slate-600 text-slate-300 hover:bg-slate-500"
          >
            {isMarket ? 'Set price' : 'Market'}
          </button>
        </div>
        {isMarket && <p className="text-xs text-slate-400 mt-1">Will execute at market price when triggered</p>}
      </FieldWrap>

      <div className="col-span-full">
        <Toggle checked={!!watch('reduceOnly')} onChange={(v) => setValue('reduceOnly', v)} label="Reduce Only" />
      </div>
    </>
  )
}
