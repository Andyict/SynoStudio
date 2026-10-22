import { reactive } from 'vue'

// Toasts
export const toasts = reactive([])
let seq = 0
export function toast(text, type = 'ok', ms = 2600) {
  const id = ++seq
  toasts.push({ id, text, type })
  setTimeout(() => {
    const i = toasts.findIndex((t) => t.id === id)
    if (i >= 0) toasts.splice(i, 1)
  }, ms)
}
export const toastErr = (e) => toast(e?.message || String(e), 'err', 4000)

// Applies studio branding (colour, title) to the page.
export function applyBranding(b) {
  if (!b) return
  if (b.color) {
    document.documentElement.style.setProperty('--brand', b.color)
    const meta = document.querySelector('meta[name=theme-color]')
    if (meta) meta.setAttribute('content', b.color)
    // pick readable text colour on the brand colour
    const c = b.color.replace('#', '')
    const [r, g, bl] = [0, 2, 4].map((i) => parseInt(c.slice(i, i + 2), 16))
    const lum = (0.299 * r + 0.587 * g + 0.114 * bl) / 255
    document.documentElement.style.setProperty('--brand-ink', lum > 0.7 ? '#1f1d1b' : '#ffffff')
  }
}
