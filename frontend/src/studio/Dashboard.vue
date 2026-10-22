<script setup>
import { ref, onMounted, onBeforeUnmount } from 'vue'
import { useRouter } from 'vue-router'
import { get } from '../shared/api.js'
import { toastErr } from '../shared/ui.js'
import Icon from '../shared/Icon.vue'
import AlbumRow from './AlbumRow.vue'
import { state, statusLabel, onStaffEvent, can } from './store.js'

const router = useRouter()
const d = ref(null)
async function load() {
  try { d.value = await get('/api/s/dashboard') } catch (e) { toastErr(e) }
}
let t = null
const off = onStaffEvent((ev) => {
  if (['notification', 'status', 'read', 'comment'].includes(ev.type)) { clearTimeout(t); t = setTimeout(load, 500) }
})
onMounted(load)
onBeforeUnmount(() => { off(); clearTimeout(t) })
</script>

<template>
  <div class="page">
    <div class="page-head">
      <h1>Xin chào, {{ state.me?.name }}</h1>
      <router-link v-if="can('albums.manage')" to="/albums?new=1" class="btn primary"><Icon name="plus" :size="18" />Tạo album</router-link>
    </div>
    <div v-if="!d" class="empty"><div class="spinner"></div></div>
    <template v-else>
      <div class="stats">
        <div v-for="(label, s) in statusLabel" :key="s" class="stat" @click="router.push(`/albums?status=${s}`)">
          <span class="badge" :class="s" style="align-self:flex-start">{{ label }}</span>
          <span class="n">{{ d.counts[s] || 0 }}</span>
        </div>
      </div>
      <div class="grid2" style="align-items:start">
        <div class="card pad0">
          <div class="section-title"><Icon name="send2" /><h3 class="grow">Khách vừa gửi ảnh chọn</h3><span class="badge">{{ d.submitted.length }}</span></div>
          <div v-if="!d.submitted.length" class="empty small">Không có album nào chờ xử lý</div>
          <div class="alist"><AlbumRow v-for="a in d.submitted" :key="a.id" :a="a" /></div>
        </div>
        <div class="card pad0">
          <div class="section-title"><Icon name="chat" /><h3 class="grow">Tin nhắn chưa đọc</h3><span class="badge">{{ d.unread.length }}</span></div>
          <div v-if="!d.unread.length" class="empty small">Đã đọc hết tin nhắn</div>
          <div class="alist"><AlbumRow v-for="a in d.unread" :key="a.id" :a="a" /></div>
        </div>
        <div class="card pad0">
          <div class="section-title"><Icon name="calendar" /><h3 class="grow">Sắp / quá hạn trả ảnh (3 ngày)</h3><span class="badge">{{ d.due.length }}</span></div>
          <div v-if="!d.due.length" class="empty small">Không có album nào sắp đến hạn</div>
          <div class="alist"><AlbumRow v-for="a in d.due" :key="a.id" :a="a" /></div>
        </div>
      </div>
    </template>
  </div>
</template>
