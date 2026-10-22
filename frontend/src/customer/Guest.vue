<script setup>
// Public view-only album (share link). No hearts, picks or chat.
import { ref, computed, onMounted } from 'vue'
import { get } from '../shared/api.js'
import { download } from '../shared/fmt.js'
import Icon from '../shared/Icon.vue'
import PhotoGrid from '../shared/PhotoGrid.vue'
import Viewer from '../shared/Viewer.vue'
import TopBar from './TopBar.vue'
import { t } from './i18n.js'
import { toast } from '../shared/ui.js'

const props = defineProps({ token: String })
const data = ref(null)
const error = ref('')
const tab = ref('all')
const viewIdx = ref(-1)

const origs = computed(() => (data.value?.photos || []).filter((p) => p.kind === 'orig'))
const finals = computed(() => (data.value?.photos || []).filter((p) => p.kind === 'final'))
const shown = computed(() => (tab.value === 'final' ? finals.value : origs.value))

onMounted(async () => {
  try {
    data.value = await get(`/api/g/${encodeURIComponent(props.token)}`)
    if (finals.value.length && data.value.album.status === 'delivered') tab.value = 'final'
  } catch { error.value = t('guest_invalid') }
})

function dl(set) {
  download(`/dl/g/${encodeURIComponent(props.token)}?set=${set}`)
  toast(t('download_started'))
}
</script>

<template>
  <TopBar guest :title="data?.album.title || t('guest_title')">
    <div v-if="data?.album.allow_download" class="menu-wrap">
      <button class="btn sm" @click="dl(tab)"><Icon name="download" :size="16" />{{ tab === 'final' ? t('download_final', { n: finals.length }) : t('download_all', { n: origs.length }) }}</button>
    </div>
  </TopBar>
  <div v-if="error" class="empty"><Icon name="link" :size="36" />{{ error }}</div>
  <div v-else-if="!data" class="empty"><div class="spinner"></div></div>
  <template v-else>
    <div class="gw">
      <div v-if="data.album.note_customer" class="alert info" style="margin-bottom:10px">{{ data.album.note_customer }}</div>
      <div v-if="finals.length" class="chips" style="margin-bottom:10px">
        <button class="chip" :class="{ on: tab === 'all' }" @click="tab = 'all'">{{ t('tab_all') }} <span class="n">{{ origs.length }}</span></button>
        <button class="chip" :class="{ on: tab === 'final' }" @click="tab = 'final'"><Icon name="star" :size="15" />{{ t('tab_final') }} <span class="n">{{ finals.length }}</span></button>
      </div>
    </div>
    <main class="gridwrap">
      <PhotoGrid :photos="shown" mode="guest" :guest="token" @open="(i) => (viewIdx = i)" />
    </main>
  </template>
  <Viewer v-if="viewIdx >= 0" :photos="shown" v-model:index="viewIdx" :guest="token" @close="viewIdx = -1">
    <template #actions="{ photo }">
      <button v-if="data.album.allow_download" class="vbtn" @click="download(`/img/${photo.id}/o?dl=1&g=${encodeURIComponent(token)}`)"><Icon name="download" :size="18" />{{ t('download_photo') }}</button>
    </template>
  </Viewer>
</template>

<style scoped>
.gw { max-width: 1400px; margin: 0 auto; padding: 14px 16px 0; }
.gridwrap { max-width: 1400px; margin: 0 auto; padding: 0 0 40px; }
@media (min-width: 640px) { .gridwrap { padding: 0 16px 40px; } }
</style>
