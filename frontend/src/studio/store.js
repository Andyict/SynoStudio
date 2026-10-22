import { reactive } from 'vue'
import { get } from '../shared/api.js'
import { connectWS } from '../shared/ws.js'

export const state = reactive({
  me: null,
  branding: null,
  notif: { items: [], unseen: 0 },
  unreadAlbums: 0,
})

export const can = (perm) => !!state.me?.perms?.[perm]

export const statusLabel = {
  selecting: 'Khách đang chọn',
  submitted: 'Khách đã gửi',
  editing: 'Đang chỉnh sửa',
  delivered: 'Đã trả ảnh',
}

export const permLabels = {
  'customers.manage': ['Quản lý khách hàng', 'Tạo/sửa/xóa khách, cấp lại mật khẩu'],
  'albums.view_all': ['Xem tất cả album', 'Nếu tắt: chỉ thấy album được gán làm người edit'],
  'albums.manage': ['Quản lý album', 'Tạo/sửa/xóa album, cài đặt, link chia sẻ'],
  'albums.status': ['Đổi trạng thái album', 'Mở khóa lựa chọn cho khách, chuyển trạng thái bất kỳ'],
  chat: ['Chat với khách', 'Trả lời chat và ghi chú trên ảnh'],
  'edit.work': ['Làm edit', 'Đánh dấu ảnh đã edit, chuyển album sang Đang chỉnh sửa / Đã trả ảnh'],
  'staff.manage': ['Quản lý nhân viên', 'Tài khoản nhân viên và nhóm quyền'],
  'settings.manage': ['Cài đặt hệ thống', 'Thương hiệu, logo, cấu hình chung'],
}

// Global realtime feed for staff; pages can subscribe for album events.
const listeners = new Set()
export function onStaffEvent(fn) {
  listeners.add(fn)
  return () => listeners.delete(fn)
}

let closeWS = null
export function startRealtime() {
  if (closeWS) return
  closeWS = connectWS('as=staff', (ev) => {
    if (ev.type === 'notification') {
      state.notif.items.unshift({ ...ev, new: true })
      state.notif.unseen++
      if (state.notif.items.length > 50) state.notif.items.pop()
      ping()
    }
    listeners.forEach((fn) => fn(ev))
  }, loadNotifications)
}
export function stopRealtime() {
  closeWS && closeWS()
  closeWS = null
}

export async function loadNotifications() {
  try { state.notif = await get('/api/s/notifications') } catch { /* ignore */ }
}

// Short beep for new notifications (only after the user interacted with the page).
let actx = null
function ping() {
  try {
    actx = actx || new AudioContext()
    const o = actx.createOscillator(), g = actx.createGain()
    o.frequency.value = 880
    g.gain.setValueAtTime(0.08, actx.currentTime)
    g.gain.exponentialRampToValueAtTime(0.0001, actx.currentTime + 0.25)
    o.connect(g).connect(actx.destination)
    o.start()
    o.stop(actx.currentTime + 0.25)
  } catch { /* ignore */ }
  if (document.hidden && 'Notification' in window && Notification.permission === 'granted') {
    const n = state.notif.items[0]
    try { new Notification('Studio Album', { body: n?.text || '' }) } catch { /* ignore */ }
  }
}

export async function loadMe() {
  state.me = await get('/api/s/me')
  startRealtime()
  loadNotifications()
}
