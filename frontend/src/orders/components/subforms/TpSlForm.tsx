import { useFormContext } from 'react-hook-form'
import { FieldWrap, Field, SelectField } from '../fields'
import type { TpSlInput } from '../../schemas'

const TRIGGER_TYPES = [
  { value: 'last', label: 'Last' },
  { value: 'mark', label: 'Mark' },
  { value: 'index', label: 'Index' },
]

export function TpSlForm() {
  const { register } = useFormContext<TpSlInput>()

  return (
    <>
      <div className="col-span-full text-xs text-blue-400 bg-blue-900/20 border border-blue-700/30 rounded-lg px-3 py-2">
        OCO: if TP triggers, SL is auto-cancelled and vice versa.
      </div>

      {/* TP */}
      <div className="col-span-full grid grid-cols-3 gap-3 p-3 rounded-lg border border-green-800/40 bg-green-900/10">
        <p className="col-span-3 text-xs font-medium text-green-400">Take Profit</p>
        <FieldWrap label="TP Trigger Price">
          <Field type="number" step="any" placeholder="70000" {...register('tp.tpTriggerPx', { valueAsNumber: true })} />
        </FieldWrap>
        <FieldWrap label="TP Order Price (-1=market)">
          <Field type="number" step="any" placeholder="-1" {...register('tp.tpOrdPx', { valueAsNumber: true })} />
        </FieldWrap>
        <FieldWrap label="Trigger Type">
          <SelectField {...register('tp.tpTriggerPxType')}>
            {TRIGGER_TYPES.map(t => <option key={t.value} value={t.value}>{t.label}</option>)}
          </SelectField>
        </FieldWrap>
      </div>

      {/* SL */}
      <div className="col-span-full grid grid-cols-3 gap-3 p-3 rounded-lg border border-red-800/40 bg-red-900/10">
        <p className="col-span-3 text-xs font-medium text-red-400">Stop Loss</p>
        <FieldWrap label="SL Trigger Price">
          <Field type="number" step="any" placeholder="60000" {...register('sl.slTriggerPx', { valueAsNumber: true })} />
        </FieldWrap>
        <FieldWrap label="SL Order Price (-1=market)">
          <Field type="number" step="any" placeholder="-1" {...register('sl.slOrdPx', { valueAsNumber: true })} />
        </FieldWrap>
        <FieldWrap label="Trigger Type">
          <SelectField {...register('sl.slTriggerPxType')}>
            {TRIGGER_TYPES.map(t => <option key={t.value} value={t.value}>{t.label}</option>)}
          </SelectField>
        </FieldWrap>
      </div>
    </>
  )
}
