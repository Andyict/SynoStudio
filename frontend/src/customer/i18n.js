import { ref } from 'vue'

const dict = {
  vi: {
    login_title: 'Xem album của bạn',
    phone: 'Số điện thoại',
    password: 'Mật khẩu',
    login: 'Đăng nhập',
    login_hint: 'Dùng số điện thoại và mật khẩu studio đã gửi cho bạn.',
    logout: 'Đăng xuất',
    my_albums: 'Album của bạn',
    no_albums: 'Bạn chưa có album nào. Studio sẽ gửi album khi ảnh sẵn sàng.',
    photos: '{n} ảnh',
    expired: 'Đã hết hạn',
    expires_on: 'Hết hạn {d}',
    tab_all: 'Tất cả',
    tab_fav: 'Yêu thích',
    tab_edit: 'Chọn edit',
    tab_final: 'Ảnh hoàn thiện',
    unseen: 'Chưa xem',
    status_selecting: 'Đang chọn ảnh',
    status_submitted: 'Đã gửi studio',
    status_editing: 'Studio đang chỉnh sửa',
    status_delivered: 'Đã trả ảnh',
    heart: 'Yêu thích',
    pick_edit: 'Chọn edit',
    picked_edit: 'Đã chọn edit',
    comment: 'Ghi chú',
    download: 'Tải xuống',
    download_photo: 'Tải ảnh này',
    download_fav: 'Tải ảnh yêu thích ({n})',
    download_edit: 'Tải ảnh chọn edit ({n})',
    download_all: 'Tải tất cả ({n})',
    download_final: 'Tải ảnh hoàn thiện ({n})',
    download_off: 'Studio chưa mở quyền tải ảnh',
    download_started: 'Đang chuẩn bị file tải xuống…',
    chat: 'Trao đổi với studio',
    chat_ph: 'Nhắn cho studio…',
    chat_empty: 'Bạn có thể nhắn cho studio tại đây',
    photo_note_ph: 'Ghi chú cho ảnh này (vd: làm gọn mặt, xóa người phía sau…)',
    photo_note_empty: 'Chưa có ghi chú cho ảnh này',
    general_note: 'Ghi chú chung',
    general_note_hint: 'Yêu cầu chung cho cả album (tông màu, phong cách chỉnh sửa…)',
    save: 'Lưu',
    saved: 'Đã lưu',
    cancel: 'Hủy',
    edit_counter: 'Đã chọn {x}/{n} ảnh edit',
    edit_counter_free: 'Đã chọn {x} ảnh edit',
    submit: 'Gửi studio',
    submit_confirm_title: 'Gửi lựa chọn cho studio?',
    submit_confirm: 'Bạn đã chọn {x} ảnh để chỉnh sửa. Sau khi gửi, bạn sẽ không thay đổi được lựa chọn (trừ khi studio mở lại).',
    submit_warn_less: 'Bạn mới chọn {x}/{n} ảnh. Bạn vẫn có thể gửi ngay.',
    submitted_ok: 'Đã gửi lựa chọn cho studio. Cảm ơn bạn!',
    locked_banner: 'Bạn đã gửi lựa chọn cho studio. Muốn thay đổi, hãy nhắn cho studio.',
    fav_hint: 'Thả tim thoải mái để lọc dần. Sau đó vào tab “Yêu thích” để chọn ảnh edit.',
    edit_hint: 'Chọn tối đa {n} ảnh để studio chỉnh sửa. Bấm biểu tượng ✎ trên ảnh trong tab Yêu thích.',
    edit_hint_free: 'Chọn ảnh để studio chỉnh sửa từ danh sách Yêu thích.',
    empty_fav: 'Bạn chưa thả tim ảnh nào. Bấm ♥ trên ảnh để thêm vào Yêu thích.',
    empty_edit: 'Chưa chọn ảnh edit nào. Vào tab Yêu thích và bấm ✎ trên ảnh muốn chỉnh sửa.',
    empty_all: 'Album chưa có ảnh.',
    empty_unseen: 'Bạn đã xem hết ảnh.',
    final_ready: 'Ảnh hoàn thiện đã sẵn sàng — xem ở tab “Ảnh hoàn thiện”.',
    limit_reached: 'Bạn đã chọn đủ {n} ảnh edit. Bỏ chọn ảnh khác để chọn ảnh này.',
    language: 'English',
    change_password: 'Đổi mật khẩu',
    old_password: 'Mật khẩu hiện tại',
    new_password: 'Mật khẩu mới',
    password_changed: 'Đã đổi mật khẩu',
    back: 'Quay lại',
    contact: 'Liên hệ studio: {p}',
    guest_title: 'Album chia sẻ',
    guest_invalid: 'Link không còn hiệu lực.',
    album_expired: 'Album đã hết hạn truy cập. Vui lòng liên hệ studio.',
    retry: 'Thử lại',
    select_mode: 'Chọn nhiều',
    selected_n: 'Đã chọn {n}',
    download_selected: 'Tải ảnh đã chọn',
    install: 'Cài lên màn hình chính',
  },
  en: {
    login_title: 'View your album',
    phone: 'Phone number',
    password: 'Password',
    login: 'Sign in',
    login_hint: 'Use the phone number and password the studio sent you.',
    logout: 'Sign out',
    my_albums: 'Your albums',
    no_albums: 'No albums yet. The studio will share your album when photos are ready.',
    photos: '{n} photos',
    expired: 'Expired',
    expires_on: 'Expires {d}',
    tab_all: 'All',
    tab_fav: 'Favorites',
    tab_edit: 'For editing',
    tab_final: 'Finished',
    unseen: 'Unseen',
    status_selecting: 'Selecting',
    status_submitted: 'Sent to studio',
    status_editing: 'Studio is editing',
    status_delivered: 'Delivered',
    heart: 'Favorite',
    pick_edit: 'Pick for editing',
    picked_edit: 'Picked for editing',
    comment: 'Note',
    download: 'Download',
    download_photo: 'Download this photo',
    download_fav: 'Download favorites ({n})',
    download_edit: 'Download editing picks ({n})',
    download_all: 'Download all ({n})',
    download_final: 'Download finished photos ({n})',
    download_off: 'Downloads are not enabled yet',
    download_started: 'Preparing your download…',
    chat: 'Chat with the studio',
    chat_ph: 'Message the studio…',
    chat_empty: 'You can message the studio here',
    photo_note_ph: 'Note for this photo (e.g. slim face, remove person behind…)',
    photo_note_empty: 'No notes for this photo yet',
    general_note: 'General note',
    general_note_hint: 'Requests for the whole album (colour tone, editing style…)',
    save: 'Save',
    saved: 'Saved',
    cancel: 'Cancel',
    edit_counter: '{x}/{n} picked for editing',
    edit_counter_free: '{x} picked for editing',
    submit: 'Send to studio',
    submit_confirm_title: 'Send your selection?',
    submit_confirm: 'You picked {x} photos for editing. After sending you cannot change your selection (unless the studio reopens it).',
    submit_warn_less: 'You have picked {x}/{n}. You can still send now.',
    submitted_ok: 'Selection sent. Thank you!',
    locked_banner: 'You have sent your selection. Message the studio if you need changes.',
    fav_hint: 'Heart as many photos as you like, then pick photos for editing in “Favorites”.',
    edit_hint: 'Pick up to {n} photos for editing. Tap ✎ on a photo in Favorites.',
    edit_hint_free: 'Pick photos for editing from your Favorites.',
    empty_fav: 'No favorites yet. Tap ♥ on a photo to add it.',
    empty_edit: 'Nothing picked yet. Go to Favorites and tap ✎ on the photos to edit.',
    empty_all: 'This album has no photos yet.',
    empty_unseen: 'You have seen every photo.',
    final_ready: 'Your finished photos are ready — see the “Finished” tab.',
    limit_reached: 'You already picked {n} photos. Unpick another one first.',
    language: 'Tiếng Việt',
    change_password: 'Change password',
    old_password: 'Current password',
    new_password: 'New password',
    password_changed: 'Password changed',
    back: 'Back',
    contact: 'Studio contact: {p}',
    guest_title: 'Shared album',
    guest_invalid: 'This link is no longer valid.',
    album_expired: 'This album has expired. Please contact the studio.',
    retry: 'Retry',
    select_mode: 'Select',
    selected_n: '{n} selected',
    download_selected: 'Download selected',
    install: 'Add to home screen',
  },
}

function initialLang() {
  try {
    const s = localStorage.getItem('sa_lang')
    if (s === 'vi' || s === 'en') return s
  } catch { /* storage unavailable */ }
  return navigator.language?.startsWith('vi') ? 'vi' : 'vi'
}

export const lang = ref(initialLang())

export function setLang(l) {
  lang.value = l
  document.documentElement.lang = l
  try { localStorage.setItem('sa_lang', l) } catch { /* ignore */ }
}

export function t(key, params = {}) {
  let s = dict[lang.value]?.[key] ?? dict.vi[key] ?? key
  for (const [k, v] of Object.entries(params)) s = s.replaceAll(`{${k}}`, v)
  return s
}

// Server error codes -> localized messages for the customer UI
export function errText(e) {
  if (lang.value === 'en') {
    const m = { invalid_login: 'Wrong phone number or password', too_many: 'Too many attempts, try again in 15 minutes',
      locked: 'Your selection was already sent', expired: 'This album has expired', network: 'Cannot reach the server',
      empty: 'Nothing selected yet' }
    if (m[e?.code]) return m[e.code]
    if (e?.code === 'limit') return e.message
  }
  return e?.message || String(e)
}

export function hasLangPref() {
  try { return !!localStorage.getItem('sa_lang') } catch { return false }
}
