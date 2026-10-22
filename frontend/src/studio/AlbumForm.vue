<script setup>
// Create / edit album settings.
import { ref, computed, onMounted } from 'vue'
import { get, post, put } from '../shared/api.js'
import { toast } from '../shared/ui.js'
import { tsToDateInput, dateInputToTs } from '../shared/fmt.js'
import Icon from '../shared/Icon.vue'
import Modal from '../shared/Modal.vue'
import FolderPicker from './FolderPicker.vue'
import CustomerCreated from './CustomerCreated.vue'

const props = defineProps({ album: Object, customerIds: Array, editorIds: Array })
const emit = defineEmits(['close', 'saved'])
const isNew = !props.album

const f = ref({
  title: props.album?.title || '',
  share: props.album?.share || '',
  rel_path: props.album?.rel_path || '',
  max_edit: props.album?.max_edit ?? 0,
  allow_download: props.album ? props.album.allow_download : true,
  expires: tsToDateInput(props.album?.expires_at),
  due: tsToDateInput(props.album?.due_at),
  note_customer: props.album?.note_customer || '',
  note_internal: props.album?.note_internal || '',
  customer_ids: [...(props.customerIds || [])],
  editor_ids: [...(props.editorIds || [])],
})
const folderImages = ref(null)
const customers = ref([])
const staff = ref([])
const cq = ref('')
const error = ref('')
const busy = ref(false)
const newCust = ref(null) // {phone,name}
const created = ref(null)

onMounted(async () => {
  const [c, s] = await Promise.all([get('/api/s/customers').catch(() => []), get('/api/s/staff').catch(() => [])])
  customers.value = c
  staff.value = s.filter((x) => x.active)
})

const filteredCustomers = computed(() => {
  const q = cq.value.trim().toLowerCase()
  const list = customers.value.filter((c) => !q || c.name.toLowerCase().includes(q) || c.phone.includes(q.replace(/\D/g, '') || q))
  // selected first
  return list.sort((a, b) => f.value.customer_ids.includes(b.id) - f.value.customer_ids.includes(a.id)).slice(0, 100)
})
const selectedCustomers = computed(() => customers.value.filter((c) => f.value.customer_ids.includes(c.id)))

function onPick(p) {
  f.value.share = p.share
  f.value.rel_path = p.path
  folderImages.value = p.images
  if (!f.value.title) f.value.title = p.name.replace(/[_-]+/g, ' ').trim()
}

async function createCustomer() {
  try {
    const r = await post('/api/s/customers', { phone: newCust.value.phone, name: newCust.value.name })
    customers.value.unshift({ id: r.id, phone: r.phone, name: newCust.value.name, albums: 0, active: true })
    f.value.customer_ids.push(r.id)
    created.value = { ...r, name: newCust.value.name }
    newCust.value = null
  } catch (e) { toast(e.message, 'err') }
}

async function save() {
  error.value = ''
  if (isNew && !f.value.rel_path) { error.value = 'Hãy chọn thư mục ảnh của album'; return }
  busy.value = true
  const body = {
    title: f.value.title, share: f.value.share, rel_path: f.value.rel_path,
    max_edit: Number(f.value.max_edit) || 0, allow_download: f.value.allow_download,
    expires_at: dateInputToTs(f.value.expires), due_at: dateInputToTs(f.value.due),
    note_customer: f.value.note_customer, note_internal: f.value.note_internal,
    customer_ids: f.value.customer_ids, editor_ids: f.value.editor_ids,
  }
  try {
    if (isNew) {
      const r = await post('/api/s/albums', body)
      emit('saved', r.id)
    } else {
      await put(`/api/s/albums/${props.album.id}`, body)
      emit('saved', props.album.id)
    }
    toast('Đã lưu album')
  } catch (e) { error.value = e.message } finally { busy.value = false }
}
</script>

<template>
  <Modal :title="isNew ? 'Tạo album mới' : 'Cài đặt album'" wide @close="emit('close')">
    <template v-if="isNew">
      <div class="field">
        <label>1. Thư mục ảnh trên NAS</label>
        <div v-if="f.rel_path" class="row alert ok">
          <Icon name="folder" /><span class="grow mono">{{ f.share }}/{{ f.rel_path }}</span>
          <span class="small">{{ folderImages }} ảnh</span>
          <button class="btn sm" @click="f.rel_path = ''">Đổi</button>
        </div>
        <FolderPicker v-else @pick="onPick" />
        <span class="hint">Ảnh chỉnh sửa xong đặt trong thư mục con <b>Final</b> của thư mục này — khách sẽ thấy ở tab “Ảnh hoàn thiện”.</span>
      </div>
    </template>
    <div v-else class="small muted"><Icon name="folder" :size="14" /> <span class="mono">{{ album.share }}/{{ album.rel_path }}</span></div>

    <div class="field"><label>Tên album</label><input class="input" v-model="f.title" placeholder="VD: Cưới hỏi Lan & Minh" /></div>

    <div class="field">
      <label>Khách hàng được xem album</label>
      <div v-if="selectedCustomers.length" class="row wrap">
        <span v-for="c in selectedCustomers" :key="c.id" class="tag">{{ c.name || c.phone }} · {{ c.phone }}
          <button class="icon-btn" style="width:20px;height:20px" @click="f.customer_ids = f.customer_ids.filter((x) => x !== c.id)"><Icon name="close" :size="12" /></button></span>
      </div>
      <div class="row">
        <input class="input" v-model="cq" placeholder="Tìm theo tên hoặc số điện thoại…" />
        <button class="btn" @click="newCust = { phone: cq.replace(/[^\d+]/g, ''), name: '' }"><Icon name="plus" :size="16" />Khách mới</button>
      </div>
      <div v-if="newCust" class="card row wrap" style="padding:10px">
        <input class="input grow" style="min-width:150px" v-model="newCust.phone" placeholder="Số điện thoại" inputmode="tel" />
        <input class="input grow" style="min-width:150px" v-model="newCust.name" placeholder="Tên khách" />
        <button class="btn primary" @click="createCustomer">Tạo</button>
        <button class="btn ghost" @click="newCust = null">Hủy</button>
      </div>
      <div class="multi" v-if="cq || !selectedCustomers.length">
        <label v-for="c in filteredCustomers" :key="c.id">
          <input type="checkbox" :value="c.id" v-model="f.customer_ids" />
          <span class="grow">{{ c.name || '(chưa có tên)' }}</span><span class="mono muted">{{ c.phone }}</span>
        </label>
        <div v-if="!filteredCustomers.length" class="empty small">Không tìm thấy khách — bấm “Khách mới”</div>
      </div>
    </div>

    <div class="grid2">
      <div class="field">
        <label>Số ảnh khách được chọn edit</label>
        <input class="input" type="number" min="0" v-model.number="f.max_edit" />
        <span class="hint">0 = không giới hạn. Khách thả tim thoải mái, chỉ giới hạn số ảnh chọn edit.</span>
      </div>
      <div class="field">
        <label>Quyền tải ảnh của khách</label>
        <label class="check" style="height:40px"><span class="switch"><input type="checkbox" v-model="f.allow_download" /><span></span></span>
          {{ f.allow_download ? 'Cho phép tải ảnh gốc' : 'Chỉ xem, không tải' }}</label>
      </div>
      <div class="field">
        <label>Hạn trả ảnh (nội bộ)</label>
        <input class="input" type="date" v-model="f.due" />
      </div>
      <div class="field">
        <label>Album hết hạn truy cập</label>
        <input class="input" type="date" v-model="f.expires" />
        <span class="hint">Để trống = không hết hạn.</span>
      </div>
    </div>

    <div class="field">
      <label>Nhân viên edit được gán</label>
      <div class="multi">
        <label v-for="s in staff" :key="s.id">
          <input type="checkbox" :value="s.id" v-model="f.editor_ids" />
          <span class="grow">{{ s.display_name || s.username }}</span><span class="small muted">{{ s.owner ? 'Chủ' : s.role }}</span>
        </label>
      </div>
    </div>
    <div class="field"><label>Lời nhắn hiển thị cho khách</label><textarea class="input" v-model="f.note_customer" rows="2" placeholder="VD: Gói của bạn gồm 30 ảnh chỉnh sửa + 1 ảnh phóng lớn."></textarea></div>
    <div class="field"><label>Ghi chú nội bộ (khách không thấy)</label><textarea class="input" v-model="f.note_internal" rows="2"></textarea></div>
    <div v-if="error" class="alert err">{{ error }}</div>
    <template #foot>
      <button class="btn" @click="emit('close')">Hủy</button>
      <button class="btn primary" :disabled="busy" @click="save">{{ isNew ? 'Tạo album' : 'Lưu' }}</button>
    </template>
  </Modal>
  <CustomerCreated v-if="created" :info="created" @close="created = null" />
</template>
