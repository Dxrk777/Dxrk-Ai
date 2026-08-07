import { useEffect, useRef, useCallback, useState } from 'react'

type MessageHandler = (data: Record<string, unknown>) => void

interface UseWebSocketReturn {
  connected: boolean
  send: (msg: object) => void
  on: (event: string, handler: MessageHandler) => void
  off: (event: string) => void
}

export function useWebSocket(url: string): UseWebSocketReturn {
  const ws = useRef<WebSocket | null>(null)
  const handlers = useRef<Map<string, MessageHandler>>(new Map())
  const [connected, setConnected] = useState(false)
  const reconnectTimeout = useRef<number | undefined>(undefined)
  const connectRef = useRef<() => void>(() => {})

  const doConnect = useCallback(() => {
    if (ws.current?.readyState === WebSocket.OPEN) return

    const socket = new WebSocket(url)
    ws.current = socket

    socket.onopen = () => setConnected(true)

    socket.onclose = () => {
      setConnected(false)
      reconnectTimeout.current = window.setTimeout(() => connectRef.current(), 3000)
    }

    socket.onmessage = (event) => {
      try {
        const msg = JSON.parse(event.data) as Record<string, unknown>
        const method = (msg.method || msg.type || 'message') as string
        const handler = handlers.current.get(method)
        if (handler) handler(msg)
      } catch {
        // ignore malformed messages
      }
    }

    socket.onerror = () => socket.close()
  }, [url])

  useEffect(() => {
    connectRef.current = doConnect
  }, [doConnect])

  useEffect(() => {
    doConnect()
    return () => {
      clearTimeout(reconnectTimeout.current)
      ws.current?.close()
    }
  }, [doConnect])

  const send = useCallback((msg: object) => {
    if (ws.current?.readyState === WebSocket.OPEN) {
      ws.current.send(JSON.stringify(msg))
    }
  }, [])

  const on = useCallback((event: string, handler: MessageHandler) => {
    handlers.current.set(event, handler)
  }, [])

  const off = useCallback((event: string) => {
    handlers.current.delete(event)
  }, [])

  return { connected, send, on, off }
}
