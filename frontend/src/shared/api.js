// Small fetch wrapper. Every mutating call carries X-SA (server-side CSRF check).
export class ApiError extends Error {
  constructor(message, code, status) {
    super(message)
    this.code = code
    this.status = status
  }
}

let onUnauthorized = null
export function setUnauthorizedHandler(fn) { onUnauthorized = fn }

export async function api(method, url, body, extraHeaders = {}) {
  const opt = { method, headers: { 'X-SA': '1', ...extraHeaders }, credentials: 'same-origin' }
  if (body instanceof FormData) opt.body = body
  else if (body !== undefined) {
    opt.headers['Content-Type'] = 'application/json'
    opt.body = JSON.stringify(body)
  }
  let r
  try {
    r = await fetch(url, opt)
  } catch {
    throw new ApiError('Không kết nối được máy chủ', 'network', 0)
  }
  let data = null
  const ct = r.headers.get('content-type') || ''
  if (ct.includes('json')) data = await r.json().catch(() => null)
  if (!r.ok) {
    if (r.status === 401 && onUnauthorized && !url.includes('/login')) onUnauthorized()
    throw new ApiError(data?.message || r.statusText, data?.error, r.status)
  }
  return data
}

export const get = (u) => api('GET', u)
export const post = (u, b = {}) => api('POST', u, b)
export const put = (u, b) => api('PUT', u, b)
export const del = (u) => api('DELETE', u)
