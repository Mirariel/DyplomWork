import { useFormContext } from 'react-hook-form'
import { FieldWrap, Field, RadioGroup, Toggle } from '../fields'
import type { AdvancedLimitInput } from '../../schemas'

const EXEC_MODES = [
  { value: 'post_only', label: 'Post Only', tooltip: 'Guaranteed maker; cancels if matched immediately' },
  { value: 'fok', label: 'FOK', tooltip: 'Fill entire order immediately or cancel completely' },
  { value: 'ioc', label: 'IOC', tooltip: "Fill what's possible immediately, cancel remainder" },
]

export function AdvancedLimitForm() {
  const { register, watch, setValue, formState: { errors } } = useFormContext<AdvancedLimitInput>()

  return (
    <>
      <div className="col-span-full">
        <FieldWrap label="Execution Mode">
          <RadioGroup
            value={watch('execMode') ?? 'post_only'}
            onChange={(v) => setValue('execMode', v as AdvancedLimitInput['execMode'])}
            options={EXEC_MODES}
          />
        </FieldWrap>
      </div>

      <FieldWrap label="Price" error={errors.px}>
        <Field type="number" step="any" placeholder="65000" {...register('px', { valueAsNumber: true })} />
      </FieldWrap>

      <FieldWrap label="Size" error={errors.sz}>
        <Field type="number" step="any" placeholder="0.01" {...register('sz', { valueAsNumber: true })} />
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
