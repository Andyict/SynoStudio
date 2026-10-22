<script setup>
import { ref, computed, onMounted } from 'vue'
import Icon from '../shared/Icon.vue'
import Toasts from '../shared/Toasts.vue'
import { toast, toastErr } from '../shared/ui.js'
import { fmtBytes, fmtDate } from '../shared/fmt.js'
import { MODE, call, actionUrl } from './consoleApi.js'

const user = ref(null)
const authErr = ref('')
const tab = ref('overview')
const status = ref(null)
const shares = ref(null)
const runAs = ref('')
const owners = ref([])
const ownerForm = ref({ username: 'admin', password: '' })
const sys = ref({ public_url: '', preview_cache_gb: '20', use_syno_thumbs: '1', update_repo: '' })
const upd = ref(null)
const updBusy = ref(false)
const logs = ref([])
const login = ref({ username: '', password: '', otp: '' })
const busy = ref(false)
const loc = window.location

// the app itself listens on plain HTTP inside the LAN (HTTPS comes from the DSM reverse proxy)
const appUrl = computed(() => `http://${location.hostname}:${status.value?.port || 8686}`)

async function init() {
  authErr.value = ''
  try {
    const w = await call('whoami')
    user.value = w.user
    refresh()
  } catch (e) {
    authErr.value = e.message
    user.value = null
  }
}
async function refresh() {
  try {
    const [st, sh, ow] = await Promise.all([call('status'), call('shares'), call('owners')])
    status.value = st
    shares.value = sh.shares
    runAs.value = sh.run_as
    owners.value = ow
    sys.value = { public_url: st.public_url, preview_cache_gb: st.preview_cache_gb, use_syno_thumbs: st.use_syno_thumbs, update_repo: st.update_repo }
  } catch (e) { toastErr(e) }
}
async function doLogin() {
  busy.value = true
  authErr.value = ''
  try {
    await call('login', login.value)
    await init()
  } catch (e) { authErr.value = e.message } finally { busy.value = false }
}
async function toggleShare(s) {
  try {
    await call('share', { name: s.name, enabled: !s.enabled })
    s.enabled = !s.enabled
    toast(s.enabled ? `Đã bật ${s.name}` : `Đã tắt ${s.name}`)
  } catch (e) { toastErr(e) }
}
async function saveOwner() {
  try {
    await call('owner', ownerForm.value)
    toast('Đã lưu tài khoản chủ')
    ownerForm.value.password = ''
    owners.value = await call('owners')
  } catch (e) { toastErr(e) }
}
async function saveSys() {
  try { await call('settings', sys.value); toast('Đã lưu') } catch (e) { toastErr(e) }
}
async function clearCache(what) {
  if (!confirm(what === 'all' ? 'Xóa toàn bộ thumbnail và ảnh xem trước? Ứng dụng sẽ tạo lại khi cần.' : 'Xóa ảnh xem trước (preview)?')) return
  try { await call('clear-cache', { what }); toast('Đã dọn cache'); refresh() } catch (e) { toastErr(e) }
}
async function loadLogs() {
  try { logs.value = (await call('logs')).lines } catch (e) { toastErr(e) }
}
async function logout() {
  await call('logout', {}).catch(() => {})
  user.value = null
}
async function checkUpdate(force) {
  updBusy.value = true
  try { upd.value = await call('update' + (force ? '&force=1' : '')) } catch (e) { toastErr(e) } finally { updBusy.value = false }
}
function openTab(t) {
  tab.value = t
  if (t === 'logs') loadLogs()
  if (t === 'update' && !upd.value) checkUpdate(false)
}
onMounted(init)
</script>

<template>
  <div class="console">
    <header class="chead">
      <span class="mark"><Icon name="shield" /></span>
      <div class="grow">
        <h1>Studio Album Console</h1>
        <div class="small muted">Quản trị ứng dụng trên NAS{{ user ? ' · ' + user : '' }}</div>
      </div>
      <a v-if="user" class="btn" :href="appUrl + '/studio/'" target="_blank"><Icon name="images" :size="16" />Mở Studio Album</a>
      <button v-if="user && MODE === 'server'" class="btn ghost" @click="logout"><Icon name="logout" :size="16" /></button>
    </header>

    <!-- auth -->
    <div v-if="!user" class="cbody">
      <div v-if="MODE === 'dsm'" class="card stack" style="max-width:560px">
        <div v-if="authErr" class="alert err">{{ authErr }}</div>
        <div v-else class="empty"><div class="spinner"></div></div>
        <p class="small" style="margin:0">Nếu Console trong DSM không mở được, dùng Console qua cổng ứng dụng (chỉ trong mạng LAN):
          <a :href="`http://${loc.hostname}:8686/console/`" target="_blank">{{ loc.hostname }}:8686/console</a></p>
      </div>
      <form v-else class="card stack" style="max-width:400px" @submit.prevent="doLogin">
        <h2>Đăng nhập quản trị DSM</h2>
        <p class="hint" style="margin:0">Dùng tài khoản thuộc nhóm <b>administrators</b> của NAS. Console chỉ mở được trong mạng nội bộ.</p>
        <div class="field"><label>Tài khoản DSM</label><input class="input" v-model="login.username" autocomplete="username" /></div>
        <div class="field"><label>Mật khẩu</label><input class="input" type="password" v-model="login.password" autocomplete="current-password" /></div>
        <div class="field"><label>Mã OTP (nếu bật xác minh 2 bước)</label><input class="input" v-model="login.otp" inputmode="numeric" /></div>
        <div v-if="authErr && authErr !== 'Đăng nhập bằng tài khoản quản trị DSM'" class="alert err">{{ authErr }}</div>
        <button class="btn primary" :disabled="busy">Đăng nhập</button>
      </form>
    </div>

    <template v-else>
      <nav class="tabs-c">
        <button :class="{ on: tab === 'overview' }" @click="openTab('overview')"><Icon name="home" :size="16" />Tổng quan</button>
        <button :class="{ on: tab === 'shares' }" @click="openTab('shares')"><Icon name="folder" :size="16" />Share folder</button>
        <button :class="{ on: tab === 'owner' }" @click="openTab('owner')"><Icon name="key" :size="16" />Tài khoản quản trị</button>
        <button :class="{ on: tab === 'settings' }" @click="openTab('settings')"><Icon name="settings" :size="16" />Cài đặt & tên miền</button>
        <button :class="{ on: tab === 'update' }" @click="openTab('update')"><Icon name="download" :size="16" />Cập nhật</button>
        <button :class="{ on: tab === 'logs' }" @click="openTab('logs')"><Icon name="file" :size="16" />Nhật ký</button>
      </nav>
      <main class="cbody">
        <!-- overview -->
        <template v-if="tab === 'overview' && status">
          <div v-if="!owners.length" class="alert warn"><Icon name="info" :size="18" /><div>Chưa có tài khoản quản trị cho ứng dụng. <a href="#" @click.prevent="openTab('owner')">Tạo tài khoản chủ</a> để nhân viên đăng nhập.</div></div>
          <div v-if="shares && !shares.some((s) => s.enabled)" class="alert warn"><Icon name="info" :size="18" /><div>Chưa bật share folder nào. <a href="#" @click.prevent="openTab('shares')">Bật share folder</a> chứa ảnh album.</div></div>
          <div class="cgrid">
            <div class="card"><div class="label">Dịch vụ</div>
              <div class="big"><span class="badge" :class="status.running ? 'delivered' : 'danger'">{{ status.running ? 'Đang chạy' : 'Không phản hồi' }}</span></div>
              <div class="small muted">Cổng {{ status.port }} · v{{ status.version }}</div></div>
            <div class="card"><div class="label">Album</div><div class="big">{{ status.counts.albums }}</div><div class="small muted">{{ status.counts.photos }} ảnh</div></div>
            <div class="card"><div class="label">Khách hàng</div><div class="big">{{ status.counts.customers }}</div><div class="small muted">{{ status.counts.staff }} nhân viên</div></div>
            <div class="card"><div class="label">Cache ảnh</div><div class="big">{{ fmtBytes(status.cache.thumbs + status.cache.previews) }}</div>
              <div class="small muted">Thumbnail {{ fmtBytes(status.cache.thumbs) }} · Preview {{ fmtBytes(status.cache.previews) }}<template v-if="status.thumbs_pending"> · đang tạo {{ status.thumbs_pending }}</template></div></div>
          </div>
          <div class="card stack">
            <div class="kv">
              <div>Thư mục dữ liệu</div><div class="mono">{{ status.data_dir }}</div>
              <div>Cơ sở dữ liệu</div><div>{{ fmtBytes(status.db_size) }}</div>
              <div>Chạy với user</div><div class="mono">{{ runAs }}</div>
              <div>Địa chỉ nội bộ</div><div><a :href="appUrl" target="_blank">{{ appUrl }}</a></div>
              <div>Địa chỉ public</div><div>{{ status.public_url || 'Chưa cài' }}</div>
            </div>
            <div class="row wrap">
              <a class="btn" :href="actionUrl('backup')"><Icon name="database" :size="16" />Tải bản sao lưu CSDL</a>
              <button class="btn" @click="clearCache('previews')"><Icon name="trash" :size="16" />Dọn ảnh preview</button>
              <button class="btn danger" @click="clearCache('all')"><Icon name="trash" :size="16" />Xóa toàn bộ cache</button>
              <button class="btn ghost" @click="refresh"><Icon name="refresh" :size="16" />Làm mới</button>
            </div>
            <p class="hint" style="margin:0">Nên sao lưu thư mục dữ liệu bằng Hyper Backup (chọn ứng dụng Studio Album hoặc thư mục trên).</p>
          </div>
        </template>

        <!-- shares -->
        <template v-if="tab === 'shares'">
          <div class="card pad0 table-wrap">
            <table class="table">
              <thead><tr><th>Share folder</th><th>Quyền đọc của ứng dụng</th><th style="width:150px">Dùng cho album</th></tr></thead>
              <tbody>
                <tr v-for="s in shares" :key="s.name">
                  <td><b>{{ s.name }}</b><div class="small muted mono">{{ s.path }}</div></td>
                  <td>
                    <span v-if="!s.known" class="badge danger">Không còn tồn tại</span>
                    <span v-else-if="s.readable" class="badge delivered"><Icon name="check" :size="12" />Có quyền</span>
                    <span v-else class="badge danger">Chưa cấp quyền</span>
                  </td>
                  <td><label class="check"><span class="switch"><input type="checkbox" :checked="s.enabled" @change="toggleShare(s)" /><span></span></span>{{ s.enabled ? 'Bật' : 'Tắt' }}</label></td>
                </tr>
              </tbody>
            </table>
          </div>
          <div class="card stack">
            <h3>Cấp quyền share folder cho ứng dụng</h3>
            <p class="small" style="margin:0">DSM 7 chạy ứng dụng bằng user hệ thống <b class="mono">{{ runAs }}</b>. Với share folder báo “Chưa cấp quyền”:</p>
            <ol class="small" style="margin:0;padding-left:20px;line-height:1.8">
              <li>Mở <b>Control Panel → Shared Folder</b>, chọn share folder, bấm <b>Edit</b>.</li>
              <li>Tab <b>Permissions</b>, ô chọn phía trên đổi sang <b>System internal user</b>.</li>
              <li>Tìm <b class="mono">{{ runAs }}</b>, tick <b>Read only</b> (đủ để xem/tải ảnh) hoặc <b>Read/Write</b>, bấm Save.</li>
              <li>Quay lại đây bấm <b>Làm mới</b>, rồi bật “Dùng cho album”.</li>
            </ol>
            <p class="hint" style="margin:0">Share folder chọn lúc cài đặt đã được cấp quyền tự động. Chỉ những share folder được bật mới hiển thị khi nhân viên tạo album.</p>
            <button class="btn" style="align-self:flex-start" @click="refresh"><Icon name="refresh" :size="16" />Làm mới</button>
          </div>
        </template>

        <!-- owner -->
        <template v-if="tab === 'owner'">
          <div class="card stack" style="max-width:560px">
            <h3>Tài khoản chủ (toàn quyền)</h3>
            <p class="small muted" style="margin:0">Tài khoản chủ đăng nhập vào Studio Album tại <a :href="appUrl + '/studio/'" target="_blank">{{ appUrl }}/studio</a>, sau đó tạo tài khoản nhân viên và phân quyền ngay trong ứng dụng. Dùng form này để tạo mới hoặc đặt lại mật khẩu khi quên.</p>
            <div v-if="owners.length" class="row wrap"><span v-for="o in owners" :key="o.id" class="tag"><Icon name="user" :size="14" />{{ o.username }}<span class="muted small" v-if="o.last_login">· {{ fmtDate(o.last_login, true) }}</span></span></div>
            <div class="field"><label>Tên đăng nhập</label><input class="input" v-model="ownerForm.username" /></div>
            <div class="field"><label>Mật khẩu mới (≥ 6 ký tự)</label><input class="input" type="password" v-model="ownerForm.password" autocomplete="new-password" /></div>
            <button class="btn primary" style="align-self:flex-start" @click="saveOwner">Lưu tài khoản chủ</button>
          </div>
        </template>

        <!-- settings -->
        <template v-if="tab === 'settings'">
          <div class="card stack" style="max-width:760px">
            <div class="field"><label>Địa chỉ public (tên miền khách truy cập)</label><input class="input mono" v-model="sys.public_url" placeholder="https://album.tenstudio.vn" /></div>
            <div class="grid2">
              <div class="field"><label>Giới hạn cache ảnh preview (GB)</label><input class="input" type="number" min="0" step="1" v-model="sys.preview_cache_gb" />
                <span class="hint">Preview (2048px) tạo khi khách mở ảnh; quá giới hạn thì xóa ảnh ít xem nhất. 0 = không giới hạn.</span></div>
              <div class="field"><label>Dùng thumbnail của Synology Photos</label>
                <label class="check" style="height:40px"><span class="switch"><input type="checkbox" :checked="sys.use_syno_thumbs === '1'" @change="sys.use_syno_thumbs = $event.target.checked ? '1' : '0'" /><span></span></span>Nhanh hơn nếu thư mục đã được Synology Photos index</label></div>
            </div>
            <div class="field"><label>Kho GitHub chứa bản phát hành (owner/repo)</label><input class="input mono" v-model="sys.update_repo" />
              <span class="hint">Để trống = dùng kho mặc định của bản build.</span></div>
            <button class="btn primary" style="align-self:flex-start" @click="saveSys">Lưu</button>
          </div>
          <div class="card stack" style="max-width:760px">
            <h3>Cấu hình tên miền (Reverse Proxy + SSL)</h3>
            <ol class="small" style="margin:0;padding-left:20px;line-height:1.8">
              <li>Trỏ bản ghi DNS (A/CNAME) của <span class="mono">album.tenstudio.vn</span> về IP public của NAS; mở cổng 443 trên router về NAS.</li>
              <li><b>Control Panel → Login Portal → Advanced → Reverse Proxy → Create</b>:<br />
                Source: <span class="mono">HTTPS · album.tenstudio.vn · 443</span> → Destination: <span class="mono">HTTP · localhost · {{ status?.port || 8686 }}</span>.</li>
              <li>Trong rule vừa tạo: tab <b>Custom Header → Create → WebSocket</b> (bắt buộc để chat realtime hoạt động).</li>
              <li><b>Control Panel → Security → Certificate → Add → Let's Encrypt</b> cho tên miền, rồi <b>Settings</b> gán chứng chỉ cho rule reverse proxy.</li>
              <li>Điền địa chỉ <span class="mono">https://album.tenstudio.vn</span> vào ô “Địa chỉ public” phía trên.</li>
            </ol>
            <p class="hint" style="margin:0">Không cần mở cổng {{ status?.port || 8686 }} ra Internet. Console chỉ truy cập được trong LAN.</p>
          </div>
        </template>

        <!-- update -->
        <template v-if="tab === 'update'">
          <div class="card stack" style="max-width:760px">
            <div v-if="!upd" class="empty"><div class="spinner"></div></div>
            <template v-else>
              <div class="kv">
                <div>Phiên bản đang chạy</div><div class="mono">{{ upd.current }}</div>
                <div>Bản mới nhất trên GitHub</div><div class="mono">{{ upd.latest || '—' }} <span v-if="upd.has_update" class="badge submitted">Có bản mới</span><span v-else-if="upd.latest" class="badge delivered">Đang dùng bản mới nhất</span></div>
                <div>Kho phát hành</div><div class="mono"><a :href="'https://github.com/' + upd.repo + '/releases'" target="_blank">{{ upd.repo || 'chưa cấu hình' }}</a></div>
              </div>
              <div v-if="upd.error" class="alert err">Không kiểm tra được: {{ upd.error }}</div>
              <pre v-if="upd.changelog" class="logs" style="max-height:200px">{{ upd.changelog }}</pre>
              <button class="btn" style="align-self:flex-start" :disabled="updBusy" @click="checkUpdate(true)"><Icon name="refresh" :size="16" />Kiểm tra lại</button>
            </template>
          </div>
          <div class="card stack" style="max-width:760px">
            <h3>Cập nhật bằng Package Center (làm 1 lần)</h3>
            <ol class="small" style="margin:0;padding-left:20px;line-height:1.8">
              <li>Mở <b>Package Center → Settings → Package Sources → Add</b>.</li>
              <li>Name: <span class="mono">Studio Album</span> · Location: <span class="mono">{{ upd?.source_url || 'http://127.0.0.1:8686/pkg-source' }}</span></li>
              <li>Từ nay khi có bản mới, Package Center hiện nút <b>Update</b> cho Studio Album (tab Installed hoặc Community). Dữ liệu album, khách, tin nhắn được giữ nguyên.</li>
            </ol>
            <p class="hint" style="margin:0">Nếu một bản cập nhật lỗi khiến ứng dụng không chạy: tải file .spk bản trước trong trang Releases trên GitHub và cài bằng <b>Manual Install</b>.</p>
          </div>
        </template>

        <!-- logs -->
        <template v-if="tab === 'logs'">
          <div class="row"><span class="spacer"></span><button class="btn sm" @click="loadLogs"><Icon name="refresh" :size="15" />Tải lại</button></div>
          <pre class="logs">{{ logs.join('\n') || 'Chưa có nhật ký' }}</pre>
        </template>
      </main>
    </template>
    <Toasts />
  </div>
</template>

<style>
body { background: #f3f4f6; }
.console { min-height: 100vh; }
.chead { display: flex; align-items: center; gap: 12px; padding: 16px 20px; background: #fff; border-bottom: 1px solid var(--border); flex-wrap: wrap; }
.chead .mark { width: 42px; height: 42px; border-radius: 12px; background: #2f6fda; color: #fff; display: flex; align-items: center; justify-content: center; }
.chead h1 { font-size: 19px; }
.tabs-c { display: flex; gap: 4px; padding: 0 16px; background: #fff; border-bottom: 1px solid var(--border); overflow-x: auto; }
.tabs-c button { border: 0; background: none; font: inherit; font-weight: 600; color: var(--muted); padding: 12px 14px; border-bottom: 2px solid transparent; cursor: pointer; white-space: nowrap; display: inline-flex; gap: 6px; align-items: center; }
.tabs-c button.on { color: #2f6fda; border-bottom-color: #2f6fda; }
.cbody { padding: 20px; max-width: 1100px; margin: 0 auto; display: flex; flex-direction: column; gap: 16px; }
.cgrid { display: grid; grid-template-columns: repeat(auto-fill, minmax(220px, 1fr)); gap: 12px; }
.cgrid .big { font-size: 26px; font-weight: 700; margin: 4px 0; }
.kv { display: grid; grid-template-columns: 170px 1fr; gap: 8px 14px; font-size: 14px; }
.kv > div:nth-child(odd) { color: var(--muted); }
.tag { display: inline-flex; align-items: center; gap: 4px; padding: 3px 8px; border-radius: 8px; background: #eef1f5; font-size: 13px; }
.logs { background: #1e1f22; color: #d6d6d6; padding: 14px; border-radius: 10px; font-size: 12px; max-height: 65vh; overflow: auto; white-space: pre-wrap; margin: 0; }
:root { --brand: #2f6fda; }
</style>
