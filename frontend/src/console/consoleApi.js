// Console API transport.
//  dsm:    page served by DSM (/webman/3rdparty/StudioAlbum/console/), API = api.cgi authenticated by the DSM session.
//  server: page served by the app on port 8686 (/console/), API = /api/console with a DSM admin login.
import { api } from '../shared/api.js'

/* global __CONSOLE_MODE__ */
export const MODE = typeof __CONSOLE_MODE__ !== 'undefined' ? __CONSOLE_MODE__ : 'server'
const CGI = '/webman/3rdparty/StudioAlbum/api.cgi'

let synoToken = ''
async function ensureToken() {
  if (synoToken) return synoToken
  const r = await fetch('/webman/login.cgi', { credentials: 'same-origin' })
  const j = await r.json().catch(() => ({}))
  synoToken = j.SynoToken || ''
  return synoToken
}

export function actionUrl(action) {
  return MODE === 'dsm'
    ? `${CGI}?action=${action}&SynoToken=${encodeURIComponent(synoToken)}`
    : `/api/console?action=${action}`
}

export async function call(action, body) {
  const method = body === undefined ? 'GET' : 'POST'
  if (MODE === 'dsm') {
    const tok = await ensureToken()
    return api(method, actionUrl(action), body, { 'X-SYNO-TOKEN': tok })
  }
  return api(method, actionUrl(action), body)
}
