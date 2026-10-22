<script setup>
import { ref, computed, watch, onMounted, onBeforeUnmount } from 'vue'
import { get, post } from '../shared/api.js'
import { connectWS } from '../shared/ws.js'
import { toast } from '../shared/ui.js'
import { download, fmtDate } from '../shared/fmt.js'
import Icon from '../shared/Icon.vue'
import Modal from '../shared/Modal.vue'
import PhotoGrid from '../shared/PhotoGrid.vue'
import Viewer from '../shared/Viewer.vue'
import ChatPanel from '../shared/ChatPanel.vue'
import TopBar from './TopBar.vue'
import { t, lang, errText } from './i18n.js'

const props = defineProps({ id: Number })

const data = ref(null)
const error = ref('')
const tab = ref('all')
const unseenOnly = ref(false)
const viewIdx = ref(-1)
const viewList = ref([]) // snapshot of the list the viewer was opened from
const panelOpen = ref(false)
const chatOpen = ref(false)
const comments = ref(null)
const noteOpen = ref(false)
const noteText = ref('')
const confirmOpen = ref(false)
const dlMenu = ref(false)

const album = computed(() => data.value?.album)
const locked = computed(() => !!data.value?.locked)
const photos = computed(() => data.value?.photos || [])
const origs = computed(() => photos.value.filter((p) => p.kind === 'orig'))
const finals = computed(() => photos.value.filter((p) => p.kind === 'final'))
const favs = computed(() => origs.value.filter((p) => p.fav))
const edits = computed(() => origs.value.filter((p) => p.edit))
const maxEdit = computed(() => album.value?.max_edit || 0)
const editIndex = computed(() => Object.fromEntries(edits.value.map((p, i) => [p.id, i + 1])))

const shown = computed(() => {
  switch (tab.value) {
    case 'fav': return favs.value
    case 'edit': return edits.value
    case 'final': return finals.value
    default: return unseenOnly.value ? origs.value.filter((p) => !p.viewed) : origs.value
  }
})
const viewPhoto = computed(() => viewList.value[viewIdx.value])
const photoComments = computed(() => (comments.value || []).filter((c) => c.photo_id === viewPhoto.value?.id))

let loadedOnce = false
async function load() {
  error.value = ''
  try {
    data.value = await get(`/api/c/albums/${props.id}`)
    if (data.value.album.status === 'delivered' && finals.value.length && tab.value === 'all' && !loadedOnce) tab.value = 'final'
    loadedOnce = true
  } catch (e) {
    error.value = e.code === 'expired' ? t('album_expired') : errText(e)
  }
}

async function loadComments() {
  try { comments.value = await get(`/api/c/albums/${props.id}/comments?scope=all`) } catch (e) { toast(errText(e), 'err') }
}

function findPhoto(id) { return photos.value.find((p) => p.id === id) }

async function setPick(p, body) {
  const before = { fav: p.fav, edit: p.edit }
  if ('fav' in body) { p.fav = body.fav; if (!body.fav) p.edit = false }
  if ('edit' in body) { p.edit = body.edit; if (body.edit) p.fav = true }
  try {
    const r = await post(`/api/c/albums/${props.id}/photos/${p.id}/pick`, body)
    p.fav = r.fav; p.edit = r.edit
  } catch (e) {
    Object.assign(p, before)
    toast(errText(e), 'err')
    if (e.code === 'locked') load()
  }
}
function heart(p) {
  if (locked.value) return toast(t('locked_banner'), 'err')
  setPick(p, { fav: !p.fav })
}
function toggleEdit(p) {
  if (locked.value) return toast(t('locked_banner'), 'err')
  if (!p.edit && maxEdit.value && edits.value.length >= maxEdit.value) return toast(t('limit_reached', { n: maxEdit.value }), 'err')
  setPick(p, { edit: !p.edit })
}

// viewer
function openViewer(i) {
  viewList.value = shown.value.slice()
  viewIdx.value = i
}
function openPhotoById(id) {
  const p = findPhoto(id)
  if (!p) return
  chatOpen.value = false
  viewList.value = p.kind === 'final' ? finals.value.slice() : origs.value.slice()
  viewIdx.value = viewList.value.indexOf(p)
  panelOpen.value = true
}
function closeViewer() { viewIdx.value = -1; panelOpen.value = false }
function onHotkey(k) {
  const p = viewPhoto.value
  if (!p || p.kind !== 'orig') return
  if (k === 'h' || k === 'f' || k === ' ') heart(p)
  else if (k === 'e') toggleEdit(p)
}
function togglePanel() {
  panelOpen.value = !panelOpen.value
  if (panelOpen.value && !comments.value) loadComments()
}

// "viewed" markers are batched
const viewedQueue = new Set()
let viewedTimer = null
watch(viewPhoto, (p) => {
  if (!p || p.viewed) return
  p.viewed = true
  viewedQueue.add(p.id)
  clearTimeout(viewedTimer)
  viewedTimer = setTimeout(flushViewed, 2000)
})
function flushViewed() {
  if (!viewedQueue.size) return
  const ids = [...viewedQueue]
  viewedQueue.clear()
  post(`/api/c/albums/${props.id}/viewed`, { ids }).catch(() => {})
}

// chat
async function openChat() {
  chatOpen.value = true
  if (!comments.value) await loadComments()
  markRead()
}
function markRead() {
  if (data.value) data.value.unread = 0
  post(`/api/c/albums/${props.id}/read`).catch(() => {})
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
  if (c.photo_id) { const p = findPhoto(c.photo_id); if (p) p.cm = (p.cm || 0) + 1 }
}
async function sendComment(body, photoId = 0) {
  try {
    addComment(await post(`/api/c/albums/${props.id}/comments`, { body, photo_id: photoId }))
  } catch (e) { toast(errText(e), 'err') }
}

// realtime
function onEvent(ev) {
  if (ev.type === 'photo') {
    const p = findPhoto(ev.id)
    if (p) { p.fav = ev.fav; p.edit = ev.edit }
  } else if (ev.type === 'comment') {
    const c = ev.comment
    addComment({ ...c, mine: false })
    if (c.author_type === 'staff') {
      if (chatOpen.value) markRead()
      else if (data.value) data.value.unread++
    }
  } else if (ev.type === 'status' || ev.type === 'album_changed') {
    load()
  } else if (ev.type === 'customer_note' && album.value) {
    album.value.customer_note = ev.text
  }
}
let closeWS = null

// note
function openNote() { noteText.value = album.value.customer_note || ''; noteOpen.value = true }
async function saveNote() {
  try {
    await post(`/api/c/albums/${props.id}/note`, { text: noteText.value })
    album.value.customer_note = noteText.value
    noteOpen.value = false
    toast(t('saved'))
  } catch (e) { toast(errText(e), 'err') }
}

async function submit() {
  try {
    await post(`/api/c/albums/${props.id}/submit`)
    confirmOpen.value = false
    toast(t('submitted_ok'))
    load()
  } catch (e) { toast(errText(e), 'err') }
}

function dl(set) {
  dlMenu.value = false
  if (!album.value.allow_download) return toast(t('download_off'), 'err')
  download(`/dl/c/${props.id}?set=${set}`)
  toast(t('download_started'))
}
function dlPhoto(p) {
  if (!album.value.allow_download) return toast(t('download_off'), 'err')
  download(`/img/${p.id}/o?dl=1`)
}

watch(shown, () => { if (tab.value !== 'all') unseenOnly.value = false })

onMounted(() => {
  load()
  closeWS = connectWS(`album=${props.id}`, onEvent, load)
})
onBeforeUnmount(() => { closeWS && closeWS(); flushViewed() })
</script>

<template>
  <TopBar back :title="album?.title">
    <button v-if="album" class="icon-btn" @click="openChat" :aria-label="t('chat')">
      <Icon name="chat" /><span v-if="data.unread" class="dot">{{ data.unread }}</span>
    </button>
    <div v-if="album" class="menu-wrap">
      <button class="icon-btn" @click="dlMenu = !dlMenu" :aria-label="t('download')"><Icon name="download" /></button>
      <template v-if="dlMenu">
        <div class="menu-catch" @click="dlMenu = false"></div>
        <div class="menu" style="z-index:61">
          <div v-if="!album.allow_download" class="small muted" style="padding:8px 10px">{{ t('download_off') }}</div>
          <button :disabled="!album.allow_download || !favs.length" @click="dl('fav')"><Icon name="heart" :size="18" />{{ t('download_fav', { n: favs.length }) }}</button>
          <button :disabled="!album.allow_download || !edits.length" @click="dl('edit')"><Icon name="edit" :size="18" />{{ t('download_edit', { n: edits.length }) }}</button>
          <button v-if="finals.length" :disabled="!album.allow_download" @click="dl('final')"><Icon name="star" :size="18" />{{ t('download_final', { n: finals.length }) }}</button>
          <button :disabled="!album.allow_download" @click="dl('all')"><Icon name="images" :size="18" />{{ t('download_all', { n: origs.length }) }}</button>
        </div>
      </template>
    </div>
    <template #menu="{ close }">
      <button v-if="album" @click="openNote(); close()"><Icon name="note" :size="18" />{{ t('general_note') }}</button>
    </template>
  </TopBar>

  <div v-if="error" class="cw"><div class="alert err">{{ error }}</div></div>
  <div v-else-if="!data" class="empty"><div class="spinner"></div></div>
  <template v-else>
    <div class="cw head">
      <div class="row wrap">
        <span class="badge" :class="album.status">{{ t('status_' + album.status) }}</span>
        <span class="small muted">{{ t('photos', { n: origs.length }) }}</span>
        <span v-if="album.expires_at" class="small muted">· {{ t('expires_on', { d: fmtDate(album.expires_at, false, lang) }) }}</span>
      </div>
      <div v-if="album.note_customer" class="alert info"><Icon name="info" :size="18" /><div style="white-space:pre-wrap">{{ album.note_customer }}</div></div>
      <div v-if="locked && album.status !== 'delivered'" class="alert warn"><Icon name="lock" :size="18" />{{ t('locked_banner') }}</div>
      <div v-if="finals.length && tab !== 'final'" class="alert ok" style="cursor:pointer" @click="tab = 'final'"><Icon name="star" :size="18" />{{ t('final_ready') }}</div>
    </div>

    <nav class="tabs">
      <div class="chips cw-in">
        <button class="chip" :class="{ on: tab === 'all' }" @click="tab = 'all'">{{ t('tab_all') }} <span class="n">{{ origs.length }}</span></button>
        <button class="chip" :class="{ on: tab === 'fav' }" @click="tab = 'fav'"><Icon name="heart" :size="15" :fill="tab !== 'fav'" style="color:var(--heart)" />{{ t('tab_fav') }} <span class="n">{{ favs.length }}</span></button>
        <button class="chip" :class="{ on: tab === 'edit' }" @click="tab = 'edit'"><Icon name="edit" :size="15" />{{ t('tab_edit') }} <span class="n">{{ edits.length }}<template v-if="maxEdit">/{{ maxEdit }}</template></span></button>
        <button v-if="finals.length" class="chip" :class="{ on: tab === 'final' }" @click="tab = 'final'"><Icon name="star" :size="15" />{{ t('tab_final') }} <span class="n">{{ finals.length }}</span></button>
        <button v-if="tab === 'all'" class="chip" :class="{ on: unseenOnly }" @click="unseenOnly = !unseenOnly"><Icon name="eye" :size="15" />{{ t('unseen') }}</button>
      </div>
    </nav>

    <div class="cw hint-row" v-if="!locked && tab !== 'final'">
      <span v-if="tab === 'all'">{{ t('fav_hint') }}</span>
      <span v-else-if="maxEdit">{{ t('edit_hint', { n: maxEdit }) }}</span>
      <span v-else>{{ t('edit_hint_free') }}</span>
    </div>

    <main class="gridwrap">
      <div v-if="!shown.length" class="empty">
        <Icon :name="tab === 'fav' ? 'heart' : tab === 'edit' ? 'edit' : 'images'" :size="36" />
        {{ tab === 'fav' ? t('empty_fav') : tab === 'edit' ? t('empty_edit') : unseenOnly ? t('empty_unseen') : t('empty_all') }}
      </div>
      <PhotoGrid v-else :photos="shown" :mode="tab === 'final' ? 'guest' : 'customer'" :locked="locked" :show-new="tab === 'all'"
        :edit-index="editIndex" :edit-toggle="tab === 'fav' || tab === 'edit'" @open="openViewer" @heart="heart" @edit="toggleEdit" />
    </main>

    <!-- sticky selection bar -->
    <div v-if="!locked && (edits.length || tab !== 'all') && tab !== 'final'" class="selbar">
      <div class="grow">
        <div class="small" style="font-weight:600">{{ maxEdit ? t('edit_counter', { x: edits.length, n: maxEdit }) : t('edit_counter_free', { x: edits.length }) }}</div>
        <div v-if="maxEdit" class="progress" :class="{ full: edits.length >= maxEdit }"><div :style="{ width: Math.min(100, edits.length / maxEdit * 100) + '%' }"></div></div>
      </div>
      <button class="btn primary" :disabled="!edits.length" @click="confirmOpen = true"><Icon name="send2" :size="18" />{{ t('submit') }}</button>
    </div>
  </template>

  <!-- viewer -->
  <Viewer v-if="viewIdx >= 0 && viewPhoto" :photos="viewList" v-model:index="viewIdx" :panel-open="panelOpen" @close="closeViewer" @hotkey="onHotkey">
    <template #actions="{ photo }">
      <template v-if="photo.kind === 'orig'">
        <button class="vbtn" :class="{ 'on-heart': photo.fav }" :disabled="locked" @click="heart(photo)"><Icon name="heart" :fill="photo.fav" :size="18" /><span class="hidden-sm">{{ t('heart') }}</span></button>
        <button class="vbtn" :class="{ 'on-edit': photo.edit }" :disabled="locked" @click="toggleEdit(photo)">
          <Icon :name="photo.edit ? 'check' : 'edit'" :size="18" />{{ photo.edit ? t('picked_edit') : t('pick_edit') }}
          <template v-if="maxEdit">({{ edits.length }}/{{ maxEdit }})</template>
        </button>
      </template>
      <button class="vbtn" :class="{ 'on-edit': panelOpen }" @click="togglePanel"><Icon name="comment" :size="18" /><span class="hidden-sm">{{ t('comment') }}</span><span v-if="photo.cm" class="dot" style="background:#fff;color:#000">{{ photo.cm }}</span></button>
      <button v-if="album.allow_download" class="vbtn" @click="dlPhoto(photo)" :aria-label="t('download_photo')"><Icon name="download" :size="18" /></button>
    </template>
    <template #panel="{ photo }">
      <div class="panel-head row">
        <h3 class="grow ellipsis">{{ t('comment') }} · {{ photo.name }}</h3>
        <button class="icon-btn" @click="panelOpen = false"><Icon name="close" /></button>
      </div>
      <ChatPanel :comments="photoComments" :loading="!comments" :show-photo-ref="false" :lang="lang"
        :placeholder="t('photo_note_ph')" :empty-text="t('photo_note_empty')" @send="(b) => sendComment(b, photo.id)" />
    </template>
  </Viewer>

  <!-- chat drawer -->
  <template v-if="chatOpen">
    <div class="drawer-overlay" @click="chatOpen = false"></div>
    <aside class="drawer">
      <div class="panel-head row">
        <h3 class="grow">{{ t('chat') }}</h3>
        <button class="icon-btn" @click="chatOpen = false"><Icon name="close" /></button>
      </div>
      <ChatPanel :comments="comments || []" :loading="!comments" :lang="lang" :placeholder="t('chat_ph')" :empty-text="t('chat_empty')"
        @send="(b) => sendComment(b)" @open-photo="openPhotoById" />
    </aside>
  </template>

  <Modal v-if="noteOpen" :title="t('general_note')" @close="noteOpen = false">
    <p class="hint" style="margin:0">{{ t('general_note_hint') }}</p>
    <textarea class="input" v-model="noteText" rows="6"></textarea>
    <template #foot>
      <button class="btn" @click="noteOpen = false">{{ t('cancel') }}</button>
      <button class="btn primary" @click="saveNote">{{ t('save') }}</button>
    </template>
  </Modal>

  <Modal v-if="confirmOpen" :title="t('submit_confirm_title')" @close="confirmOpen = false">
    <p style="margin:0">{{ t('submit_confirm', { x: edits.length }) }}</p>
    <div v-if="maxEdit && edits.length < maxEdit" class="alert warn">{{ t('submit_warn_less', { x: edits.length, n: maxEdit }) }}</div>
    <template #foot>
      <button class="btn" @click="confirmOpen = false">{{ t('cancel') }}</button>
      <button class="btn primary" @click="submit"><Icon name="send2" :size="18" />{{ t('submit') }}</button>
    </template>
  </Modal>
</template>

<style scoped>
.cw { max-width: 1400px; margin: 0 auto; padding: 0 16px; }
.head { padding-top: 14px; display: flex; flex-direction: column; gap: 10px; }
.tabs { position: sticky; top: 56px; z-index: 30; background: rgba(246,245,243,.94); backdrop-filter: blur(8px); padding: 10px 0; }
.cw-in { max-width: 1400px; margin: 0 auto; padding: 0 16px; }
.hint-row { font-size: 13px; color: var(--muted); padding-bottom: 10px; }
.gridwrap { max-width: 1400px; margin: 0 auto; padding: 0 0 110px; }
@media (min-width: 640px) { .gridwrap { padding: 0 16px 110px; } }
.pick-list-hint { text-align: center; padding: 14px; }
.selbar { position: fixed; left: 0; right: 0; bottom: 0; z-index: 35; display: flex; align-items: center; gap: 14px;
  padding: 12px 16px; padding-bottom: max(12px, env(safe-area-inset-bottom)); background: var(--surface); border-top: 1px solid var(--border); box-shadow: 0 -6px 20px rgba(0,0,0,.06); }
.selbar .progress { margin-top: 6px; max-width: 360px; }
.panel-head { padding: 10px 8px 10px 16px; border-bottom: 1px solid var(--border); }
.menu-catch { position: fixed; inset: 0; z-index: 50; }
</style>
