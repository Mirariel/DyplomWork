import { useEffect, useRef, useState, useCallback } from 'react'
import type { LivePosition } from './api'

// ─── WS Message types ─────────────────────────────────────────────────────────

interface WsAuthMessage {
  type: 'auth'
  token: string
}

interface WsUpdateMessage {
  type: 'update'
  positions: LivePosition[]
  spot_prices: Record<string, number>
}

type WsIncomingMessage = WsUpdateMessage | { type: string }

// ─── Hook ─────────────────────────────────────────────────────────────────────

export interface WsState {
  positions: LivePosition[]
  spotPrices: Record<string, number>
  connected: boolean
}

export function useWebSocket(): WsState {
  const [positions, setPositions] = useState<LivePosition[]>([])
  const [spotPrices, setSpotPrices] = useState<Record<string, number>>({})
  const [connected, setConnected] = useState(false)
  const wsRef = useRef<WebSocket | null>(null)
  const reconnectTimer = useRef<ReturnType<typeof setTimeout> | null>(null)
  const unmounted = useRef(false)

  const connect = useCallback(() => {
    if (unmounted.current) return

    const token = localStorage.getItem('tt_token') ?? ''
    const protocol = window.location.protocol === 'https:' ? 'wss' : 'ws'
    const host = window.location.host
    const ws = new WebSocket(`${protocol}://${host}/ws`)
    wsRef.current = ws

    ws.onopen = () => {
      setConnected(true)
      const authMsg: WsAuthMessage = { type: 'auth', token }
      ws.send(JSON.stringify(authMsg))
    }

    ws.onmessage = (event: MessageEvent<string>) => {
      try {
        const msg = JSON.parse(event.data) as WsIncomingMessage
        if (msg.type === 'update') {
          const upd = msg as WsUpdateMessage
          setPositions(upd.positions ?? [])
          setSpotPrices(upd.spot_prices ?? {})
        }
      } catch {
        // ignore malformed messages
      }
    }

    ws.onclose = () => {
      setConnected(false)
      if (!unmounted.current) {
        reconnectTimer.current = setTimeout(connect, 3000)
      }
    }

    ws.onerror = () => {
      ws.close()
    }
  }, [])

  useEffect(() => {
    unmounted.current = false
    connect()

    return () => {
      unmounted.current = true
      if (reconnectTimer.current) clearTimeout(reconnectTimer.current)
      wsRef.current?.close()
    }
  }, [connect])

  return { positions, spotPrices, connected }
}
