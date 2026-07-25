import { describe, it, expect } from 'vitest'
import { toOkxPayload } from '../adapters/okx'
import type { AnyOrder } from '../types'

const base = {
  symbol: 'BTC-USDT-SWAP',
  credential_id: 1,
  category: 'futures' as const,
  side: 'buy' as const,
  tdMode: 'cross' as const,
  lever: 10,
}

describe('toOkxPayload', () => {
  it('limit → /trade/order with px and sz', () => {
    const order: AnyOrder = { ordType: 'limit', ...base, px: 65000, sz: 0.01 }
    const { endpoint, payload } = toOkxPayload(order)
    expect(endpoint).toBe('/api/v5/trade/order')
    expect(payload.ordType).toBe('limit')
    expect(payload.px).toBe('65000')
    expect(payload.sz).toBe('0.01')
    expect(payload.lever).toBe('10')
  })

  it('market → /trade/order with ordType market', () => {
    const order: AnyOrder = { ordType: 'market', ...base, sz: 0.01 }
    const { endpoint, payload } = toOkxPayload(order)
    expect(endpoint).toBe('/api/v5/trade/order')
    expect(payload.ordType).toBe('market')
    expect(payload.px).toBeUndefined()
  })

  it('tpsl with both TP and SL → oco + /trade/order-algo', () => {
    const order: AnyOrder = {
      ordType: 'tpsl', ...base, sz: 0.01,
      tp: { tpTriggerPx: 70000, tpOrdPx: -1, tpTriggerPxType: 'last' },
      sl: { slTriggerPx: 60000, slOrdPx: -1, slTriggerPxType: 'last' },
    }
    const { endpoint, payload } = toOkxPayload(order)
    expect(endpoint).toBe('/api/v5/trade/order-algo')
    expect(payload.ordType).toBe('oco')
    expect(payload.tpTriggerPx).toBe('70000')
    expect(payload.slTriggerPx).toBe('60000')
    expect(payload.tpOrdPx).toBe('-1')
  })

  it('chase → /trade/order-algo', () => {
    const order: AnyOrder = {
      ordType: 'chase', ...base, sz: 0.01,
      chaseType: 'distance', chaseVal: 50,
      maxChaseType: 'distance', maxChaseVal: 200,
    }
    const { endpoint, payload } = toOkxPayload(order)
    expect(endpoint).toBe('/api/v5/trade/order-algo')
    expect(payload.chaseVal).toBe('50')
  })

  it('advanced_limit post_only → /trade/order with ordType post_only', () => {
    const order: AnyOrder = { ordType: 'advanced_limit', ...base, execMode: 'post_only', px: 65000, sz: 0.01 }
    const { endpoint, payload } = toOkxPayload(order)
    expect(endpoint).toBe('/api/v5/trade/order')
    expect(payload.ordType).toBe('post_only')
  })

  it('trailing_stop ratio mode → move_order_stop with callbackRatio as decimal', () => {
    const order: AnyOrder = {
      ordType: 'trailing_stop', ...base, sz: 0.01,
      callbackMode: 'ratio', callbackRatio: 1.5,
    }
    const { endpoint, payload } = toOkxPayload(order)
    expect(endpoint).toBe('/api/v5/trade/order-algo')
    expect(payload.ordType).toBe('move_order_stop')
    expect(payload.callbackRatio).toBe('0.015')
  })

  it('trigger with market execution (orderPx=-1)', () => {
    const order: AnyOrder = {
      ordType: 'trigger', ...base, sz: 0.01,
      triggerPx: 65000, triggerPxType: 'last', orderPx: -1,
    }
    const { endpoint, payload } = toOkxPayload(order)
    expect(endpoint).toBe('/api/v5/trade/order-algo')
    expect(payload.orderPx).toBe('-1')
  })

  it('scaled flat → twap with szList and pxList', () => {
    const order: AnyOrder = {
      ordType: 'scaled', ...base,
      priceLow: 60000, priceHigh: 62000, subOrderCount: 3, totalSz: 0.03, distribution: 'flat',
    }
    const { endpoint, payload } = toOkxPayload(order)
    expect(endpoint).toBe('/api/v5/trade/order-algo')
    expect(payload.szList).toBeDefined()
    expect(payload.pxList).toBeDefined()
    // 3 equal parts
    const sizes = payload.szList!.split(',').map(Number)
    expect(sizes).toHaveLength(3)
    expect(sizes[0]).toBeCloseTo(0.01)
  })
})
