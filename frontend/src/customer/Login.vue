<script setup>
import { ref } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { post, get } from '../shared/api.js'
import Icon from '../shared/Icon.vue'
import { state } from './store.js'
import { t, lang, setLang, errText } from './i18n.js'

const router = useRouter()
const route = useRoute()
const phone = ref('')
const password = ref('')
const show = ref(false)
const busy = ref(false)
const error = ref('')

async function submit() {
  error.value = ''
  busy.value = true
  try {
    await post('/api/c/login', { phone: phone.value, password: password.value })
    state.me = await get('/api/c/me')
    const next = route.query.next
    router.replace(typeof next === 'string' && next.startsWith('/') ? next : '/')
  } catch (e) {
    error.value = errText(e)
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <div class="login-page">
    <div class="login-card">
      <div class="brand">
        <img v-if="state.branding?.logo" :src="'/branding/logo'" alt="" class="logo" />
        <div v-else class="logo-ph"><Icon name="image" :size="30" /></div>
        <h1>{{ state.branding?.name || 'Studio' }}</h1>
        <p class="muted" v-if="state.branding?.welcome">{{ state.branding.welcome }}</p>
      </div>
      <form class="stack" @submit.prevent="submit">
        <h2>{{ t('login_title') }}</h2>
        <div class="field">
          <label for="phone">{{ t('phone') }}</label>
          <input id="phone" class="input lg" v-model="phone" type="tel" inputmode="tel" autocomplete="username" required placeholder="09xx xxx xxx" />
        </div>
        <div class="field">
          <label for="pw">{{ t('password') }}</label>
          <div class="pw">
            <input id="pw" class="input lg" v-model="password" :type="show ? 'text' : 'password'" autocomplete="current-password" required />
            <button type="button" class="icon-btn" @click="show = !show" :aria-label="show ? 'Hide' : 'Show'"><Icon :name="show ? 'eyeoff' : 'eye'" /></button>
          </div>
        </div>
        <div v-if="error" class="alert err">{{ error }}</div>
        <button class="btn primary lg block" :disabled="busy">{{ t('login') }}</button>
        <p class="hint center">{{ t('login_hint') }}</p>
      </form>
      <div class="foot row">
        <span v-if="state.branding?.phone" class="small muted">{{ t('contact', { p: state.branding.phone }) }}</span>
        <span class="spacer"></span>
        <button class="btn ghost sm" @click="setLang(lang === 'vi' ? 'en' : 'vi')"><Icon name="globe" :size="16" />{{ t('language') }}</button>
      </div>
    </div>
  </div>
</template>

<style scoped>
.login-page { min-height: 100vh; min-height: 100dvh; display: flex; align-items: center; justify-content: center; padding: 16px;
  background: radial-gradient(1200px 600px at 10% -10%, var(--brand-soft), transparent), var(--bg); }
.login-card { width: 100%; max-width: 400px; background: var(--surface); border-radius: 20px; padding: 28px 24px 16px; box-shadow: var(--shadow); display: flex; flex-direction: column; gap: 22px; }
.brand { display: flex; flex-direction: column; align-items: center; gap: 8px; text-align: center; }
.brand p { margin: 0; font-size: 14px; }
.logo { max-height: 72px; max-width: 200px; object-fit: contain; }
.logo-ph { width: 60px; height: 60px; border-radius: 18px; background: var(--brand); color: var(--brand-ink); display: flex; align-items: center; justify-content: center; }
.input.lg { height: 48px; font-size: 16px; }
.pw { position: relative; }
.pw .icon-btn { position: absolute; right: 5px; top: 5px; color: var(--muted); }
.foot { border-top: 1px solid var(--border); padding-top: 10px; }
</style>
