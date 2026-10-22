<script setup>
import { ref, onMounted } from 'vue'
import { get, put, api, del } from '../shared/api.js'
import { toast, toastErr, applyBranding } from '../shared/ui.js'
import Icon from '../shared/Icon.vue'
import { state, can } from './store.js'

const s = ref(null)
const hasLogo = ref(false)
const version = ref('')
const logoKey = ref(Date.now())

onMounted(async () => {
  try {
    const r = await get('/api/s/settings')
    s.value = r.settings
    hasLogo.value = r.logo
    version.value = r.version
  } catch (e) { toastErr(e) }
})

async function save() {
  try {
    await put('/api/s/settings', s.value)
    const b = await get('/api/public/branding')
    state.branding = b
    applyBranding(b)
    toast('Đã lưu cài đặt')
  } catch (e) { toastErr(e) }
}
async function uploadLogo(e) {
  const file = e.target.files[0]
  if (!file) return
  const fd = new FormData()
  fd.append('logo', file)
  try {
    await api('POST', '/api/s/settings/logo', fd)
    hasLogo.value = true
    logoKey.value = Date.now()
    if (state.branding) state.branding.logo = true
    toast('Đã cập nhật logo')
  } catch (err) { toastErr(err) }
  e.target.value = ''
}
async function removeLogo() {
  try { await del('/api/s/settings/logo'); hasLogo.value = false; if (state.branding) state.branding.logo = false } catch (e) { toastErr(e) }
}
</script>

<template>
  <div class="page" style="max-width:860px">
    <div class="page-head"><h1>Cài đặt</h1></div>
    <div v-if="!s" class="empty"><div class="spinner"></div></div>
    <template v-else>
      <div v-if="!can('settings.manage')" class="alert warn">Bạn chỉ có quyền xem. Liên hệ quản trị để thay đổi.</div>
      <fieldset :disabled="!can('settings.manage')" class="card stack" style="border:1px solid var(--border)">
        <h2>Thương hiệu</h2>
        <div class="row wrap" style="gap:16px">
          <div class="logo-box">
            <img v-if="hasLogo" :src="`/branding/logo?k=${logoKey}`" alt="logo" />
            <span v-else class="muted small">Chưa có logo</span>
          </div>
          <div class="stack" style="gap:8px">
            <label class="btn sm"><Icon name="image" :size="15" />Tải logo lên<input type="file" accept="image/png,image/jpeg,image/webp" hidden @change="uploadLogo" /></label>
            <button v-if="hasLogo" class="btn sm danger" @click="removeLogo">Xóa logo</button>
            <span class="hint">PNG nền trong suốt, cao ~200px, tối đa 3MB.</span>
          </div>
        </div>
        <div class="grid2">
          <div class="field"><label>Tên studio</label><input class="input" v-model="s.studio_name" /></div>
          <div class="field"><label>Màu chủ đạo</label>
            <div class="row"><input type="color" v-model="s.primary_color" style="width:46px;height:40px;border:0;background:none;padding:0" /><input class="input mono" v-model="s.primary_color" /></div>
          </div>
          <div class="field"><label>Số điện thoại liên hệ (hiện cho khách)</label><input class="input" v-model="s.contact_phone" /></div>
          <div class="field"><label>Ngôn ngữ mặc định cho khách</label>
            <select class="input" v-model="s.default_lang"><option value="vi">Tiếng Việt</option><option value="en">English</option></select>
          </div>
        </div>
        <div class="field"><label>Lời chào trên trang đăng nhập của khách</label><textarea class="input" rows="2" v-model="s.welcome_text" placeholder="VD: Cảm ơn bạn đã tin tưởng Studio. Hãy chọn những tấm ảnh bạn yêu thích nhé!"></textarea></div>

        <h2 style="margin-top:8px">Hệ thống</h2>
        <div class="field"><label>Địa chỉ public của ứng dụng</label><input class="input mono" v-model="s.public_url" placeholder="https://album.tenstudio.vn" />
          <span class="hint">Dùng để tạo link gửi khách và link xem công khai.</span></div>
        <div class="grid2">
          <div class="field"><label>Tên thư mục ảnh hoàn thiện</label><input class="input" v-model="s.final_folder" />
            <span class="hint">Thư mục con trong album chứa ảnh đã chỉnh sửa (không phân biệt hoa thường).</span></div>
        </div>
        <div class="row"><span class="spacer"></span><button class="btn primary" @click="save">Lưu cài đặt</button></div>
      </fieldset>
      <p class="small muted center">Studio Album v{{ version }} · Share folder, dung lượng cache và tài khoản chủ được quản lý trong Studio Album Console (DSM).</p>
    </template>
  </div>
</template>

<style scoped>
fieldset { margin: 0; min-width: 0; }
.logo-box { width: 180px; height: 90px; border: 1px dashed var(--border); border-radius: 10px; display: flex; align-items: center; justify-content: center; background: #faf8f6; }
.logo-box img { max-width: 160px; max-height: 76px; object-fit: contain; }
</style>
