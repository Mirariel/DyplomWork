import { useFormContext } from 'react-hook-form'
import { FieldWrap, Field, Toggle } from '../fields'
import type { LimitInput } from '../../schemas'

// eslint-disable-next-line @typescript-eslint/no-explicit-any
type AnyRegister = (name: any, opts?: any) => any

export function LimitForm() {
  const { register: _register, watch, setValue, formState: { errors } } = useFormContext<LimitInput>()
  const register = _register as AnyRegister
  const attachTpSl = watch('attachTpSl')

  return (
    <>
      <FieldWrap label="Price" error={errors.px}>
        <Field type="number" step="any" placeholder="65000" {...register('px', { valueAsNumber: true })} />
      </FieldWrap>

      <div className="col-span-full flex gap-4 flex-wrap">
        <Toggle
          checked={!!watch('reduceOnly')}
          onChange={(v) => setValue('reduceOnly', v)}
          label="Reduce Only"
          tooltip="Only reduces existing position"
        />
        <Toggle
          checked={!!attachTpSl}
          onChange={(v) => setValue('attachTpSl', v)}
          label="Attach TP/SL"
          tooltip="Attach take-profit and stop-loss to this order"
        />
      </div>

      {attachTpSl && (
        <div className="col-span-full grid grid-cols-2 gap-3 p-3 rounded-lg border border-slate-600 bg-slate-800/50">
          <p className="col-span-2 text-xs text-slate-400 font-medium">Attached TP/SL</p>
          <FieldWrap label="TP Trigger Price">
            <Field type="number" step="any" placeholder="70000" {...register('attachedTp.tpTriggerPx', { valueAsNumber: true })} />
          </FieldWrap>
          <FieldWrap label="TP Order Price (-1 = market)">
            <Field type="number" step="any" placeholder="-1" defaultValue="-1" {...register('attachedTp.tpOrdPx', { valueAsNumber: true })} />
          </FieldWrap>
          <FieldWrap label="SL Trigger Price">
            <Field type="number" step="any" placeholder="60000" {...register('attachedSl.slTriggerPx', { valueAsNumber: true })} />
          </FieldWrap>
          <FieldWrap label="SL Order Price (-1 = market)">
            <Field type="number" step="any" placeholder="-1" defaultValue="-1" {...register('attachedSl.slOrdPx', { valueAsNumber: true })} />
          </FieldWrap>
        </div>
      )}
    </>
  )
}
