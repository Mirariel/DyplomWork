import { describe, it, expect } from 'vitest'
import {
  limitSchema, marketSchema, tpslSchema, chaseSchema,
  advancedLimitSchema, trailingStopSchema, triggerSchema, scaledSchema,
} from '../schemas'

const base = {
  symbol: 'BTC-USDT-SWAP',
  credential_id: 1,
  category: 'futures' as const,
  side: 'buy' as const,
  tdMode: 'cross' as const,
  lever: 10,
}

describe('limitSchema', () => {
  it('valid', () => {
    expect(limitSchema.safeParse({ ordType: 'limit', ...base, px: 65000, sz: 0.01 }).success).toBe(true)
  })
  it('rejects px=0', () => {
    expect(limitSchema.safeParse({ ordType: 'limit', ...base, px: 0, sz: 0.01 }).success).toBe(false)
  })
})

describe('marketSchema', () => {
  it('valid', () => {
    expect(marketSchema.safeParse({ ordType: 'market', ...base, sz: 0.01 }).success).toBe(true)
  })
  it('rejects sz<=0', () => {
    expect(marketSchema.safeParse({ ordType: 'market', ...base, sz: -1 }).success).toBe(false)
  })
})

describe('tpslSchema', () => {
  it('valid TP only', () => {
    expect(tpslSchema.safeParse({
      ordType: 'tpsl', ...base, sz: 0.01,
      tp: { tpTriggerPx: 70000, tpOrdPx: -1, tpTriggerPxType: 'last' },
    }).success).toBe(true)
  })
  it('rejects empty TP and SL', () => {
    expect(tpslSchema.safeParse({ ordType: 'tpsl', ...base, sz: 0.01 }).success).toBe(false)
  })
})

describe('chaseSchema', () => {
  it('valid current type', () => {
    expect(chaseSchema.safeParse({
      ordType: 'chase', ...base, sz: 0.01,
      chaseType: 'current', maxChaseType: 'distance', maxChaseVal: 100,
    }).success).toBe(true)
  })
  it('rejects distance without chaseVal', () => {
    expect(chaseSchema.safeParse({
      ordType: 'chase', ...base, sz: 0.01,
      chaseType: 'distance', maxChaseType: 'distance', maxChaseVal: 100,
    }).success).toBe(false)
  })
})

describe('advancedLimitSchema', () => {
  it('valid post_only', () => {
    expect(advancedLimitSchema.safeParse({
      ordType: 'advanced_limit', ...base, execMode: 'post_only', px: 65000, sz: 0.01,
    }).success).toBe(true)
  })
})

describe('trailingStopSchema', () => {
  it('valid ratio mode', () => {
    expect(trailingStopSchema.safeParse({
      ordType: 'trailing_stop', ...base, sz: 0.01, callbackMode: 'ratio', callbackRatio: 1.5,
    }).success).toBe(true)
  })
  it('rejects missing callback value', () => {
    expect(trailingStopSchema.safeParse({
      ordType: 'trailing_stop', ...base, sz: 0.01, callbackMode: 'ratio',
    }).success).toBe(false)
  })
})

describe('triggerSchema', () => {
  it('valid market execution', () => {
    expect(triggerSchema.safeParse({
      ordType: 'trigger', ...base, sz: 0.01, triggerPx: 65000, triggerPxType: 'last', orderPx: -1,
    }).success).toBe(true)
  })
})

describe('scaledSchema', () => {
  it('valid flat distribution', () => {
    expect(scaledSchema.safeParse({
      ordType: 'scaled', ...base, priceLow: 60000, priceHigh: 70000,
      subOrderCount: 5, totalSz: 0.05, distribution: 'flat',
    }).success).toBe(true)
  })
  it('rejects priceLow >= priceHigh', () => {
    expect(scaledSchema.safeParse({
      ordType: 'scaled', ...base, priceLow: 70000, priceHigh: 60000,
      subOrderCount: 5, totalSz: 0.05, distribution: 'flat',
    }).success).toBe(false)
  })
  it('rejects subOrderCount < 2', () => {
    expect(scaledSchema.safeParse({
      ordType: 'scaled', ...base, priceLow: 60000, priceHigh: 70000,
      subOrderCount: 1, totalSz: 0.05, distribution: 'flat',
    }).success).toBe(false)
  })
})
