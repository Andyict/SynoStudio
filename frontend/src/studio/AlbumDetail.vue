<script setup>
import { ref, computed, watch, onMounted, onBeforeUnmount } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { get, post, del } from '../shared/api.js'
import { connectWS } from '../shared/ws.js'
import { toast, toastErr } from '../shared/ui.js'
import { download, fmtDate, fmtTime, copyText, imgUrl, daysLeft, tsToDateInput, dateInputToTs } from '../shared/fmt.js'
import Icon from '../shared/Icon.vue'
import Modal from '../shared/Modal.vue'
import PhotoGrid from '../shared/PhotoGrid.vue'
import Viewer from '../shared/Viewer.vue'
import ChatPanel from '../shared/ChatPanel.vue'
import AlbumForm from './AlbumForm.vue'
import { can, statusLabel } from './store.js'

const props = defineProps({ id: Number })
const route = useRoute()
const router = useRouter()

const data = ref(null)
const error = ref('')
const tab = ref(route.query.tab || 'photos')
const filter = ref('all')
const comments = ref(null)
const viewList = ref([])
const viewIdx = ref(-1)
const panelOpen = ref(false)
const editing = ref(false)
const shareOpen = ref(false)
const share = ref({ enabled: false, expires: '' })
const statusMenu = ref(false)
const dlMenu = ref(false)
const confirmDelete = ref(false)

const album = computed(() => data.value?.album)
const photos = computed(() => data.value?.photos || [])
const origs = computed(() => photos.value.filter((p) => p.kind === 'orig'))
const finals = computed(() => photos.value.filter((p) => p.kind === 'final'))
const favs = computed(() => origs.value.filter((p) => p.fav))
const edits = computed(() => origs.value.filter((p) => p.edit))
const doneCount = computed(() => edits.value.filter((p) => p.done).length)
const editIndex = computed(() => Object.fromEntries(edits.value.map((p, i) => [p.id, i + 1])))
const shown = computed(() => ({ all: origs.value, fav: favs.value, edit: edits.value, final: finals.value })[filter.value])
const viewPhoto = computed(() => viewList.value[viewIdx.value])
const commentsByPhoto = computed(() => {
  const m = {}
  for (const c of comments.value || []) if (c.photo_id) (m[c.photo_id] ||= []).push(c)
  return m
})
const photoComments = computed(() => commentsByPhoto.value[viewPhoto.value?.id] || [])
const shareUrl = computed(() => `${data.value?.public_url || location.origin}/s/${album.value?.share_token}`)

const statusTargets = computed(() => {
  if (can('albums.status')) return Object.keys(statusLabel).filter((s) => s !== album.value?.status)
  if (can('edit.work')) return ['editing', 'delivered'].filter((s) => s !== album.value?.status)
  return []
})
const statusAction = {
  selecting: 'Mở lại cho khách chọn ảnh',
  submitted: 'Đánh dấu khách đã gửi (khóa chọn)',
  editing: 'Bắt đầu chỉnh sửa',
  delivered: 'Đã trả ảnh cho khách',
}

async function load() {
  try {
    data.value = await get(`/api/s/albums/${props.id}`)
    if (data.value.scan_error) error.value = data.value.scan_error
  } catch (e) { error.value = e.message }
}
async function loadComments() {
  try { comments.value = await get(`/api/s/albums/${props.id}/comments?scope=all`) } catch (e) { toastErr(e) }
}
function markRead() {
  if (album.value) album.value.unread = 0
  post(`/api/s/albums/${props.id}/read`).catch(() => {})
}
watch(tab, (t) => {
  router.replace({ query: { ...route.query, tab: t === 'photos' ? undefined : t } })
  if (t === 'chat') markRead()
})

function onEvent(ev) {
  if (ev.type === 'photo') {
    const p = photos.value.find((x) => x.id === ev.id)
    if (p) { p.fav = ev.fav; p.edit = ev.edit }
    if (album.value) {
      album.value.fav_count = favs.value.length
      album.value.edit_count = edits.value.length
    }
  } else if (ev.type === 'done') {
    const p = photos.value.find((x) => x.id === ev.id)
    if (p) p.done = ev.done
  } else if (ev.type === 'comment') {
    const c = ev.comment
    addComment({ ...c, mine: false })
    if (c.author_type !== 'staff') {
      if (tab.value === 'chat') markRead()
      else if (album.value) album.value.unread++
    }
  } else if (ev.type === 'status' || ev.type === 'album_changed') {
    load()
  } else if (ev.type === 'customer_note' && album.value) {
    album.value.customer_note = ev.text
  }
}
let closeWS = null
onMounted(async () => {
  await load()
  loadComments()
  if (tab.value === 'chat') markRead()
  closeWS = connectWS(`album=${props.id}&as=staff`, onEvent, () => { load(); loadComments() })
})
onBeforeUnmount(() => closeWS && closeWS())

// viewer
function openViewer(list, i) { viewList.value = list.slice(); viewIdx.value = i }
function openPhotoById(id) {
  const p = photos.value.find((x) => x.id === id)
  if (!p) return
  const list = p.kind === 'final' ? finals.value : origs.value
  openViewer(list, list.indexOf(p))
  panelOpen.value = true
}

// Adds a comment once (it may arrive both from the POST response and the websocket).
function addComment(c) {
  if (!comments.value) comments.value = []
  const i = comments.value.findIndex((x) => x.id === c.id)
  if (i >= 0) {
    if (c.mine) comments.value[i] = c
    return
  }
  comments.value.push(c)
  if (c.photo_id) { const p = photos.value.find((x) => x.id === c.photo_id); if (p) p.cm = (p.cm || 0) + 1 }
}
async function sendComment(body, photoId = 0) {
  try {
    addComment(await post(`/api/s/albums/${props.id}/comments`, { body, photo_id: photoId }))
  } catch (e) { toastErr(e) }
}

async function setDone(p, on) {
  const before = p.done
  p.done = on
  try { await post(`/api/s/albums/${props.id}/photos/${p.id}/done`, { on }) } catch (e) { p.done = before; toastErr(e) }
}

async function setStatus(s) {
  statusMenu.value = false
  try { await post(`/api/s/albums/${props.id}/status`, { status: s }); toast('Đã cập nhật trạng thái'); load() } catch (e) { toastErr(e) }
}
async function rescan() {
  try { await post(`/api/s/albums/${props.id}/rescan`); toast('Đã quét lại thư mục'); load() } catch (e) { toastErr(e) }
}
function dl(set) {
  dlMenu.value = false
  download(`/dl/s/${props.id}?set=${set}`)
  toast('Đang chuẩn bị file ZIP…')
}
async function copyNames() {
  const r = await fetch(`/dl/s/${props.id}/list?set=edit&inline=1`, { credentials: 'same-origin' })
  const text = await r.text()
  if (await copyText(text)) toast(`Đã chép ${edits.value.length} tên file — dán vào ô tìm kiếm Library Filter của Lightroom`)
}

function openShare() {
  share.value = { enabled: album.value.share_enabled, expires: tsToDateInput(album.value.share_expires_at) }
  shareOpen.value = true
}
async function saveShare(regenerate = false) {
  try {
    const r = await post(`/api/s/albums/${props.id}/share`, { enabled: share.value.enabled, regenerate, expires_at: dateInputToTs(share.value.expires) })
    Object.assign(album.value, { share_token: r.share_token, share_enabled: r.share_enabled, share_expires_at: r.share_expires_at })
    toast(regenerate ? 'Đã tạo link mới' : 'Đã lưu')
  } catch (e) { toastErr(e) }
}
async function copyShare() { if (await copyText(shareUrl.value)) toast('Đã chép link') }

async function deleteAlbum() {
  try { await del(`/api/s/albums/${props.id}`); toast('Đã xóa album'); router.replace('/albums') } catch (e) { toastErr(e) }
}
function onSaved() { editing.value = false; load() }
</script>

<template>
  <div class="page">
    <div v-if="!data && !error" class="empty"><div class="spinner"></div></div>
    <div v-else-if="!data" class="alert err">{{ error }}</div>
    <template v-else>
      <div class="page-head">
        <router-link to="/albums" class="icon-btn"><Icon name="back" /></router-link>
        <div class="grow" style="min-width:200px">
          <h1>{{ album.title }}</h1>
          <div class="row wrap small muted" style="margin-top:4px">
            <span class="badge" :class="album.status">{{ statusLabel[album.status] }}</span>
            <span>{{ album.customers.join(', ') || 'Chưa gán khách' }}</span>
            <span v-if="album.submitted_at">· gửi lúc {{ fmtTime(album.submitted_at) }}</span>
            <span v-if="album.due_at && album.status !== 'delivered'" :class="daysLeft(album.due_at) < 0 ? 'badge danger' : ''">· Hạn trả {{ fmtDate(album.due_at) }}</span>
          </div>
        </div>
        <div class="row wrap">
          <div v-if="statusTargets.length" class="menu-wrap">
            <button class="btn" @click="statusMenu = !statusMenu"><Icon name="refresh" :size="16" />Trạng thái</button>
            <template v-if="statusMenu">
              <div class="menu-catch" @click="statusMenu = false"></div>
              <div class="menu" style="z-index:61">
                <button v-for="s in statusTargets" :key="s" @click="setStatus(s)"><span class="badge" :class="s">{{ statusLabel[s] }}</span>{{ statusAction[s] }}</button>
              </div>
            </template>
          </div>
          <div class="menu-wrap">
            <button class="btn" @click="dlMenu = !dlMenu"><Icon name="download" :size="16" />Tải</button>
            <template v-if="dlMenu">
              <div class="menu-catch" @click="dlMenu = false"></div>
              <div class="menu" style="z-index:61">
                <button :disabled="!edits.length" @click="dl('edit')"><Icon name="edit" :size="18" />Ảnh khách chọn edit ({{ edits.length }})</button>
                <button :disabled="!favs.length" @click="dl('fav')"><Icon name="heart" :size="18" />Ảnh yêu thích ({{ favs.length }})</button>
                <button :disabled="!finals.length" @click="dl('final')"><Icon name="star" :size="18" />Ảnh Final ({{ finals.length }})</button>
                <button @click="dl('all')"><Icon name="images" :size="18" />Tất cả ảnh gốc ({{ origs.length }})</button>
              </div>
            </template>
          </div>
          <button v-if="can('albums.manage')" class="btn" @click="openShare"><Icon name="link" :size="16" /><span class="hidden-sm">Link xem</span></button>
          <button v-if="can('albums.manage')" class="btn" @click="editing = true"><Icon name="settings" :size="16" /><span class="hidden-sm">Cài đặt</span></button>
        </div>
      </div>

      <div v-if="error" class="alert err">{{ error }}</div>
      <div v-if="album.customer_note" class="alert info"><Icon name="note" :size="18" /><div><b>Ghi chú chung của khách:</b><div style="white-space:pre-wrap">{{ album.customer_note }}</div></div></div>

      <div class="tabs-line">
        <button :class="{ on: tab === 'photos' }" @click="tab = 'photos'"><Icon name="images" :size="16" />Ảnh</button>
        <button :class="{ on: tab === 'edit' }" @click="tab = 'edit'"><Icon name="edit" :size="16" />Danh sách edit <span class="badge">{{ doneCount }}/{{ edits.length }}<template v-if="album.max_edit"> · tối đa {{ album.max_edit }}</template></span></button>
        <button :class="{ on: tab === 'chat' }" @click="tab = 'chat'"><Icon name="chat" :size="16" />Trao đổi <span v-if="album.unread" class="dot">{{ album.unread }}</span></button>
        <button :class="{ on: tab === 'info' }" @click="tab = 'info'"><Icon name="info" :size="16" />Thông tin</button>
      </div>

      <!-- photos -->
      <template v-if="tab === 'photos'">
        <div class="row wrap">
          <div class="chips">
            <button class="chip" :class="{ on: filter === 'all' }" @click="filter = 'all'">Tất cả <span class="n">{{ origs.length }}</span></button>
            <button class="chip" :class="{ on: filter === 'fav' }" @click="filter = 'fav'"><Icon name="heart" :size="15" />Yêu thích <span class="n">{{ favs.length }}</span></button>
            <button class="chip" :class="{ on: filter === 'edit' }" @click="filter = 'edit'"><Icon name="edit" :size="15" />Chọn edit <span class="n">{{ edits.length }}</span></button>
            <button class="chip" :class="{ on: filter === 'final' }" @click="filter = 'final'"><Icon name="star" :size="15" />Final <span class="n">{{ finals.length }}</span></button>
          </div>
          <span class="spacer"></span>
          <button class="btn sm" @click="rescan"><Icon name="refresh" :size="15" />Quét lại thư mục</button>
        </div>
        <div v-if="!shown.length" class="empty"><Icon name="images" :size="36" />{{ filter === 'final' ? 'Chưa có ảnh trong thư mục Final' : 'Không có ảnh' }}</div>
        <PhotoGrid v-else :photos="shown" mode="staff" :edit-index="editIndex" @open="(i) => openViewer(shown, i)" />
      </template>

      <!-- editor list -->
      <template v-if="tab === 'edit'">
        <div class="row wrap">
          <div class="grow" style="min-width:200px">
            <div class="small">Đã edit <b>{{ doneCount }}</b> / {{ edits.length }} ảnh</div>
            <div class="progress" :class="{ full: edits.length && doneCount === edits.length }" style="max-width:360px;margin-top:6px"><div :style="{ width: (edits.length ? doneCount / edits.length * 100 : 0) + '%' }"></div></div>
          </div>
          <button class="btn sm" :disabled="!edits.length" @click="copyNames"><Icon name="copy" :size="15" />Chép tên file (Lightroom)</button>
          <a class="btn sm" :href="`/dl/s/${id}/list?set=edit&format=csv`"><Icon name="file" :size="15" />Xuất CSV</a>
          <button class="btn sm" :disabled="!edits.length" @click="dl('edit')"><Icon name="download" :size="15" />Tải ZIP</button>
        </div>
        <div class="card pad0 table-wrap">
          <div v-if="!edits.length" class="empty">Khách chưa chọn ảnh edit</div>
          <table v-else class="table">
            <thead><tr><th style="width:44px">#</th><th style="width:80px">Ảnh</th><th>Tên file</th><th>Ghi chú của khách</th><th style="width:90px">Đã edit</th></tr></thead>
            <tbody>
              <tr v-for="(p, i) in edits" :key="p.id">
                <td class="muted">{{ i + 1 }}</td>
                <td><img :src="imgUrl(p.id, 's')" style="width:64px;height:64px;object-fit:cover;border-radius:6px;cursor:pointer" loading="lazy" @click="openViewer(edits, i)" /></td>
                <td class="mono">{{ p.name }}</td>
                <td>
                  <div v-for="c in commentsByPhoto[p.id] || []" :key="c.id" class="small"><b>{{ c.author_type === 'staff' ? 'Studio' : c.author_name }}:</b> {{ c.body }}</div>
                  <span v-if="!(commentsByPhoto[p.id] || []).length" class="muted small">—</span>
                </td>
                <td><label class="check"><input type="checkbox" :checked="p.done" :disabled="!can('edit.work')" @change="setDone(p, $event.target.checked)" /></label></td>
              </tr>
            </tbody>
          </table>
        </div>
      </template>

      <!-- chat -->
      <div v-if="tab === 'chat'" class="card pad0 chatbox">
        <ChatPanel :comments="comments || []" :loading="!comments" :can-post="can('chat')" placeholder="Trả lời khách…"
          empty-text="Chưa có trao đổi nào" @send="(b) => sendComment(b)" @open-photo="openPhotoById" />
      </div>

      <!-- info -->
      <div v-if="tab === 'info'" class="card">
        <div class="kv">
          <div>Thư mục</div><div class="mono">{{ album.share }}/{{ album.rel_path }}</div>
          <div>Khách hàng</div><div>{{ album.customers.join(', ') || '—' }}</div>
          <div>Nhân viên edit</div><div>{{ album.editors.join(', ') || '—' }}</div>
          <div>Số ảnh</div><div>{{ origs.length }} ảnh gốc · {{ finals.length }} ảnh Final</div>
          <div>Giới hạn chọn edit</div><div>{{ album.max_edit || 'Không giới hạn' }}</div>
          <div>Khách tải ảnh</div><div>{{ album.allow_download ? 'Cho phép' : 'Không' }}</div>
          <div>Hạn trả ảnh</div><div>{{ fmtDate(album.due_at) || '—' }}</div>
          <div>Hết hạn truy cập</div><div>{{ fmtDate(album.expires_at) || 'Không' }}</div>
          <div>Link xem công khai</div><div>{{ album.share_enabled ? shareUrl : 'Tắt' }}</div>
          <div>Lời nhắn cho khách</div><div style="white-space:pre-wrap">{{ album.note_customer || '—' }}</div>
          <div>Ghi chú nội bộ</div><div style="white-space:pre-wrap">{{ album.note_internal || '—' }}</div>
          <div>Tạo lúc</div><div>{{ fmtDate(album.created_at, true) }}</div>
        </div>
        <div v-if="can('albums.manage')" class="row" style="margin-top:18px">
          <span class="spacer"></span>
          <button class="btn danger" @click="confirmDelete = true"><Icon name="trash" :size="16" />Xóa album</button>
        </div>
      </div>
    </template>
  </div>

  <Viewer v-if="viewIdx >= 0 && viewPhoto" :photos="viewList" v-model:index="viewIdx" :panel-open="panelOpen" @close="viewIdx = -1; panelOpen = false">
    <template #top="{ photo }">
      <span v-if="photo.fav" class="badge" style="background:var(--heart);color:#fff"><Icon name="heart" :size="12" fill />Yêu thích</span>
      <span v-if="photo.edit" class="badge" style="background:var(--brand);color:var(--brand-ink)"><Icon name="edit" :size="12" />Edit #{{ editIndex[photo.id] }}</span>
    </template>
    <template #actions="{ photo }">
      <button v-if="photo.edit && can('edit.work')" class="vbtn" :class="{ 'on-edit': photo.done }" @click="setDone(photo, !photo.done)">
        <Icon name="check" :size="18" />{{ photo.done ? 'Đã edit' : 'Đánh dấu đã edit' }}</button>
      <button class="vbtn" :class="{ 'on-edit': panelOpen }" @click="panelOpen = !panelOpen"><Icon name="comment" :size="18" />Ghi chú<span v-if="photo.cm" class="dot" style="background:#fff;color:#000">{{ photo.cm }}</span></button>
      <button class="vbtn" @click="download(`/img/${photo.id}/o?dl=1`)"><Icon name="download" :size="18" />Gốc</button>
    </template>
    <template #panel="{ photo }">
      <div class="row" style="padding:10px 8px 10px 16px;border-bottom:1px solid var(--border)">
        <h3 class="grow ellipsis">Ghi chú · {{ photo.name }}</h3>
        <button class="icon-btn" @click="panelOpen = false"><Icon name="close" /></button>
      </div>
      <ChatPanel :comments="photoComments" :loading="!comments" :show-photo-ref="false" :can-post="can('chat')"
        placeholder="Trả lời ghi chú của khách…" empty-text="Chưa có ghi chú cho ảnh này" @send="(b) => sendComment(b, photo.id)" />
    </template>
  </Viewer>

  <AlbumForm v-if="editing" :album="album" :customer-ids="data.customer_ids" :editor-ids="data.editor_ids" @close="editing = false" @saved="onSaved" />

  <Modal v-if="shareOpen" title="Link xem công khai" @close="shareOpen = false">
    <p class="hint" style="margin:0">Ai có link đều xem được album (chỉ xem, không thả tim/chat; tải ảnh theo cài đặt album). Dùng để chia sẻ cho người thân của khách.</p>
    <label class="check"><span class="switch"><input type="checkbox" v-model="share.enabled" @change="saveShare(false)" /><span></span></span>Bật link xem công khai</label>
    <template v-if="album.share_token && share.enabled">
      <div class="row"><input class="input mono" readonly :value="shareUrl" /><button class="btn" @click="copyShare"><Icon name="copy" :size="16" /></button></div>
      <div class="field"><label>Link hết hạn</label><input class="input" type="date" v-model="share.expires" @change="saveShare(false)" /></div>
      <button class="btn danger sm" style="align-self:flex-start" @click="saveShare(true)"><Icon name="refresh" :size="15" />Tạo link mới (vô hiệu link cũ)</button>
    </template>
    <div v-if="!data?.public_url" class="alert warn small">Chưa cài “Địa chỉ public” trong Cài đặt — link đang dùng địa chỉ bạn đang mở.</div>
  </Modal>

  <Modal v-if="confirmDelete" title="Xóa album?" @close="confirmDelete = false">
    <p style="margin:0">Album, lựa chọn của khách và tin nhắn sẽ bị xóa khỏi ứng dụng. <b>File ảnh trên NAS không bị xóa.</b></p>
    <template #foot><button class="btn" @click="confirmDelete = false">Hủy</button><button class="btn danger solid" @click="deleteAlbum">Xóa album</button></template>
  </Modal>
</template>

<style scoped>
.chatbox { display: flex; flex-direction: column; height: calc(100vh - 260px); min-height: 420px; overflow: hidden; }
.menu-catch { position: fixed; inset: 0; z-index: 50; }
</style>
