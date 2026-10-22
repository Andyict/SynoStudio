export function fmtDate(ts, withTime = false, lang = 'vi') {
  if (!ts) return ''
  const d = new Date(ts * 1000)
  const opt = { day: '2-digit', month: '2-digit', year: 'numeric' }
  if (withTime) Object.assign(opt, { hour: '2-digit', minute: '2-digit' })
  return d.toLocaleString(lang === 'en' ? 'en-GB' : 'vi-VN', opt)
}

export function fmtTime(ts, lang = 'vi') {
  if (!ts) return ''
  const d = new Date(ts * 1000)
  const today = new Date()
  const sameDay = d.toDateString() === today.toDateString()
  const loc = lang === 'en' ? 'en-GB' : 'vi-VN'
  return sameDay
    ? d.toLocaleTimeString(loc, { hour: '2-digit', minute: '2-digit' })
    : d.toLocaleString(loc, { day: '2-digit', month: '2-digit', hour: '2-digit', minute: '2-digit' })
}

export function fmtBytes(n) {
  if (!n) return '0 B'
  const u = ['B', 'KB', 'MB', 'GB', 'TB']
  let i = 0
  while (n >= 1024 && i < u.length - 1) { n /= 1024; i++ }
  return `${n.toFixed(i ? 1 : 0)} ${u[i]}`
}

// <input type="date"> helpers (local midnight / end of day)
export function tsToDateInput(ts) {
  if (!ts) return ''
  const d = new Date(ts * 1000)
  const p = (n) => String(n).padStart(2, '0')
  return `${d.getFullYear()}-${p(d.getMonth() + 1)}-${p(d.getDate())}`
}
export function dateInputToTs(s, endOfDay = true) {
  if (!s) return 0
  const [y, m, d] = s.split('-').map(Number)
  const dt = endOfDay ? new Date(y, m - 1, d, 23, 59, 59) : new Date(y, m - 1, d)
  return Math.floor(dt.getTime() / 1000)
}

export function daysLeft(ts) {
  if (!ts) return null
  return Math.ceil((ts * 1000 - Date.now()) / 86400000)
}

export const imgUrl = (id, size, guest) => `/img/${id}/${size}${guest ? `?g=${encodeURIComponent(guest)}` : ''}`

export function download(url) {
  const a = document.createElement('a')
  a.href = url
  a.rel = 'noopener'
  document.body.appendChild(a)
  a.click()
  a.remove()
}

export async function copyText(text) {
  try {
    await navigator.clipboard.writeText(text)
    return true
  } catch {
    const ta = document.createElement('textarea')
    ta.value = text
    document.body.appendChild(ta)
    ta.select()
    const ok = document.execCommand('copy')
    ta.remove()
    return ok
  }
}
