<script setup>
import { ref, watch, onMounted, computed } from 'vue'
import { get, post, put, del } from '../shared/api.js'
import { toast, toastErr } from '../shared/ui.js'
import { fmtDate } from '../shared/fmt.js'
import Icon from '../shared/Icon.vue'
import Modal from '../shared/Modal.vue'
import CustomerCreated from './CustomerCreated.vue'
import { can } from './store.js'

const list = ref(null)
const q = ref('')
const form = ref(null) // {id?, phone, name, password, note, active, album_ids}
const albums = ref([])
const aq = ref('')
const created = ref(null)
const confirmDel = ref(null)
const error = ref('')

async function load() {
  try { list.value = await get('/api/s/customers' + (q.value ? '?q=' + encodeURIComponent(q.value) : '')) } catch (e) { toastErr(e) }
}
let timer = null
watch(q, () => { clearTimeout(timer); timer = setTimeout(load, 300) })
onMounted(load)

async function openForm(c) {
  error.value = ''
  aq.value = ''
  if (!albums.value.length) albums.value = await get('/api/s/albums').catch(() => [])
  if (c) {
    form.value = { id: c.id, phone: c.phone, name: c.name, password: '', note: c.note, active: c.active, album_ids: [...c.album_ids] }
  } else {
    form.value = { phone: '', name: '', password: '', note: '', active: true, album_ids: [] }
  }
}
const filteredAlbums = computed(() => {
  const s = aq.value.trim().toLowerCase()
  return albums.value.filter((a) => !s || a.title.toLowerCase().includes(s) || form.value?.album_ids.includes(a.id)).slice(0, 80)
})

async function save() {
  error.value = ''
  try {
    if (form.value.id) {
      await put(`/api/s/customers/${form.value.id}`, form.value)
      toast('Đã lưu')
    } else {
      const r = await post('/api/s/customers', form.value)
      created.value = { ...r, name: form.value.name }
    }
    form.value = null
    load()
  } catch (e) { error.value = e.message }
}
async function resetPw(c) {
  try {
    const r = await post(`/api/s/customers/${c.id}/reset-password`)
    created.value = { phone: c.phone, name: c.name, password: r.password }
  } catch (e) { toastErr(e) }
}
async function remove() {
  try { await del(`/api/s/customers/${confirmDel.value.id}`); confirmDel.value = null; toast('Đã xóa'); load() } catch (e) { toastErr(e) }
}
</script>

<template>
  <div class="page">
    <div class="page-head">
      <h1>Khách hàng</h1>
      <button v-if="can('customers.manage')" class="btn primary" @click="openForm(null)"><Icon name="plus" :size="18" />Thêm khách</button>
    </div>
    <div style="position:relative">
      <input class="input" v-model="q" placeholder="Tìm theo tên hoặc số điện thoại…" style="padding-left:36px" />
      <Icon name="search" :size="18" class="muted" style="position:absolute;left:11px;top:11px" />
    </div>
    <div class="card pad0 table-wrap">
      <div v-if="!list" class="empty"><div class="spinner"></div></div>
      <div v-else-if="!list.length" class="empty"><Icon name="users" :size="36" />Chưa có khách hàng</div>
      <table v-else class="table">
        <thead><tr><th>Khách hàng</th><th>Số điện thoại</th><th class="hidden-sm">Album</th><th class="hidden-sm">Đăng nhập gần nhất</th><th></th></tr></thead>
        <tbody>
          <tr v-for="c in list" :key="c.id">
            <td><b>{{ c.name || '(chưa có tên)' }}</b> <span v-if="!c.active" class="badge danger">Đã khóa</span><div v-if="c.note" class="small muted ellipsis" style="max-width:280px">{{ c.note }}</div></td>
            <td class="mono">{{ c.phone }}</td>
            <td class="hidden-sm">{{ c.albums }}</td>
            <td class="hidden-sm small muted">{{ fmtDate(c.last_login, true) || 'Chưa đăng nhập' }}</td>
            <td style="text-align:right;white-space:nowrap">
              <template v-if="can('customers.manage')">
                <button class="btn sm" @click="resetPw(c)" title="Cấp mật khẩu mới"><Icon name="key" :size="15" /><span class="hidden-sm">Mật khẩu mới</span></button>
                <button class="btn sm" @click="openForm(c)"><Icon name="edit" :size="15" /></button>
                <button class="btn sm danger" @click="confirmDel = c"><Icon name="trash" :size="15" /></button>
              </template>
            </td>
          </tr>
        </tbody>
      </table>
    </div>
  </div>

  <Modal v-if="form" :title="form.id ? 'Sửa khách hàng' : 'Thêm khách hàng'" @close="form = null">
    <div class="grid2">
      <div class="field"><label>Số điện thoại (tên đăng nhập)</label><input class="input" v-model="form.phone" inputmode="tel" /></div>
      <div class="field"><label>Tên khách</label><input class="input" v-model="form.name" /></div>
    </div>
    <div class="field">
      <label>{{ form.id ? 'Đặt mật khẩu mới (để trống = giữ nguyên)' : 'Mật khẩu (để trống = tạo ngẫu nhiên)' }}</label>
      <input class="input" v-model="form.password" autocomplete="new-password" />
    </div>
    <div class="field"><label>Ghi chú nội bộ</label><textarea class="input" v-model="form.note" rows="2"></textarea></div>
    <div class="field">
      <label>Album của khách</label>
      <input class="input" v-model="aq" placeholder="Tìm album…" />
      <div class="multi">
        <label v-for="a in filteredAlbums" :key="a.id"><input type="checkbox" :value="a.id" v-model="form.album_ids" /><span class="grow">{{ a.title }}</span><span class="small muted">{{ a.photo_count }} ảnh</span></label>
        <div v-if="!filteredAlbums.length" class="empty small">Chưa có album</div>
      </div>
    </div>
    <label v-if="form.id" class="check"><span class="switch"><input type="checkbox" v-model="form.active" /><span></span></span>Cho phép đăng nhập</label>
    <div v-if="error" class="alert err">{{ error }}</div>
    <template #foot><button class="btn" @click="form = null">Hủy</button><button class="btn primary" @click="save">Lưu</button></template>
  </Modal>

  <CustomerCreated v-if="created" :info="created" @close="created = null" />

  <Modal v-if="confirmDel" title="Xóa khách hàng?" @close="confirmDel = null">
    <p style="margin:0">Xóa <b>{{ confirmDel.name || confirmDel.phone }}</b>. Album và ảnh vẫn giữ nguyên; tin nhắn của khách vẫn còn trong album.</p>
    <template #foot><button class="btn" @click="confirmDel = null">Hủy</button><button class="btn danger solid" @click="remove">Xóa</button></template>
  </Modal>
</template>
