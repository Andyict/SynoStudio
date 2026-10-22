<script setup>
import { ref, onMounted } from 'vue'
import { get } from '../shared/api.js'
import Icon from '../shared/Icon.vue'
import TopBar from './TopBar.vue'
import { imgUrl, fmtDate } from '../shared/fmt.js'
import { t, lang, errText } from './i18n.js'
import { state } from './store.js'

const list = ref(null)
const error = ref('')

async function load() {
  error.value = ''
  try { list.value = await get('/api/c/albums') } catch (e) { error.value = errText(e) }
}
onMounted(load)
</script>

<template>
  <TopBar :title="t('my_albums')" />
  <main class="cw">
    <div v-if="state.me?.name" class="hello">{{ state.me.name }}</div>
    <div v-if="error" class="alert err">{{ error }} <button class="btn sm" @click="load">{{ t('retry') }}</button></div>
    <div v-else-if="!list" class="empty"><div class="spinner"></div></div>
    <div v-else-if="!list.length" class="empty"><Icon name="images" :size="40" />{{ t('no_albums') }}</div>
    <div v-else class="albums">
      <router-link v-for="it in list" :key="it.album.id" :to="`/a/${it.album.id}`" class="acard" :class="{ off: it.expired }">
        <div class="cover">
          <img v-if="it.album.cover" :src="imgUrl(it.album.cover, 's')" alt="" loading="lazy" />
          <span v-if="it.unread" class="dot unread">{{ it.unread }}</span>
        </div>
        <div class="info">
          <h3 class="ellipsis">{{ it.album.title }}</h3>
          <div class="row wrap small muted">
            <span>{{ t('photos', { n: it.album.photo_count }) }}</span>
            <span v-if="it.fav_count">· <Icon name="heart" :size="13" fill style="color:var(--heart)" /> {{ it.fav_count }}</span>
            <span v-if="it.edit_count">· <Icon name="edit" :size="13" /> {{ it.edit_count }}<template v-if="it.album.max_edit">/{{ it.album.max_edit }}</template></span>
          </div>
          <div class="row wrap">
            <span class="badge" :class="it.album.status">{{ t('status_' + it.album.status) }}</span>
            <span v-if="it.expired" class="badge danger">{{ t('expired') }}</span>
            <span v-else-if="it.album.expires_at" class="small muted">{{ t('expires_on', { d: fmtDate(it.album.expires_at, false, lang) }) }}</span>
          </div>
        </div>
      </router-link>
    </div>
  </main>
</template>

<style scoped>
.cw { max-width: 1100px; margin: 0 auto; padding: 16px; }
.hello { font-size: 20px; font-weight: 650; margin: 4px 0 14px; }
.albums { display: grid; grid-template-columns: repeat(auto-fill, minmax(260px, 1fr)); gap: 16px; }
.acard { background: var(--surface); border-radius: 16px; overflow: hidden; color: inherit; box-shadow: var(--shadow); display: flex; flex-direction: column; transition: transform .15s; }
.acard:hover { transform: translateY(-2px); }
.acard.off { opacity: .6; pointer-events: none; }
.cover { aspect-ratio: 3 / 2; background: #e9e5e1; position: relative; }
.cover img { width: 100%; height: 100%; object-fit: cover; }
.cover .unread { position: absolute; top: 10px; right: 10px; }
.info { padding: 12px 14px 14px; display: flex; flex-direction: column; gap: 6px; }
</style>
