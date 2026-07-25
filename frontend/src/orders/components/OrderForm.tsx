import { useCallback, useState } from 'react'
import { useForm, FormProvider } from 'react-hook-form'
import { zodResolver } from '@hookform/resolvers/zod'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { getCredentials, listCredentialGroups, placeOrder, createSmartOrder } from '../../api'
import type { Credential } from '../../api'
import { OrderTypeSelector } from './OrderTypeSelector'
import { FieldWrap, Field, SelectField, RadioGroup } from './fields'
import { LimitForm } from './subforms/LimitForm'
import { MarketForm } from './subforms/MarketForm'
import { TpSlForm } from './subforms/TpSlForm'
import { ChaseForm } from './subforms/ChaseForm'
import { AdvancedLimitForm } from './subforms/AdvancedLimitForm'
import { TrailingStopForm } from './subforms/TrailingStopForm'
import { TriggerForm } from './subforms/TriggerForm'
import { ScaledForm } from './subforms/ScaledForm'
import {
  limitSchema, marketSchema, tpslSchema, chaseSchema,
  advancedLimitSchema, trailingStopSchema, triggerSchema, scaledSchema,
} from '../schemas'
import type { OrderType } from '../types'
import { z } from 'zod'

function credLabel(c: Credential) {
  return c.label || `${c.exchange.toUpperCase()} (${c.api_key_hint})`
}

const SCHEMA_MAP: Record<OrderType, z.ZodTypeAny> = {
  limit: limitSchema,
  market: marketSchema,
  tpsl: tpslSchema,
  chase: chaseSchema,
  advanced_limit: advancedLimitSchema,
  trailing_stop: trailingStopSchema,
  trigger: triggerSchema,
  scaled: scaledSchema,
}

// Order types that go to smart_orders backend
const SMART_TYPES: OrderType[] = ['tpsl', 'trailing_stop', 'trigger']

interface Props {
  symbol: string
  onSymbolChange?: (s: string) => void
}

export function OrderForm({ symbol, onSymbolChange: _onSymbolChange }: Props) {
  const qc = useQueryClient()
  const [orderType, setOrderType] = useState<OrderType>('limit')
  const [selectedTarget, setSelectedTarget] = useState('')
  const [sizeMode, setSizeMode] = useState<'abs' | 'pct'>('abs')
  const [feedback, setFeedback] = useState<{ type: 'ok' | 'err'; msg: string } | null>(null)

  const { data: credentials = [] } = useQuery({ queryKey: ['credentials'], queryFn: getCredentials, staleTime: 30_000 })
  const { data: groups = [] } = useQuery({ queryKey: ['credential-groups'], queryFn: listCredentialGroups, staleTime: 30_000 })

  const activeCreds = credentials.filter((c) => c.is_active)

  const schema = SCHEMA_MAP[orderType]
  const methods = useForm({ resolver: zodResolver(schema), mode: 'onChange' })
  const { handleSubmit, register, watch, setValue, formState: { errors, isValid } } = methods

  const category = watch('category') as 'spot' | 'futures' | undefined

  const orderMut = useMutation({
    mutationFn: placeOrder,
    onSuccess: () => { void qc.invalidateQueries({ queryKey: ['orders'] }); setFeedback({ type: 'ok', msg: 'Order placed!' }) },
    onError: (e: { response?: { data?: { error?: string } } }) => setFeedback({ type: 'err', msg: e?.response?.data?.error ?? 'Failed' }),
  })

  const smartMut = useMutation({
    mutationFn: createSmartOrder,
    onSuccess: () => { void qc.invalidateQueries({ queryKey: ['smart-orders'] }); setFeedback({ type: 'ok', msg: 'Smart order created!' }) },
    onError: (e: { response?: { data?: { error?: string } } }) => setFeedback({ type: 'err', msg: e?.response?.data?.error ?? 'Failed' }),
  })

  // Resolve credential IDs from selected target
  const resolveCredIDs = useCallback((): number[] | null => {
    if (selectedTarget.startsWith('cred:')) return [parseInt(selectedTarget.slice(5))]
    if (selectedTarget.startsWith('group:')) {
      const gid = parseInt(selectedTarget.slice(6))
      const g = groups.find((x) => x.id === gid)
      return g ? g.member_ids : null
    }
    return null
  }, [selectedTarget, groups])

  const onSubmit = handleSubmit((data) => {
    const credIDs = resolveCredIDs()
    if (!credIDs || credIDs.length === 0) {
      setFeedback({ type: 'err', msg: 'Select an API key or group.' })
      return
    }

    for (const credID of credIDs) {
      if (SMART_TYPES.includes(orderType)) {
        // Map to smart order payload
        const smartPayload = {
          credential_id: credID,
          symbol,
          side: data.side as string,
          category: (data.category as string) ?? 'futures',
          order_type: orderType === 'tpsl' ? 'take_profit' : orderType === 'trailing_stop' ? 'trailing_stop' : 'stop_loss',
          quantity: (data as { sz?: number }).sz ?? 0,
          ...(orderType === 'trailing_stop' && 'callbackRatio' in data
            ? { callback_rate: (data as { callbackRatio?: number }).callbackRatio }
            : {}),
          ...((orderType === 'tpsl' || orderType === 'trigger') && 'triggerPx' in data
            ? { trigger_price: (data as { triggerPx?: number }).triggerPx }
            : {}),
        }
        smartMut.mutate(smartPayload)
      } else {
        // Map to regular order payload
        const sz = (data as { sz?: number }).sz ?? 0
        const regularPayload: Record<string, unknown> = {
          credential_id: credID,
          symbol,
          side: data.side as string,
          category: (data.category as string) ?? 'futures',
          type: orderType === 'advanced_limit' ? (data as { execMode: string }).execMode : orderType,
          price: (data as { px?: number }).px,
          leverage: category === 'futures' && 'lever' in data ? `${(data as { lever: number }).lever}x` : undefined,
        }
        if (sizeMode === 'pct') {
          regularPayload.amount_pct = sz
        } else {
          regularPayload.quantity = sz
        }
        orderMut.mutate(regularPayload as unknown as Parameters<typeof placeOrder>[0])
      }
    }
  })

  const handleTypeChange = (t: OrderType) => {
    setOrderType(t)
    // Reset type-specific fields, preserve common ones
    const side = watch('side')
    const cat = watch('category')
    const lever = watch('lever')
    methods.reset({ ordType: t, side, category: cat, lever } as Record<string, unknown>)
  }

  const isFutures = category === 'futures'
  const isPending = orderMut.isPending || smartMut.isPending

  return (
    <FormProvider {...methods}>
      <form onSubmit={onSubmit} className="bg-slate-800 rounded-xl border border-slate-700 p-5">
        <div className="flex items-center justify-between mb-4">
          <h2 className="font-semibold text-white">Place Order</h2>
          <OrderTypeSelector value={orderType} onChange={handleTypeChange} />
        </div>

        {feedback && (
          <div className={`mb-3 px-3 py-2 rounded text-sm ${
            feedback.type === 'ok'
              ? 'bg-green-900/40 border border-green-700 text-green-300'
              : 'bg-red-900/40 border border-red-700 text-red-300'
          }`}>
            {feedback.msg}
          </div>
        )}

        <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-3">
          {/* API Key / Group */}
          <FieldWrap label="API Key / Group">
            <SelectField value={selectedTarget} onChange={(e) => setSelectedTarget(e.target.value)} required>
              <option value="">Select...</option>
              {activeCreds.length > 0 && (
                <optgroup label="API Keys">
                  {activeCreds.map((c) => (
                    <option key={c.id} value={`cred:${c.id}`}>{credLabel(c)}</option>
                  ))}
                </optgroup>
              )}
              {groups.length > 0 && (
                <optgroup label="Groups">
                  {groups.map((g) => (
                    <option key={g.id} value={`group:${g.id}`}>{g.name} ({g.member_ids.length} keys)</option>
                  ))}
                </optgroup>
              )}
            </SelectField>
          </FieldWrap>

          {/* Side */}
          <FieldWrap label="Side">
            <div className="flex rounded-lg overflow-hidden border border-slate-600">
              {(['buy', 'sell'] as const).map((s) => (
                <button
                  key={s}
                  type="button"
                  onClick={() => setValue('side', s)}
                  className={`flex-1 py-2 text-sm font-medium transition-colors capitalize ${
                    watch('side') === s
                      ? s === 'buy' ? 'bg-green-600 text-white' : 'bg-red-600 text-white'
                      : 'bg-slate-700 text-slate-400 hover:bg-slate-600'
                  }`}
                >
                  {s}
                </button>
              ))}
            </div>
          </FieldWrap>

          {/* Category */}
          <FieldWrap label="Category">
            <RadioGroup
              value={watch('category') ?? 'futures'}
              onChange={(v) => setValue('category', v as 'spot' | 'futures')}
              options={[{ value: 'spot', label: 'Spot' }, { value: 'futures', label: 'Futures' }]}
            />
          </FieldWrap>

          {/* Leverage — futures only */}
          {isFutures && (
            <FieldWrap label="Leverage" error={errors.lever as { message?: string } | undefined}>
              <Field
                type="number"
                min="1"
                max="125"
                placeholder="10"
                {...register('lever', { valueAsNumber: true })}
              />
            </FieldWrap>
          )}

          {isFutures && (
            <FieldWrap label="Margin Mode">
              <RadioGroup
                value={watch('tdMode') ?? 'cross'}
                onChange={(v) => setValue('tdMode', v as 'cross' | 'isolated')}
                options={[
                  { value: 'cross', label: 'Cross' },
                  { value: 'isolated', label: 'Isolated' },
                ]}
              />
            </FieldWrap>
          )}

          {/* Size — shared across all types */}
          <FieldWrap label={
            <span className="flex items-center gap-2">
              Size
              <button
                type="button"
                onClick={() => setSizeMode(sizeMode === 'abs' ? 'pct' : 'abs')}
                className="text-xs px-1.5 py-0.5 rounded bg-slate-600 text-blue-400 hover:bg-slate-500"
              >
                {sizeMode === 'abs' ? 'USDT' : '%'}
              </button>
            </span>
          } error={errors.sz as { message?: string } | undefined}>
            <div className="relative">
              <Field
                type="number"
                step="any"
                min={sizeMode === 'pct' ? '0.1' : '0'}
                max={sizeMode === 'pct' ? '100' : undefined}
                placeholder={sizeMode === 'abs' ? '0.01' : '10'}
                {...register('sz', { valueAsNumber: true })}
                className="pr-14"
              />
              <span className="absolute right-3 top-1/2 -translate-y-1/2 text-xs text-slate-500 pointer-events-none">
                {sizeMode === 'abs' ? 'USDT' : '%'}
              </span>
            </div>
          </FieldWrap>

          {/* Type-specific sub-form */}
          {orderType === 'limit' && <LimitForm />}
          {orderType === 'market' && <MarketForm />}
          {orderType === 'tpsl' && <TpSlForm />}
          {orderType === 'chase' && <ChaseForm />}
          {orderType === 'advanced_limit' && <AdvancedLimitForm />}
          {orderType === 'trailing_stop' && <TrailingStopForm />}
          {orderType === 'trigger' && <TriggerForm />}
          {orderType === 'scaled' && <ScaledForm />}

          {/* Submit */}
          <div className="col-span-full pt-1 flex items-center gap-3">
            <button
              type="submit"
              disabled={isPending || !selectedTarget}
              title={!isValid ? 'Fill in all required fields' : undefined}
              className="px-6 py-2 bg-blue-600 hover:bg-blue-500 disabled:opacity-50 text-white text-sm font-medium rounded-lg transition-colors"
            >
              {isPending ? 'Placing...' : 'Place Order'}
            </button>
            {!isValid && <span className="text-xs text-slate-500">Fill all required fields</span>}
          </div>
        </div>
      </form>
    </FormProvider>
  )
}
