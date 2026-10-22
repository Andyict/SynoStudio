<script setup>
// Customer header: brand/back button, optional title, slot for actions, account menu.
import { ref } from 'vue'
import { useRouter } from 'vue-router'
import Icon from '../shared/Icon.vue'
import Modal from '../shared/Modal.vue'
import { post } from '../shared/api.js'
import { toast } from '../shared/ui.js'
import { state } from './store.js'
import { t, lang, setLang, errText } from './i18n.js'

defineProps({ title: String, back: Boolean, guest: Boolean })
const router = useRouter()
const menu = ref(false)
const pwOpen = ref(false)
const pw = ref({ old: '', new: '' })
const pwErr = ref('')

function toggleLang() {
  const l = lang.value === 'vi' ? 'en' : 'vi'
  setLang(l)
  if (state.me) post('/api/c/lang', { lang: l }).catch(() => {})
  menu.value = false
}
async function logout() {
  await post('/api/c/logout').catch(() => {})
  state.me = null
  router.replace('/login')
}
async function changePw() {
  pwErr.value = ''
  try {
    await post('/api/c/password', pw.value)
    pwOpen.value = false
    pw.value = { old: '', new: '' }
    toast(t('password_changed'))
  } catch (e) { pwErr.value = errText(e) }
}
</script>

<template>
  <header class="topbar">
    <button v-if="back" class="icon-btn" @click="router.push('/')" :aria-label="t('back')"><Icon name="back" /></button>
    <div v-else class="brandmini">
      <img v-if="state.branding?.logo" :src="'/branding/logo'" alt="" />
      <span v-else>{{ state.branding?.name }}</span>
    </div>
    <div class="grow tb-title ellipsis">{{ title }}</div>
    <slot />
    <div class="menu-wrap">
      <button class="icon-btn" @click="menu = !menu" aria-label="Menu"><Icon name="more" /></button>
      <div v-if="menu" class="menu" @click.stop>
        <slot name="menu" :close="() => (menu = false)" />
        <button @click="toggleLang"><Icon name="globe" :size="18" />{{ t('language') }}</button>
        <template v-if="!guest && state.me">
          <button @click="pwOpen = true; menu = false"><Icon name="key" :size="18" />{{ t('change_password') }}</button>
          <hr />
          <div class="small muted" style="padding:4px 10px">{{ state.me.name || state.me.phone }}</div>
          <button @click="logout"><Icon name="logout" :size="18" />{{ t('logout') }}</button>
        </template>
      </div>
      <div v-if="menu" class="menu-catch" @click="menu = false"></div>
    </div>
  </header>
  <Modal v-if="pwOpen" :title="t('change_password')" @close="pwOpen = false">
    <div class="field"><label>{{ t('old_password') }}</label><input class="input" type="password" v-model="pw.old" autocomplete="current-password" /></div>
    <div class="field"><label>{{ t('new_password') }}</label><input class="input" type="password" v-model="pw.new" autocomplete="new-password" /></div>
    <div v-if="pwErr" class="alert err">{{ pwErr }}</div>
    <template #foot>
      <button class="btn" @click="pwOpen = false">{{ t('cancel') }}</button>
      <button class="btn primary" @click="changePw">{{ t('save') }}</button>
    </template>
  </Modal>
</template>

<style scoped>
.topbar { position: sticky; top: 0; z-index: 40; display: flex; align-items: center; gap: 4px; height: 56px; padding: 0 8px 0 12px;
  background: rgba(255,255,255,.92); backdrop-filter: blur(10px); border-bottom: 1px solid var(--border); }
.brandmini { display: flex; align-items: center; font-weight: 700; font-size: 17px; margin-right: 6px; }
.brandmini img { height: 34px; max-width: 120px; object-fit: contain; }
.tb-title { font-weight: 650; font-size: 16px; }
.menu-catch { position: fixed; inset: 0; z-index: 50; }
.menu { z-index: 61; }
</style>
