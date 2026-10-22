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
// Inside the DSM window the title bar is drawn by DSM, so we drop our own header.
const embedded = window.self !== window.top

// the app itself listens on plain HTTP inside the LAN (HTTPS comes from the DSM reverse proxy)
const appUrl = computed(() => `http://${location.hostname}:${status.value?.port || 8686}`)

const nav = [
  { id: 'overview', icon: 'grid', label: 'Tổng quan' },
  { id: 'shares', icon: 'folder', label: 'Share folder' },
  { id: 'owner', icon: 'key', label: 'Tài khoản quản trị' },
  { id: 'settings', icon: 'settings', label: 'Cài đặt' },
  { id: 'update', icon: 'download', label: 'Cập nhật' },
  { id: 'logs', icon: 'menu', label: 'Nhật ký' },
]

// Health summary shown at the top of the overview, like Drive's "Server is healthy".
const problems = computed(() => {
  const p = []
  if (status.value && !status.value.running) p.push({ text: 'Dịch vụ Studio Album không phản hồi. Thử Stop/Run lại trong Package Center.', tab: 'logs' })
  if (!owners.value.length) p.push({ text: 'Chưa có tài khoản quản trị cho ứng dụng.', tab: 'owner' })
  if (shares.value && !shares.value.some((s) => s.enabled)) p.push({ text: 'Chưa bật share folder nào cho album.', tab: 'shares' })
  if (shares.value && shares.value.some((s) => s.enabled && !s.readable)) p.push({ text: 'Có share folder đã bật nhưng ứng dụng chưa có quyền đọc.', tab: 'shares' })
  return p
})

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
  if (t === 'overview' || t === 'shares') refresh()
}
onMounted(init)
</script>

<template>
  <div class="dsm" :class="{ embedded }">
    <header v-if="!embedded" class="dsm-title">
      <img :src="'/icons/icon-192.png'" alt="" />
      <span>Studio Album Console</span>
      <span class="spacer"></span>
      <template v-if="user">
        <span class="small muted">{{ user }}</span>
        <button v-if="MODE === 'server'" class="icon-btn" title="Đăng xuất" @click="logout"><Icon name="logout" :size="18" /></button>
      </template>
    </header>

    <!-- auth -->
    <div v-if="!user" class="dsm-auth">
      <div v-if="MODE === 'dsm'" class="dcard" style="max-width:560px;width:100%">
        <div class="dcard-b stack">
          <div v-if="authErr" class="alert err">{{ authErr }}</div>
          <div v-else class="empty"><div class="spinner"></div></div>
          <p class="small" style="margin:0">Nếu Console trong DSM không mở được, dùng Console qua cổng ứng dụng (chỉ trong mạng LAN):
            <a :href="`http://${loc.hostname}:8686/console/`" target="_blank">{{ loc.hostname }}:8686/console</a></p>
        </div>
      </div>
      <form v-else class="dcard" style="max-width:400px;width:100%" @submit.prevent="doLogin">
        <div class="dcard-h">Đăng nhập quản trị DSM</div>
        <div class="dcard-b stack">
          <p class="hint" style="margin:0">Dùng tài khoản thuộc nhóm <b>administrators</b> của NAS. Console chỉ mở được trong mạng nội bộ.</p>
          <div class="field"><label>Tài khoản DSM</label><input class="input" v-model="login.username" autocomplete="username" /></div>
          <div class="field"><label>Mật khẩu</label><input class="input" type="password" v-model="login.password" autocomplete="current-password" /></div>
          <div class="field"><label>Mã OTP (nếu bật xác minh 2 bước)</label><input class="input" v-model="login.otp" inputmode="numeric" /></div>
          <div v-if="authErr && authErr !== 'Đăng nhập bằng tài khoản quản trị DSM'" class="alert err">{{ authErr }}</div>
          <button class="btn primary" :disabled="busy">Đăng nhập</button>
        </div>
      </form>
    </div>

    <div v-else class="dsm-body">
      <aside class="dsm-nav">
        <button v-for="n in nav" :key="n.id" :class="{ on: tab === n.id }" @click="openTab(n.id)">
          <Icon :name="n.icon" :size="18" /><span>{{ n.label }}</span>
          <span v-if="n.id === 'overview' && problems.length" class="navdot"></span>
        </button>
      </aside>

      <main class="dsm-main">
        <!-- overview -->
        <template v-if="tab === 'overview'">
          <section class="dcard health">
            <div class="hicon" :class="problems.length ? 'warn' : 'ok'">
              <Icon :name="problems.length ? 'info' : 'check'" :size="30" />
            </div>
            <div class="grow">
              <h2>{{ !status ? 'Đang kiểm tra…' : problems.length ? 'Studio Album cần được cấu hình' : 'Studio Album đang hoạt động tốt' }}</h2>
              <div v-if="status && !problems.length" class="muted">Dịch vụ chạy ở cổng {{ status.port }} · phiên bản {{ status.version }}</div>
              <ul v-else-if="status" class="plist">
                <li v-for="p in problems" :key="p.text">{{ p.text }} <a href="#" @click.prevent="openTab(p.tab)">Xử lý</a></li>
              </ul>
            </div>
            <a class="btn" :href="appUrl + '/studio/'" target="_blank"><Icon name="images" :size="16" />Mở Studio Album</a>
          </section>

          <section v-if="status" class="dcard">
            <div class="dcard-h">Thống kê<span class="spacer"></span>
              <button class="icon-btn sm" title="Làm mới" @click="refresh"><Icon name="refresh" :size="16" /></button></div>
            <div class="stats4">
              <div class="big">
                <div class="n">{{ status.counts.albums }}</div><div class="muted">Album</div>
              </div>
              <div><Icon name="images" :size="26" class="si" /><b>Ảnh</b><span>{{ status.counts.photos }}</span></div>
              <div><Icon name="users" :size="26" class="si" /><b>Khách hàng</b><span>{{ status.counts.customers }}</span></div>
              <div><Icon name="user" :size="26" class="si" /><b>Nhân viên</b><span>{{ status.counts.staff }}</span></div>
            </div>
          </section>

          <section v-if="status" class="dcard">
            <div class="dcard-h">Thông tin hệ thống</div>
            <div class="dcard-b">
              <div class="kv">
                <div>Địa chỉ nội bộ</div><div><a :href="appUrl" target="_blank">{{ appUrl }}</a></div>
                <div>Địa chỉ public</div><div>{{ status.public_url || 'Chưa cài' }}</div>
                <div>Chạy với user</div><div class="mono">{{ runAs }}</div>
                <div>Thư mục dữ liệu</div><div class="mono">{{ status.data_dir }}</div>
                <div>Cơ sở dữ liệu</div><div>{{ fmtBytes(status.db_size) }}</div>
                <div>Cache ảnh</div><div>Thumbnail {{ fmtBytes(status.cache.thumbs) }} · Preview {{ fmtBytes(status.cache.previews) }}<template v-if="status.thumbs_pending"> · đang tạo {{ status.thumbs_pending }} ảnh</template></div>
              </div>
            </div>
          </section>

          <section v-if="status" class="dcard">
            <div class="dcard-h">Bảo trì</div>
            <div class="dcard-b stack">
              <div class="row wrap">
                <a class="btn" :href="actionUrl('backup')"><Icon name="database" :size="16" />Tải bản sao lưu CSDL</a>
                <button class="btn" @click="clearCache('previews')"><Icon name="trash" :size="16" />Dọn ảnh preview</button>
                <button class="btn danger" @click="clearCache('all')"><Icon name="trash" :size="16" />Xóa toàn bộ cache</button>
              </div>
              <p class="hint" style="margin:0">Nên sao lưu thư mục dữ liệu bằng Hyper Backup (chọn ứng dụng Studio Album hoặc thư mục dữ liệu ở trên).</p>
            </div>
          </section>
        </template>

        <!-- shares -->
        <template v-if="tab === 'shares'">
          <section class="dcard">
            <div class="dcard-h">Share folder dùng cho album<span class="spacer"></span>
              <button class="btn sm" @click="refresh"><Icon name="refresh" :size="15" />Làm mới</button></div>
            <div class="table-wrap">
              <table class="table">
                <thead><tr><th>Tên</th><th>Quyền đọc của ứng dụng</th><th style="width:150px">Dùng cho album</th></tr></thead>
                <tbody>
                  <tr v-for="s in shares" :key="s.name">
                    <td><div class="row"><Icon name="folder" :size="18" style="color:#d9a441" /><b>{{ s.name }}</b></div><div class="small muted mono">{{ s.path }}</div></td>
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
          </section>
          <section class="dcard">
            <div class="dcard-h">Cấp quyền share folder cho ứng dụng</div>
            <div class="dcard-b stack">
              <p class="small" style="margin:0">DSM 7 chạy ứng dụng bằng user hệ thống <b class="mono">{{ runAs }}</b>. Với share folder báo “Chưa cấp quyền”:</p>
              <ol class="small" style="margin:0;padding-left:20px;line-height:1.8">
                <li>Mở <b>Control Panel → Shared Folder</b>, chọn share folder, bấm <b>Edit</b>.</li>
                <li>Tab <b>Permissions</b>, ô chọn phía trên đổi sang <b>System internal user</b>.</li>
                <li>Tìm <b class="mono">{{ runAs }}</b>, tick <b>Read only</b> (đủ để xem/tải ảnh) hoặc <b>Read/Write</b>, bấm Save.</li>
                <li>Quay lại đây bấm <b>Làm mới</b>, rồi bật “Dùng cho album”.</li>
              </ol>
              <p class="hint" style="margin:0">Chỉ những share folder được bật mới hiển thị khi nhân viên tạo album.</p>
            </div>
          </section>
        </template>

        <!-- owner -->
        <template v-if="tab === 'owner'">
          <section class="dcard">
            <div class="dcard-h">Tài khoản chủ (toàn quyền)</div>
            <div class="dcard-b stack" style="max-width:560px">
              <p class="small muted" style="margin:0">Tài khoản chủ đăng nhập vào Studio Album tại <a :href="appUrl + '/studio/'" target="_blank">{{ appUrl }}/studio</a>, sau đó tạo tài khoản nhân viên và phân quyền ngay trong ứng dụng. Dùng form này để tạo mới hoặc đặt lại mật khẩu khi quên.</p>
              <div v-if="owners.length" class="row wrap"><span v-for="o in owners" :key="o.id" class="tag"><Icon name="user" :size="14" />{{ o.username }}<span class="muted small" v-if="o.last_login">· {{ fmtDate(o.last_login, true) }}</span></span></div>
              <div class="field"><label>Tên đăng nhập</label><input class="input" v-model="ownerForm.username" /></div>
              <div class="field"><label>Mật khẩu mới (≥ 6 ký tự)</label><input class="input" type="password" v-model="ownerForm.password" autocomplete="new-password" /></div>
              <button class="btn primary" style="align-self:flex-start" @click="saveOwner">Lưu tài khoản chủ</button>
            </div>
          </section>
        </template>

        <!-- settings -->
        <template v-if="tab === 'settings'">
          <section class="dcard">
            <div class="dcard-h">Chung</div>
            <div class="dcard-b stack" style="max-width:760px">
              <div class="field"><label>Địa chỉ public (tên miền khách truy cập)</label><input class="input mono" v-model="sys.public_url" placeholder="https://album.tenstudio.vn" /></div>
              <div class="grid2">
                <div class="field"><label>Giới hạn cache ảnh preview (GB)</label><input class="input" type="number" min="0" step="1" v-model="sys.preview_cache_gb" />
                  <span class="hint">Preview (2048px) tạo khi khách mở ảnh; quá giới hạn thì xóa ảnh ít xem nhất. 0 = không giới hạn.</span></div>
                <div class="field"><label>Dùng thumbnail của Synology Photos</label>
                  <label class="check" style="height:40px"><span class="switch"><input type="checkbox" :checked="sys.use_syno_thumbs === '1'" @change="sys.use_syno_thumbs = $event.target.checked ? '1' : '0'" /><span></span></span>Nhanh hơn nếu thư mục đã được index</label></div>
              </div>
              <div class="field"><label>Kho GitHub chứa bản phát hành (owner/repo)</label><input class="input mono" v-model="sys.update_repo" />
                <span class="hint">Để trống = dùng kho mặc định của bản build.</span></div>
              <button class="btn primary" style="align-self:flex-start" @click="saveSys">Lưu</button>
            </div>
          </section>
          <section class="dcard">
            <div class="dcard-h">Cấu hình tên miền (Reverse Proxy + SSL)</div>
            <div class="dcard-b stack" style="max-width:760px">
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
          </section>
        </template>

        <!-- update -->
        <template v-if="tab === 'update'">
          <section class="dcard">
            <div class="dcard-h">Phiên bản<span class="spacer"></span>
              <button class="btn sm" :disabled="updBusy" @click="checkUpdate(true)"><Icon name="refresh" :size="15" />Kiểm tra lại</button></div>
            <div class="dcard-b stack">
              <div v-if="!upd" class="empty"><div class="spinner"></div></div>
              <template v-else>
                <div class="kv">
                  <div>Phiên bản đang chạy</div><div class="mono">{{ upd.current }}</div>
                  <div>Bản mới nhất trên GitHub</div><div class="mono">{{ upd.latest || '—' }} <span v-if="upd.has_update" class="badge submitted">Có bản mới</span><span v-else-if="upd.latest" class="badge delivered">Đang dùng bản mới nhất</span></div>
                  <div>Kho phát hành</div><div class="mono"><a :href="'https://github.com/' + upd.repo + '/releases'" target="_blank">{{ upd.repo || 'chưa cấu hình' }}</a></div>
                </div>
                <div v-if="upd.error" class="alert err">Không kiểm tra được: {{ upd.error }}</div>
                <pre v-if="upd.changelog" class="logs" style="max-height:200px">{{ upd.changelog }}</pre>
              </template>
            </div>
          </section>
          <section class="dcard">
            <div class="dcard-h">Cập nhật bằng Package Center (làm 1 lần)</div>
            <div class="dcard-b stack">
              <ol class="small" style="margin:0;padding-left:20px;line-height:1.8">
                <li>Mở <b>Package Center → Settings → Package Sources → Add</b>.</li>
                <li>Name: <span class="mono">Studio Album</span> · Location: <span class="mono">{{ upd?.source_url || 'http://127.0.0.1:8686/pkg-source' }}</span></li>
                <li>Từ nay khi có bản mới, Package Center hiện nút <b>Update</b> cho Studio Album. Dữ liệu album, khách, tin nhắn được giữ nguyên.</li>
              </ol>
              <p class="hint" style="margin:0">Nếu một bản cập nhật lỗi khiến ứng dụng không chạy: tải file .spk bản trước trong trang Releases trên GitHub và cài bằng <b>Manual Install</b>.</p>
            </div>
          </section>
        </template>

        <!-- logs -->
        <template v-if="tab === 'logs'">
          <section class="dcard">
            <div class="dcard-h">Nhật ký dịch vụ<span class="spacer"></span>
              <button class="btn sm" @click="loadLogs"><Icon name="refresh" :size="15" />Tải lại</button></div>
            <div class="dcard-b"><pre class="logs">{{ logs.join('\n') || 'Chưa có nhật ký' }}</pre></div>
          </section>
        </template>
      </main>
    </div>
    <Toasts />
  </div>
</template>

<style>
/* DSM 7 look: white sidebar with blue selection, light grey content, white cards */
:root { --brand: #0b74de; --dsm-bg: #f3f5f8; --dsm-line: #e3e7ed; --dsm-text: #34404c; --dsm-sel: #e1edfb; }
body { background: var(--dsm-bg); color: var(--dsm-text); font-size: 14px; }
.dsm { height: 100vh; display: flex; flex-direction: column; }
.dsm-title { height: 44px; flex-shrink: 0; display: flex; align-items: center; gap: 10px; padding: 0 12px 0 16px; background: #fff; border-bottom: 1px solid var(--dsm-line); font-weight: 600; }
.dsm-title img { width: 22px; height: 22px; }
.dsm-auth { flex: 1; display: flex; align-items: center; justify-content: center; padding: 20px; }
.dsm-body { flex: 1; min-height: 0; display: flex; }
.dsm-nav { width: 240px; flex-shrink: 0; background: #fff; border-right: 1px solid var(--dsm-line); padding: 12px; display: flex; flex-direction: column; gap: 2px; overflow-y: auto; }
.dsm-nav button { position: relative; display: flex; align-items: center; gap: 12px; height: 42px; padding: 0 12px; border: 0; border-radius: 8px; background: none; font: inherit; font-size: 15px; color: var(--dsm-text); cursor: pointer; text-align: left; }
.dsm-nav button:hover { background: #f1f4f8; }
.dsm-nav button.on { background: var(--dsm-sel); color: #0b5cad; }
.dsm-nav button .icon { color: #6a7785; }
.dsm-nav button.on .icon { color: #0b74de; }
.navdot { position: absolute; right: 12px; width: 8px; height: 8px; border-radius: 50%; background: #e8a33d; }
.dsm-main { flex: 1; min-width: 0; overflow-y: auto; padding: 16px 20px 24px; display: flex; flex-direction: column; gap: 16px; }
.dcard { background: #fff; border: 1px solid var(--dsm-line); border-radius: 6px; }
.dcard-h { display: flex; align-items: center; gap: 8px; min-height: 48px; padding: 0 18px; border-bottom: 1px solid var(--dsm-line); font-weight: 700; font-size: 15px; }
.dcard-b { padding: 18px; }
.dcard .table th { background: #fafbfc; }
.health { display: flex; align-items: center; gap: 18px; padding: 26px 22px; }
.health h2 { font-size: 22px; font-weight: 700; }
.hicon { width: 50px; height: 50px; border-radius: 50%; display: flex; align-items: center; justify-content: center; color: #fff; flex-shrink: 0; }
.hicon.ok { background: #1db35b; }
.hicon.warn { background: #e8a33d; }
.plist { margin: 6px 0 0; padding-left: 18px; line-height: 1.8; }
.stats4 { display: grid; grid-template-columns: 1.2fr repeat(3, 1fr); padding: 20px 0; }
.stats4 > div { display: flex; flex-direction: column; align-items: center; justify-content: center; gap: 6px; }
.stats4 > div.big { border-right: 1px solid var(--dsm-line); }
.stats4 .n { font-size: 32px; font-weight: 400; }
.stats4 .si { color: #6a7785; }
.stats4 span { font-size: 16px; }
.kv { display: grid; grid-template-columns: 170px 1fr; gap: 10px 14px; font-size: 14px; }
.kv > div:nth-child(odd) { color: var(--muted); }
.tag { display: inline-flex; align-items: center; gap: 4px; padding: 3px 8px; border-radius: 6px; background: #eef2f6; font-size: 13px; }
.logs { background: #1e1f22; color: #d6d6d6; padding: 14px; border-radius: 6px; font-size: 12px; max-height: 65vh; overflow: auto; white-space: pre-wrap; margin: 0; }
.icon-btn.sm { width: 30px; height: 30px; }
.btn { border-radius: 6px; }
.btn.primary { background: #0b74de; border-color: #0b74de; }
@media (max-width: 720px) {
  .dsm-body { flex-direction: column; }
  .dsm-nav { width: auto; flex-direction: row; overflow-x: auto; border-right: 0; border-bottom: 1px solid var(--dsm-line); padding: 6px; }
  .dsm-nav button span:not(.navdot) { white-space: nowrap; }
  .stats4 { grid-template-columns: repeat(2, 1fr); gap: 16px; }
  .stats4 > div.big { border-right: 0; }
  .kv { grid-template-columns: 1fr; gap: 2px; }
  .kv > div:nth-child(even) { margin-bottom: 8px; }
}
</style>
