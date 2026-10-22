// Reconnecting WebSocket. onReconnect lets callers refetch state they may have missed.
export function connectWS(query, onMessage, onReconnect) {
  let ws = null
  let closed = false
  let attempt = 0
  let timer = null
  let everOpened = false

  function open() {
    const proto = location.protocol === 'https:' ? 'wss:' : 'ws:'
    ws = new WebSocket(`${proto}//${location.host}/ws?${query}`)
    ws.onopen = () => {
      if (everOpened && onReconnect) onReconnect()
      everOpened = true
      attempt = 0
    }
    ws.onmessage = (e) => {
      try { onMessage(JSON.parse(e.data)) } catch { /* ignore */ }
    }
    ws.onclose = () => {
      if (closed) return
      attempt++
      timer = setTimeout(open, Math.min(30000, 1000 * 2 ** Math.min(attempt, 5)))
    }
  }
  open()
  const onVisible = () => {
    if (document.visibilityState === 'visible' && ws && ws.readyState > 1) {
      clearTimeout(timer)
      open()
    }
  }
  document.addEventListener('visibilitychange', onVisible)
  return () => {
    closed = true
    clearTimeout(timer)
    document.removeEventListener('visibilitychange', onVisible)
    ws && ws.close()
  }
}
