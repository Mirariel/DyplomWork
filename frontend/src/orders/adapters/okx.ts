import type { AnyOrder } from '../types'

export interface OkxOrderPayload {
  instId: string
  tdMode: string
  side: string
  ordType: string
  sz: string
  px?: string
  reduceOnly?: boolean
  attachAlgoOrds?: OkxAttachAlgoOrd[]
  lever?: string
  // algo fields
  tpTriggerPx?: string
  tpOrdPx?: string
  tpTriggerPxType?: string
  slTriggerPx?: string
  slOrdPx?: string
  slTriggerPxType?: string
  triggerPx?: string
  triggerPxType?: string
  orderPx?: string
  activePx?: string
  callbackRatio?: string
  callbackSpread?: string
  chaseType?: string
  chaseVal?: string
  maxChaseType?: string
  maxChaseVal?: string
  pxLow?: string
  pxHigh?: string
  szList?: string
  pxList?: string
}

export interface OkxAttachAlgoOrd {
  tpTriggerPx?: string
  tpOrdPx?: string
  tpTriggerPxType?: string
  slTriggerPx?: string
  slOrdPx?: string
  slTriggerPxType?: string
}

export type OkxEndpoint = '/api/v5/trade/order' | '/api/v5/trade/order-algo'

export interface OkxRequest {
  endpoint: OkxEndpoint
  payload: OkxOrderPayload
}

function px(n: number | -1): string {
  return n === -1 ? '-1' : String(n)
}

export function toOkxPayload(order: AnyOrder): OkxRequest {
  const instId = order.symbol
  const side = order.side
  const tdMode = order.category === 'spot' ? 'cash' : ('tdMode' in order ? order.tdMode : 'cross')
  const lever = 'lever' in order ? String(order.lever) : undefined

  switch (order.ordType) {
    case 'limit': {
      const payload: OkxOrderPayload = {
        instId, tdMode, side,
        ordType: 'limit',
        sz: String(order.sz),
        px: String(order.px),
        reduceOnly: order.reduceOnly,
        lever,
      }
      if (order.attachTpSl && (order.attachedTp || order.attachedSl)) {
        const attachAlgoOrds: OkxAttachAlgoOrd = {}
        if (order.attachedTp) {
          attachAlgoOrds.tpTriggerPx = String(order.attachedTp.tpTriggerPx)
          attachAlgoOrds.tpOrdPx = px(order.attachedTp.tpOrdPx)
          attachAlgoOrds.tpTriggerPxType = order.attachedTp.tpTriggerPxType
        }
        if (order.attachedSl) {
          attachAlgoOrds.slTriggerPx = String(order.attachedSl.slTriggerPx)
          attachAlgoOrds.slOrdPx = px(order.attachedSl.slOrdPx)
          attachAlgoOrds.slTriggerPxType = order.attachedSl.slTriggerPxType
        }
        payload.attachAlgoOrds = [attachAlgoOrds]
      }
      return { endpoint: '/api/v5/trade/order', payload }
    }

    case 'market': {
      return {
        endpoint: '/api/v5/trade/order',
        payload: { instId, tdMode, side, ordType: 'market', sz: String(order.sz), reduceOnly: order.reduceOnly, lever },
      }
    }

    case 'tpsl': {
      const payload: OkxOrderPayload = {
        instId, tdMode, side,
        ordType: order.tp && order.sl ? 'oco' : 'conditional',
        sz: String(order.sz),
        lever,
      }
      if (order.closeFraction !== undefined) payload.sz = String(order.closeFraction)
      if (order.tp) {
        payload.tpTriggerPx = String(order.tp.tpTriggerPx)
        payload.tpOrdPx = px(order.tp.tpOrdPx)
        payload.tpTriggerPxType = order.tp.tpTriggerPxType
      }
      if (order.sl) {
        payload.slTriggerPx = String(order.sl.slTriggerPx)
        payload.slOrdPx = px(order.sl.slOrdPx)
        payload.slTriggerPxType = order.sl.slTriggerPxType
      }
      return { endpoint: '/api/v5/trade/order-algo', payload }
    }

    case 'chase': {
      return {
        endpoint: '/api/v5/trade/order-algo',
        payload: {
          instId, tdMode, side,
          ordType: 'chase',
          sz: String(order.sz),
          chaseType: order.chaseType,
          chaseVal: order.chaseVal !== undefined ? String(order.chaseVal) : undefined,
          maxChaseType: order.maxChaseType,
          maxChaseVal: String(order.maxChaseVal),
          reduceOnly: order.reduceOnly,
          lever,
        },
      }
    }

    case 'advanced_limit': {
      return {
        endpoint: '/api/v5/trade/order',
        payload: {
          instId, tdMode, side,
          ordType: order.execMode,
          sz: String(order.sz),
          px: String(order.px),
          reduceOnly: order.reduceOnly,
          lever,
        },
      }
    }

    case 'trailing_stop': {
      const payload: OkxOrderPayload = {
        instId, tdMode, side,
        ordType: 'move_order_stop',
        sz: String(order.sz),
        lever,
      }
      if (order.activePx !== undefined) payload.activePx = String(order.activePx)
      if (order.callbackMode === 'ratio') payload.callbackRatio = String(order.callbackRatio! / 100)
      else payload.callbackSpread = String(order.callbackSpread)
      if (order.reduceOnly) payload.reduceOnly = true
      return { endpoint: '/api/v5/trade/order-algo', payload }
    }

    case 'trigger': {
      return {
        endpoint: '/api/v5/trade/order-algo',
        payload: {
          instId, tdMode, side,
          ordType: 'trigger',
          sz: String(order.sz),
          triggerPx: String(order.triggerPx),
          triggerPxType: order.triggerPxType,
          orderPx: px(order.orderPx),
          reduceOnly: order.reduceOnly,
          lever,
        },
      }
    }

    case 'scaled': {
      const step = (order.priceHigh - order.priceLow) / (order.subOrderCount - 1)
      const prices = Array.from({ length: order.subOrderCount }, (_, i) => order.priceLow + i * step)
      const weights = prices.map((_, i) => {
        if (order.distribution === 'flat') return 1
        return order.distribution === 'ascending' ? i + 1 : order.subOrderCount - i
      })
      const wSum = weights.reduce((a, b) => a + b, 0)
      const sizes = weights.map((w) => (w / wSum) * order.totalSz)
      return {
        endpoint: '/api/v5/trade/order-algo',
        payload: {
          instId, tdMode, side,
          ordType: 'twap',
          sz: String(order.totalSz),
          pxLow: String(order.priceLow),
          pxHigh: String(order.priceHigh),
          szList: sizes.map(s => s.toFixed(8)).join(','),
          pxList: prices.map(p => p.toFixed(2)).join(','),
          lever,
        },
      }
    }
  }
}
