import { z } from 'zod'

const side = z.enum(['buy', 'sell'])
const marginMode = z.enum(['cross', 'isolated'])
const triggerPxType = z.enum(['last', 'mark', 'index'])
const distribution = z.enum(['flat', 'ascending', 'descending'])

const baseFields = {
  side,
  symbol: z.string().min(1, 'Symbol required'),
  credential_id: z.number().int().positive('Select an API key'),
  category: z.enum(['spot', 'futures']),
}

const futuresFields = {
  tdMode: marginMode,
  lever: z.number().int().min(1).max(125),
}

export const limitSchema = z.object({
  ordType: z.literal('limit'),
  ...baseFields,
  ...futuresFields,
  px: z.number().positive('Price must be > 0'),
  sz: z.number().positive('Size must be > 0'),
  reduceOnly: z.boolean().optional(),
  attachTpSl: z.boolean().optional(),
})

export const marketSchema = z.object({
  ordType: z.literal('market'),
  ...baseFields,
  ...futuresFields,
  sz: z.number().positive('Size must be > 0'),
  reduceOnly: z.boolean().optional(),
})

export const tpslSchema = z.object({
  ordType: z.literal('tpsl'),
  ...baseFields,
  sz: z.number().positive('Size must be > 0'),
  closeFraction: z.number().min(0).max(1).optional(),
  tp: z.object({
    tpTriggerPx: z.number().positive(),
    tpOrdPx: z.union([z.number().positive(), z.literal(-1)]),
    tpTriggerPxType: triggerPxType,
  }).optional(),
  sl: z.object({
    slTriggerPx: z.number().positive(),
    slOrdPx: z.union([z.number().positive(), z.literal(-1)]),
    slTriggerPxType: triggerPxType,
  }).optional(),
}).refine(
  (data) => data.tp !== undefined || data.sl !== undefined,
  { message: 'At least one of TP or SL must be set' },
)

export const chaseSchema = z.object({
  ordType: z.literal('chase'),
  ...baseFields,
  ...futuresFields,
  sz: z.number().positive('Size must be > 0'),
  chaseType: z.enum(['current', 'distance']),
  chaseVal: z.number().optional(),
  maxChaseType: z.enum(['distance', 'ratio']),
  maxChaseVal: z.number().positive('Max chase value must be > 0'),
  reduceOnly: z.boolean().optional(),
}).refine(
  (data) => data.chaseType !== 'distance' || (data.chaseVal !== undefined && data.chaseVal > 0),
  { message: 'Chase value required when type is distance', path: ['chaseVal'] },
)

export const advancedLimitSchema = z.object({
  ordType: z.literal('advanced_limit'),
  ...baseFields,
  ...futuresFields,
  execMode: z.enum(['post_only', 'fok', 'ioc']),
  px: z.number().positive('Price must be > 0'),
  sz: z.number().positive('Size must be > 0'),
  reduceOnly: z.boolean().optional(),
})

export const trailingStopSchema = z.object({
  ordType: z.literal('trailing_stop'),
  ...baseFields,
  ...futuresFields,
  sz: z.number().positive('Size must be > 0'),
  activePx: z.number().positive().optional(),
  callbackMode: z.enum(['ratio', 'spread']),
  callbackRatio: z.number().min(0.01).max(50).optional(),
  callbackSpread: z.number().positive().optional(),
  reduceOnly: z.boolean().optional(),
}).refine(
  (data) => {
    if (data.callbackMode === 'ratio') return data.callbackRatio !== undefined && data.callbackRatio > 0
    return data.callbackSpread !== undefined && data.callbackSpread > 0
  },
  { message: 'Callback value required', path: ['callbackRatio'] },
)

export const triggerSchema = z.object({
  ordType: z.literal('trigger'),
  ...baseFields,
  ...futuresFields,
  sz: z.number().positive('Size must be > 0'),
  triggerPx: z.number().positive('Trigger price must be > 0'),
  triggerPxType: triggerPxType,
  orderPx: z.union([z.number().positive(), z.literal(-1)]),
  reduceOnly: z.boolean().optional(),
})

export const scaledSchema = z.object({
  ordType: z.literal('scaled'),
  ...baseFields,
  ...futuresFields,
  priceLow: z.number().positive('Low price must be > 0'),
  priceHigh: z.number().positive('High price must be > 0'),
  subOrderCount: z.number().int().min(2).max(100),
  totalSz: z.number().positive('Total size must be > 0'),
  distribution,
  reduceOnly: z.boolean().optional(),
}).refine(
  (data) => data.priceLow < data.priceHigh,
  { message: 'Low price must be less than high price', path: ['priceHigh'] },
)

export const anyOrderSchema = z.union([
  limitSchema,
  marketSchema,
  tpslSchema,
  chaseSchema,
  advancedLimitSchema,
  trailingStopSchema,
  triggerSchema,
  scaledSchema,
])

export type LimitInput = z.infer<typeof limitSchema>
export type MarketInput = z.infer<typeof marketSchema>
export type TpSlInput = z.infer<typeof tpslSchema>
export type ChaseInput = z.infer<typeof chaseSchema>
export type AdvancedLimitInput = z.infer<typeof advancedLimitSchema>
export type TrailingStopInput = z.infer<typeof trailingStopSchema>
export type TriggerInput = z.infer<typeof triggerSchema>
export type ScaledInput = z.infer<typeof scaledSchema>
export type AnyOrderInput = z.infer<typeof anyOrderSchema>
