import { useFormContext } from 'react-hook-form'
import { FieldWrap, Field, SelectField, Toggle } from '../fields'
import type { ChaseInput } from '../../schemas'

export function ChaseForm() {
  const { register, watch, setValue, formState: { errors } } = useFormContext<ChaseInput>()
  const chaseType = watch('chaseType')

  return (
    <>
      <FieldWrap label="Chase Type" tooltip="'Current' tracks best price; 'Distance' keeps a fixed offset">
        <SelectField {...register('chaseType')}>
          <option value="current">Current (track best price)</option>
          <option value="distance">Distance (fixed offset)</option>
        </SelectField>
      </FieldWrap>

      {chaseType === 'distance' && (
        <FieldWrap label="Chase Value" error={errors.chaseVal}>
          <Field type="number" step="any" placeholder="50" {...register('chaseVal', { valueAsNumber: true })} />
        </FieldWrap>
      )}

      <FieldWrap label="Max Chase Type" tooltip="Maximum allowed deviation from initial price">
        <SelectField {...register('maxChaseType')}>
          <option value="distance">Distance (absolute)</option>
          <option value="ratio">Ratio (%)</option>
        </SelectField>
      </FieldWrap>

      <FieldWrap label="Max Chase Value" error={errors.maxChaseVal}>
        <Field type="number" step="any" placeholder="200" {...register('maxChaseVal', { valueAsNumber: true })} />
      </FieldWrap>

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
