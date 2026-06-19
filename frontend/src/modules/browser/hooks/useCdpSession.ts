import { useCallback, useEffect, useRef, useState } from 'react'
import { toast } from '../../../shared/components'

export interface CdpStartOptions {
  wsUrl: string
  // 连接建立后回调：用 send 启用所需 CDP 域；isReconnect 区分首连/重连。
  onOpen: (send: (payload: any) => void, isReconnect: boolean) => void
  // 收到 CDP 消息回调（已 JSON.parse）。
  onMessage: (data: any, send: (payload: any) => void) => void
}

const MAX_RECONNECT = 3

// useCdpSession 封装 DevTools 的 CDP WebSocket 生命周期（连接/重连/发送）。
// 仅拥有 WebSocket 与重连状态；页面通过 onOpen/onMessage 注入业务逻辑，保持原行为。
// 用 ref 跟踪重连计数，修复原页面 onclose 中读取 state 的 stale-closure 隐患。
export function useCdpSession() {
  const wsRef = useRef<WebSocket | null>(null)
  const optsRef = useRef<CdpStartOptions | null>(null)
  const attemptsRef = useRef(0)
  const manualStopRef = useRef(false)
  const reconnectingRef = useRef(false)
  const timerRef = useRef<number | null>(null)
  const [capturing, setCapturing] = useState(false)
  const [reconnecting, setReconnecting] = useState(false)

  const setReconn = useCallback((v: boolean) => {
    reconnectingRef.current = v
    setReconnecting(v)
  }, [])

  const send = useCallback((payload: any) => {
    const ws = wsRef.current
    if (ws && ws.readyState === WebSocket.OPEN) {
      ws.send(JSON.stringify(payload))
    }
  }, [])

  // getSocket 暴露当前 WebSocket，供需要临时注册一次性 message 监听的场景（存储/JS/截图）。
  const getSocket = useCallback(() => wsRef.current, [])

  const connect = useCallback((isReconnect: boolean) => {
    const opts = optsRef.current
    if (!opts) return
    try {
      const websocket = new WebSocket(opts.wsUrl)
      wsRef.current = websocket

      websocket.onopen = () => {
        setCapturing(true)
        setReconn(false)
        attemptsRef.current = 0
        opts.onOpen(send, isReconnect)
        toast.success(isReconnect ? '重连成功' : '开发工具已连接')
      }

      websocket.onmessage = (event) => {
        try {
          const data = JSON.parse(event.data)
          opts.onMessage(data, send)
        } catch (error) {
          console.error('解析消息失败:', error)
        }
      }

      websocket.onerror = () => {
        if (!reconnectingRef.current) {
          toast.error('WebSocket连接失败')
        }
      }

      websocket.onclose = () => {
        setCapturing(false)
        wsRef.current = null

        if (manualStopRef.current) {
          manualStopRef.current = false
          toast.info('开发工具已断开')
          return
        }

        if (attemptsRef.current < MAX_RECONNECT) {
          attemptsRef.current += 1
          setReconn(true)
          toast.info(`连接断开，正在重连... (${attemptsRef.current}/${MAX_RECONNECT})`)
          timerRef.current = window.setTimeout(() => connect(true), 2000 * attemptsRef.current)
        } else {
          toast.error('重连失败，请手动重新连接')
          setReconn(false)
          attemptsRef.current = 0
        }
      }
    } catch (error: any) {
      toast.error(error?.message || '启动失败')
      setReconn(false)
    }
  }, [send, setReconn])

  const start = useCallback((opts: CdpStartOptions) => {
    optsRef.current = opts
    manualStopRef.current = false
    attemptsRef.current = 0
    connect(false)
  }, [connect])

  const stop = useCallback(() => {
    manualStopRef.current = true
    if (timerRef.current) {
      clearTimeout(timerRef.current)
      timerRef.current = null
    }
    attemptsRef.current = 0
    setReconn(false)
    setCapturing(false)
    if (wsRef.current) {
      wsRef.current.close()
      wsRef.current = null
    }
  }, [setReconn])

  // 卸载时清理定时器与连接
  useEffect(() => () => {
    if (timerRef.current) clearTimeout(timerRef.current)
    if (wsRef.current) wsRef.current.close()
  }, [])

  return { capturing, reconnecting, start, stop, send, getSocket }
}
