<script setup>
// Browse enabled share folders and pick the album folder.
import { ref, onMounted } from 'vue'
import { get } from '../shared/api.js'
import Icon from '../shared/Icon.vue'

const emit = defineEmits(['pick'])
const shares = ref(null)
const share = ref('')
const cur = ref(null) // browse result
const error = ref('')
const loading = ref(false)

onMounted(async () => {
  try {
    shares.value = await get('/api/s/shares')
    if (shares.value?.length === 1) open(shares.value[0].name, '')
  } catch (e) { error.value = e.message }
})

async function open(s, path) {
  error.value = ''
  loading.value = true
  share.value = s
  try { cur.value = await get(`/api/s/browse?share=${encodeURIComponent(s)}&path=${encodeURIComponent(path)}`) }
  catch (e) { error.value = e.message }
  finally { loading.value = false }
}
function crumbs() {
  if (!cur.value) return []
  const parts = cur.value.path ? cur.value.path.split('/') : []
  return parts.map((p, i) => ({ name: p, path: parts.slice(0, i + 1).join('/') }))
}
function pick() {
  const name = cur.value.path.split('/').pop() || share.value
  emit('pick', { share: share.value, path: cur.value.path, name, images: cur.value.images })
}
</script>

<template>
  <div class="picker">
    <div v-if="shares && !shares.length" class="alert warn" style="margin:10px">
      Chưa có share folder nào được bật. Quản trị viên NAS cần bật share folder trong <b>Studio Album Console</b>.
    </div>
    <template v-else>
      <div class="crumbs">
        <Icon name="folder" :size="16" class="muted" />
        <template v-if="shares && shares.length > 1">
          <button @click="cur = null; share = ''">Share</button><span class="muted">/</span>
        </template>
        <template v-if="share">
          <button @click="open(share, '')">{{ share }}</button>
          <template v-for="c in crumbs()" :key="c.path"><span class="muted">/</span><button @click="open(share, c.path)">{{ c.name }}</button></template>
        </template>
      </div>
      <div class="dirs">
        <div v-if="loading || !shares" class="empty small"><div class="spinner"></div></div>
        <template v-else-if="!share">
          <div v-for="s in shares" :key="s.name" class="dir" @click="open(s.name, '')">
            <Icon name="database" :size="18" class="muted" /><span class="grow">{{ s.name }}</span>
            <span v-if="!s.readable" class="badge danger">Không có quyền đọc</span>
          </div>
        </template>
        <template v-else-if="cur">
          <div v-for="d in cur.dirs" :key="d.path" class="dir" @click="open(share, d.path)">
            <Icon name="folder" :size="18" style="color:#d9a441" /><span class="grow ellipsis">{{ d.name }}</span><Icon name="right" :size="16" class="muted" />
          </div>
          <div v-if="!cur.dirs.length" class="empty small">Không có thư mục con</div>
        </template>
      </div>
      <div v-if="error" class="alert err" style="margin:8px">{{ error }}</div>
      <div v-if="cur" class="foot">
        <span class="grow small">
          <b>{{ cur.images }}</b> ảnh trong thư mục này
          <span v-if="cur.album_id" class="badge danger">Đã dùng cho album #{{ cur.album_id }}</span>
        </span>
        <button class="btn primary sm" :disabled="!cur.path || !!cur.album_id" @click="pick">Chọn thư mục này</button>
      </div>
    </template>
  </div>
</template>
