<script setup>
import { ref, onMounted } from 'vue'
import { get, post, put, del } from '../shared/api.js'
import { toast, toastErr } from '../shared/ui.js'
import { fmtDate } from '../shared/fmt.js'
import Icon from '../shared/Icon.vue'
import Modal from '../shared/Modal.vue'
import { state, permLabels, can } from './store.js'

const tab = ref('staff')
const staff = ref(null)
const roles = ref([])
const allPerms = ref([])
const form = ref(null)
const roleForm = ref(null)
const error = ref('')

async function load() {
  try {
    const [s, r] = await Promise.all([get('/api/s/staff'), get('/api/s/roles')])
    staff.value = s
    roles.value = r.roles
    allPerms.value = r.all_perms
  } catch (e) { toastErr(e) }
}
onMounted(load)

function openStaff(s) {
  error.value = ''
  form.value = s
    ? { id: s.id, username: s.username, display_name: s.display_name, role_id: s.role_id, active: s.active, owner: s.owner, password: '' }
    : { username: '', display_name: '', role_id: roles.value[0]?.id || 0, active: true, password: '' }
}
async function saveStaff() {
  error.value = ''
  try {
    if (form.value.id) await put(`/api/s/staff/${form.value.id}`, form.value)
    else await post('/api/s/staff', form.value)
    form.value = null
    toast('Đã lưu')
    load()
  } catch (e) { error.value = e.message }
}
async function removeStaff(s) {
  if (!confirm(`Xóa tài khoản ${s.username}?`)) return
  try { await del(`/api/s/staff/${s.id}`); load() } catch (e) { toastErr(e) }
}

function openRole(r) {
  error.value = ''
  roleForm.value = r ? { id: r.id, name: r.name, perms: [...r.perms] } : { name: '', perms: [] }
}
async function saveRole() {
  error.value = ''
  try {
    if (roleForm.value.id) await put(`/api/s/roles/${roleForm.value.id}`, roleForm.value)
    else await post('/api/s/roles', roleForm.value)
    roleForm.value = null
    toast('Đã lưu nhóm quyền')
    load()
  } catch (e) { error.value = e.message }
}
async function removeRole(r) {
  if (!confirm(`Xóa nhóm quyền "${r.name}"?`)) return
  try { await del(`/api/s/roles/${r.id}`); load() } catch (e) { toastErr(e) }
}
</script>

<template>
  <div class="page">
    <div class="page-head">
      <h1>Nhân viên & phân quyền</h1>
      <button v-if="tab === 'staff' && can('staff.manage')" class="btn primary" @click="openStaff(null)"><Icon name="plus" :size="18" />Thêm nhân viên</button>
      <button v-if="tab === 'roles' && can('staff.manage')" class="btn primary" @click="openRole(null)"><Icon name="plus" :size="18" />Thêm nhóm quyền</button>
    </div>
    <div class="tabs-line">
      <button :class="{ on: tab === 'staff' }" @click="tab = 'staff'"><Icon name="users" :size="16" />Nhân viên</button>
      <button :class="{ on: tab === 'roles' }" @click="tab = 'roles'"><Icon name="shield" :size="16" />Nhóm quyền</button>
    </div>

    <div v-if="tab === 'staff'" class="card pad0 table-wrap">
      <div v-if="!staff" class="empty"><div class="spinner"></div></div>
      <table v-else class="table">
        <thead><tr><th>Nhân viên</th><th>Nhóm quyền</th><th class="hidden-sm">Đăng nhập gần nhất</th><th></th></tr></thead>
        <tbody>
          <tr v-for="s in staff" :key="s.id">
            <td><b>{{ s.display_name || s.username }}</b> <span class="muted small">@{{ s.username }}</span>
              <span v-if="s.owner" class="badge delivered">Chủ</span><span v-if="!s.active" class="badge danger">Đã khóa</span></td>
            <td>{{ s.owner ? 'Toàn quyền' : s.role || '—' }}</td>
            <td class="hidden-sm small muted">{{ fmtDate(s.last_login, true) || 'Chưa đăng nhập' }}</td>
            <td style="text-align:right;white-space:nowrap">
              <template v-if="can('staff.manage') && (!s.owner || state.me.owner)">
                <button class="btn sm" @click="openStaff(s)"><Icon name="edit" :size="15" /></button>
                <button v-if="!s.owner && s.id !== state.me.id" class="btn sm danger" @click="removeStaff(s)"><Icon name="trash" :size="15" /></button>
              </template>
            </td>
          </tr>
        </tbody>
      </table>
    </div>

    <div v-if="tab === 'roles'" class="stack">
      <div class="alert info"><Icon name="info" :size="18" />Tài khoản <b>Chủ</b> luôn có toàn quyền. Mỗi nhân viên thuộc một nhóm quyền; tick các quyền phù hợp cho từng nhóm.</div>
      <div v-for="r in roles" :key="r.id" class="card">
        <div class="row" style="margin-bottom:10px">
          <h3 class="grow">{{ r.name }} <span class="small muted">· {{ r.staff_count }} nhân viên</span></h3>
          <template v-if="can('staff.manage')">
            <button class="btn sm" @click="openRole(r)"><Icon name="edit" :size="15" />Sửa</button>
            <button class="btn sm danger" :disabled="r.staff_count > 0" @click="removeRole(r)"><Icon name="trash" :size="15" /></button>
          </template>
        </div>
        <div class="row wrap">
          <span v-for="p in allPerms" :key="p" class="badge" :class="r.perms.includes(p) ? 'delivered' : ''" :style="r.perms.includes(p) ? '' : 'opacity:.45;text-decoration:line-through'">{{ permLabels[p]?.[0] || p }}</span>
        </div>
      </div>
    </div>
  </div>

  <Modal v-if="form" :title="form.id ? 'Sửa nhân viên' : 'Thêm nhân viên'" @close="form = null">
    <div class="grid2">
      <div class="field"><label>Tên đăng nhập</label><input class="input" v-model="form.username" :disabled="!!form.id" autocomplete="off" /></div>
      <div class="field"><label>Tên hiển thị</label><input class="input" v-model="form.display_name" /></div>
    </div>
    <div class="field"><label>{{ form.id ? 'Mật khẩu mới (để trống = giữ nguyên)' : 'Mật khẩu (≥ 6 ký tự)' }}</label><input class="input" type="text" v-model="form.password" autocomplete="new-password" /></div>
    <div v-if="!form.owner" class="field"><label>Nhóm quyền</label>
      <select class="input" v-model.number="form.role_id"><option v-for="r in roles" :key="r.id" :value="r.id">{{ r.name }}</option></select>
    </div>
    <label v-if="form.id && !form.owner" class="check"><span class="switch"><input type="checkbox" v-model="form.active" /><span></span></span>Cho phép đăng nhập</label>
    <div v-if="error" class="alert err">{{ error }}</div>
    <template #foot><button class="btn" @click="form = null">Hủy</button><button class="btn primary" @click="saveStaff">Lưu</button></template>
  </Modal>

  <Modal v-if="roleForm" :title="roleForm.id ? 'Sửa nhóm quyền' : 'Thêm nhóm quyền'" @close="roleForm = null">
    <div class="field"><label>Tên nhóm</label><input class="input" v-model="roleForm.name" placeholder="VD: Thợ edit, Lễ tân…" /></div>
    <div class="multi" style="max-height:none">
      <label v-for="p in allPerms" :key="p" style="align-items:flex-start">
        <input type="checkbox" :value="p" v-model="roleForm.perms" style="margin-top:2px" />
        <div><div style="font-weight:600">{{ permLabels[p]?.[0] || p }}</div><div class="small muted">{{ permLabels[p]?.[1] }}</div></div>
      </label>
    </div>
    <div v-if="error" class="alert err">{{ error }}</div>
    <template #foot><button class="btn" @click="roleForm = null">Hủy</button><button class="btn primary" @click="saveRole">Lưu</button></template>
  </Modal>
</template>
