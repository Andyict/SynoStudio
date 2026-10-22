<script setup>
import { ref, computed, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import Icon from '../shared/Icon.vue'
import Toasts from '../shared/Toasts.vue'
import Modal from '../shared/Modal.vue'
import { post } from '../shared/api.js'
import { toast, toastErr } from '../shared/ui.js'
import { fmtTime } from '../shared/fmt.js'
import { state, can, stopRealtime } from './store.js'

const route = useRoute()
const router = useRouter()
const sideOpen = ref(false)
const notifOpen = ref(false)
const userMenu = ref(false)
const pwOpen = ref(false)
const pw = ref({ old: '', new: '' })

const isLogin = computed(() => route.path === '/login')
watch(() => route.fullPath, () => { sideOpen.value = false; notifOpen.value = false })

const nav = computed(() => [
  { to: '/', icon: 'home', label: 'Tổng quan', exact: true },
  { to: '/albums', icon: 'images', label: 'Album' },
  { to: '/customers', icon: 'users', label: 'Khách hàng' },
  can('staff.manage') || state.me?.owner ? { to: '/staff', icon: 'shield', label: 'Nhân viên & quyền' } : null,
  { to: '/settings', icon: 'settings', label: 'Cài đặt' },
].filter(Boolean))

function isActive(n) { return n.exact ? route.path === n.to : route.path.startsWith(n.to) }

const notifIcon = { submitted: 'send2', message: 'chat', note: 'note' }
function openNotif() {
  notifOpen.value = !notifOpen.value
  if (notifOpen.value && state.notif.unseen) {
    post('/api/s/notifications/seen').catch(() => {})
    state.notif.unseen = 0
  }
  if ('Notification' in window && Notification.permission === 'default') Notification.requestPermission().catch(() => {})
}
function goNotif(n) {
  notifOpen.value = false
  n.new = false
  router.push(`/albums/${n.album_id}` + (n.kind === 'message' ? '?tab=chat' : ''))
}

async function logout() {
  await post('/api/s/logout').catch(() => {})
  state.me = null
  stopRealtime()
  router.replace('/login')
}
async function changePw() {
  try {
    await post('/api/s/password', pw.value)
    pwOpen.value = false
    pw.value = { old: '', new: '' }
    toast('Đã đổi mật khẩu')
  } catch (e) { toastErr(e) }
}
</script>

<template>
  <router-view v-if="isLogin" />
  <div v-else class="shell">
    <aside class="side" :class="{ open: sideOpen }">
      <div class="brand">
        <img v-if="state.branding?.logo" :src="'/branding/logo'" alt="" />
        <span v-else class="mark"><Icon name="image" /></span>
        <span class="ellipsis">{{ state.branding?.name || 'Studio' }}</span>
      </div>
      <nav>
        <router-link v-for="n in nav" :key="n.to" :to="n.to" :class="{ active: isActive(n) }"><Icon :name="n.icon" />{{ n.label }}</router-link>
      </nav>
      <div class="me" v-if="state.me">
        <div style="color:#fff;font-weight:600">{{ state.me.name }}</div>
        <div class="small" style="opacity:.6">@{{ state.me.username }}{{ state.me.owner ? ' · Chủ' : '' }}</div>
        <div class="small" style="opacity:.4;margin-top:6px">Studio Album v{{ state.branding?.version }}</div>
      </div>
    </aside>
    <div v-if="sideOpen" class="drawer-overlay" style="z-index:94" @click="sideOpen = false"></div>
    <div class="main">
      <header class="topbar-s">
        <button class="icon-btn mobile-only" @click="sideOpen = true"><Icon name="menu" /></button>
        <div class="spacer"></div>
        <a class="btn ghost sm hidden-sm" href="/" target="_blank"><Icon name="eye" :size="16" />Trang khách</a>
        <div class="menu-wrap">
          <button class="icon-btn" @click="openNotif" aria-label="Thông báo"><Icon name="bell" /><span v-if="state.notif.unseen" class="dot">{{ state.notif.unseen }}</span></button>
          <template v-if="notifOpen">
            <div class="menu-catch" @click="notifOpen = false"></div>
            <div class="notif-panel">
              <div class="section-title"><h3 class="grow">Thông báo</h3></div>
              <div v-if="!state.notif.items.length" class="empty small">Chưa có thông báo</div>
              <div v-for="n in state.notif.items" :key="n.id" class="notif-item" :class="{ new: n.new }" @click="goNotif(n)">
                <span class="ic"><Icon :name="notifIcon[n.kind] || 'bell'" :size="16" /></span>
                <div class="grow"><div>{{ n.text }}</div><div class="small muted">{{ fmtTime(n.created_at) }}</div></div>
              </div>
            </div>
          </template>
        </div>
        <div class="menu-wrap">
          <button class="icon-btn" @click="userMenu = !userMenu"><Icon name="user" /></button>
          <template v-if="userMenu">
            <div class="menu-catch" @click="userMenu = false"></div>
            <div class="menu" style="z-index:71" @click="userMenu = false">
              <div class="small muted" style="padding:6px 10px">{{ state.me?.name }}</div>
              <button @click="pwOpen = true"><Icon name="key" :size="18" />Đổi mật khẩu</button>
              <button @click="logout"><Icon name="logout" :size="18" />Đăng xuất</button>
            </div>
          </template>
        </div>
      </header>
      <router-view :key="route.path" />
    </div>
  </div>
  <Modal v-if="pwOpen" title="Đổi mật khẩu" @close="pwOpen = false">
    <div class="field"><label>Mật khẩu hiện tại</label><input class="input" type="password" v-model="pw.old" /></div>
    <div class="field"><label>Mật khẩu mới (≥ 6 ký tự)</label><input class="input" type="password" v-model="pw.new" /></div>
    <template #foot><button class="btn" @click="pwOpen = false">Hủy</button><button class="btn primary" @click="changePw">Lưu</button></template>
  </Modal>
  <Toasts />
</template>

<style>
.menu-catch { position: fixed; inset: 0; z-index: 50; }
</style>
