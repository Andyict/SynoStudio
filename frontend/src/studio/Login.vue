<script setup>
import { ref } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { post } from '../shared/api.js'
import Icon from '../shared/Icon.vue'
import { state, loadMe } from './store.js'

const router = useRouter()
const route = useRoute()
const username = ref('')
const password = ref('')
const error = ref('')
const busy = ref(false)

async function submit() {
  error.value = ''
  busy.value = true
  try {
    await post('/api/s/login', { username: username.value, password: password.value })
    await loadMe()
    const next = route.query.next
    router.replace(typeof next === 'string' && next.startsWith('/') && next !== '/login' ? next : '/')
  } catch (e) {
    error.value = e.message
  } finally {
    busy.value = false
  }
}
</script>

<template>
  <div class="slogin">
    <form class="card stack" @submit.prevent="submit">
      <div class="row">
        <span class="mark"><Icon name="image" /></span>
        <div>
          <h2>{{ state.branding?.name || 'Studio' }}</h2>
          <div class="small muted">Đăng nhập khu vực nhân viên</div>
        </div>
      </div>
      <div class="field"><label>Tên đăng nhập</label><input class="input" v-model="username" autocomplete="username" required autofocus /></div>
      <div class="field"><label>Mật khẩu</label><input class="input" type="password" v-model="password" autocomplete="current-password" required /></div>
      <div v-if="error" class="alert err">{{ error }}</div>
      <button class="btn primary block" :disabled="busy">Đăng nhập</button>
      <p class="hint" style="margin:0">Chưa có tài khoản quản trị? Quản trị viên NAS tạo trong <b>Studio Album Console</b> (menu chính DSM).</p>
    </form>
  </div>
</template>

<style scoped>
.slogin { min-height: 100vh; display: flex; align-items: center; justify-content: center; padding: 16px; background: #1f1d1b; }
.slogin .card { width: 100%; max-width: 380px; padding: 24px; border-radius: 16px; }
.mark { width: 42px; height: 42px; border-radius: 12px; background: var(--brand); color: var(--brand-ink); display: flex; align-items: center; justify-content: center; }
</style>
