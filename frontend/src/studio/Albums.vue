<script setup>
import { ref, watch, onMounted, onBeforeUnmount } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { get } from '../shared/api.js'
import { toastErr } from '../shared/ui.js'
import Icon from '../shared/Icon.vue'
import AlbumRow from './AlbumRow.vue'
import AlbumForm from './AlbumForm.vue'
import { statusLabel, can, onStaffEvent } from './store.js'

const route = useRoute()
const router = useRouter()
const list = ref(null)
const q = ref(route.query.q || '')
const status = ref(route.query.status || '')
const creating = ref(route.query.new === '1')

async function load() {
  try {
    const p = new URLSearchParams()
    if (q.value) p.set('q', q.value)
    if (status.value) p.set('status', status.value)
    list.value = await get('/api/s/albums?' + p)
  } catch (e) { toastErr(e) }
}
let timer = null
watch(q, () => { clearTimeout(timer); timer = setTimeout(load, 300) })
watch(status, () => {
  router.replace({ query: { ...route.query, status: status.value || undefined } })
  load()
})
const off = onStaffEvent((ev) => { if (ev.type === 'status' || ev.type === 'notification') { clearTimeout(timer); timer = setTimeout(load, 600) } })
onMounted(load)
onBeforeUnmount(off)

function onSaved(id) {
  creating.value = false
  router.push(`/albums/${id}`)
}
</script>

<template>
  <div class="page">
    <div class="page-head">
      <h1>Album</h1>
      <button v-if="can('albums.manage')" class="btn primary" @click="creating = true"><Icon name="plus" :size="18" />Tạo album</button>
    </div>
    <div class="row wrap">
      <div class="grow" style="min-width:220px;position:relative">
        <input class="input" v-model="q" placeholder="Tìm theo tên album, thư mục, tên/SĐT khách…" style="padding-left:36px" />
        <Icon name="search" :size="18" class="muted" style="position:absolute;left:11px;top:11px" />
      </div>
      <div class="chips">
        <button class="chip" :class="{ on: !status }" @click="status = ''">Tất cả</button>
        <button v-for="(l, s) in statusLabel" :key="s" class="chip" :class="{ on: status === s }" @click="status = s">{{ l }}</button>
      </div>
    </div>
    <div class="card pad0">
      <div v-if="!list" class="empty"><div class="spinner"></div></div>
      <div v-else-if="!list.length" class="empty"><Icon name="images" :size="36" />Không có album nào</div>
      <div v-else class="alist"><AlbumRow v-for="a in list" :key="a.id" :a="a" /></div>
    </div>
  </div>
  <AlbumForm v-if="creating" @close="creating = false; router.replace({ query: { ...route.query, new: undefined } })" @saved="onSaved" />
</template>
