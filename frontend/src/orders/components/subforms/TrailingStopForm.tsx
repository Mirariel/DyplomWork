import { useFormContext } from 'react-hook-form'
import { FieldWrap, Field, RadioGroup, Toggle } from '../fields'
import type { TrailingStopInput } from '../../schemas'

export function TrailingStopForm() {
  const { register, watch, setValue, formState: { errors } } = useFormContext<TrailingStopInput>()
  const callbackMode = watch('callbackMode')

  return (
    <>
      <FieldWrap label="Activation Price" tooltip="Order becomes active when this price is reached. Leave empty to activate immediately.">
        <Field type="number" step="any" placeholder="Optional" {...register('activePx', { valueAsNumber: true })} />
      </FieldWrap>

      <div className="col-span-full">
        <FieldWrap label="Callback Mode">
          <RadioGroup
            value={callbackMode ?? 'ratio'}
            onChange={(v) => setValue('callbackMode', v as 'ratio' | 'spread')}
            options={[
              { value: 'ratio', label: '% Ratio' },
              { value: 'spread', label: 'Absolute' },
            ]}
          />
        </FieldWrap>
      </div>

      {callbackMode === 'ratio' ? (
        <FieldWrap label="Callback Ratio (%)" error={errors.callbackRatio}>
          <Field type="number" step="0.01" min="0.01" max="50" placeholder="1.5" {...register('callbackRatio', { valueAsNumber: true })} />
        </FieldWrap>
      ) : (
        <FieldWrap label="Callback Spread" error={errors.callbackSpread}>
          <Field type="number" step="any" placeholder="500" {...register('callbackSpread', { valueAsNumber: true })} />
        </FieldWrap>
      )}

      <div className="col-span-full">
        <Toggle checked={!!watch('reduceOnly')} onChange={(v) => setValue('reduceOnly', v)} label="Reduce Only" />
      </div>
    </>
  )
}
