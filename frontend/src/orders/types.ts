// ─── Shared fields present in all order types ─────────────────────────────────

export type Side = 'buy' | 'sell'
export type MarginMode = 'cross' | 'isolated'
export type TriggerPxType = 'last' | 'mark' | 'index'
export type ExecMode = 'post_only' | 'fok' | 'ioc'
export type ChaseType = 'current' | 'distance'
export type MaxChaseType = 'distance' | 'ratio'
export type Distribution = 'flat' | 'ascending' | 'descending'

export interface BaseOrderFields {
  side: Side
  symbol: string           // e.g. "BTC-USDT-SWAP"
  credential_id: number
  category: 'spot' | 'futures'
}

export interface FuturesFields {
  tdMode: MarginMode
  lever: number            // leverage multiplier
}

// ─── 1. Limit order ───────────────────────────────────────────────────────────

export interface LimitOrder extends BaseOrderFields, FuturesFields {
  ordType: 'limit'
  px: number
  sz: number
  reduceOnly?: boolean
  attachTpSl?: boolean
  // if attachTpSl=true:
  attachedTp?: { tpTriggerPx: number; tpOrdPx: number | -1; tpTriggerPxType: TriggerPxType }
  attachedSl?: { slTriggerPx: number; slOrdPx: number | -1; slTriggerPxType: TriggerPxType }
}

// ─── 2. Market order ─────────────────────────────────────────────────────────

export interface MarketOrder extends BaseOrderFields, FuturesFields {
  ordType: 'market'
  sz: number
  reduceOnly?: boolean
}

// ─── 3. TP/SL (conditional / OCO) ────────────────────────────────────────────

export interface TpSlOrder extends BaseOrderFields {
  ordType: 'tpsl'
  sz: number
  closeFraction?: number  // 1 = close whole position
  tp?: {
    tpTriggerPx: number
    tpOrdPx: number | -1
    tpTriggerPxType: TriggerPxType
  }
  sl?: {
    slTriggerPx: number
    slOrdPx: number | -1
    slTriggerPxType: TriggerPxType
  }
}

// ─── 4. Chase (sliding limit) ─────────────────────────────────────────────────

export interface ChaseOrder extends BaseOrderFields, FuturesFields {
  ordType: 'chase'
  sz: number
  chaseType: ChaseType
  chaseVal?: number       // required if chaseType='distance'
  maxChaseType: MaxChaseType
  maxChaseVal: number
  reduceOnly?: boolean
}

// ─── 5. Advanced limit (post_only / FOK / IOC) ───────────────────────────────

export interface AdvancedLimitOrder extends BaseOrderFields, FuturesFields {
  ordType: 'advanced_limit'
  execMode: ExecMode
  px: number
  sz: number
  reduceOnly?: boolean
}

// ─── 6. Trailing stop ─────────────────────────────────────────────────────────

export interface TrailingStopOrder extends BaseOrderFields, FuturesFields {
  ordType: 'trailing_stop'
  side: Side
  sz: number
  activePx?: number
  callbackMode: 'ratio' | 'spread'
  callbackRatio?: number   // % (e.g. 1.5 means 1.5%)
  callbackSpread?: number  // absolute value
  reduceOnly?: boolean
}

// ─── 7. Trigger ──────────────────────────────────────────────────────────────

export interface TriggerOrder extends BaseOrderFields, FuturesFields {
  ordType: 'trigger'
  sz: number
  triggerPx: number
  triggerPxType: TriggerPxType
  orderPx: number | -1     // -1 = market
  reduceOnly?: boolean
}

// ─── 8. Scaled (TWAP) ────────────────────────────────────────────────────────

export interface ScaledOrder extends BaseOrderFields, FuturesFields {
  ordType: 'scaled'
  priceLow: number
  priceHigh: number
  subOrderCount: number    // 2-100
  totalSz: number
  distribution: Distribution
  reduceOnly?: boolean
}

// ─── Union ────────────────────────────────────────────────────────────────────

export type AnyOrder =
  | LimitOrder
  | MarketOrder
  | TpSlOrder
  | ChaseOrder
  | AdvancedLimitOrder
  | TrailingStopOrder
  | TriggerOrder
  | ScaledOrder

export type OrderType = AnyOrder['ordType']

// Sub-order preview for scaled orders
export interface SubOrderPreview {
  price: number
  size: number
}

export function calcScaledSubOrders(order: ScaledOrder): SubOrderPreview[] {
  const { priceLow, priceHigh, subOrderCount, totalSz, distribution } = order
  if (subOrderCount < 2) return []
  const step = (priceHigh - priceLow) / (subOrderCount - 1)
  const prices = Array.from({ length: subOrderCount }, (_, i) => priceLow + i * step)

  let sizes: number[]
  if (distribution === 'flat') {
    sizes = prices.map(() => totalSz / subOrderCount)
  } else {
    // linear weights
    const weights = prices.map((_, i) =>
      distribution === 'ascending' ? i + 1 : subOrderCount - i
    )
    const wSum = weights.reduce((a, b) => a + b, 0)
    sizes = weights.map((w) => (w / wSum) * totalSz)
  }

  return prices.map((price, i) => ({ price, size: sizes[i] }))
}
