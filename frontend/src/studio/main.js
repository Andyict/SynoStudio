import { createApp } from 'vue'
import { createRouter, createWebHistory } from 'vue-router'
import '../shared/base.css'
import './studio.css'
import App from './App.vue'
import Login from './Login.vue'
import Dashboard from './Dashboard.vue'
import Albums from './Albums.vue'
import AlbumDetail from './AlbumDetail.vue'
import Customers from './Customers.vue'
import Staff from './Staff.vue'
import Settings from './Settings.vue'
import { get, setUnauthorizedHandler } from '../shared/api.js'
import { applyBranding } from '../shared/ui.js'
import { state, loadMe, stopRealtime } from './store.js'

const router = createRouter({
  history: createWebHistory('/studio/'),
  routes: [
    { path: '/login', component: Login, meta: { public: true } },
    { path: '/', component: Dashboard },
    { path: '/albums', component: Albums },
    { path: '/albums/:id', component: AlbumDetail, props: (r) => ({ id: Number(r.params.id) }) },
    { path: '/customers', component: Customers },
    { path: '/staff', component: Staff },
    { path: '/settings', component: Settings },
    { path: '/:p(.*)*', redirect: '/' },
  ],
})

router.beforeEach(async (to) => {
  if (to.meta.public) return true
  if (!state.me) {
    try { await loadMe() } catch { return { path: '/login', query: { next: to.fullPath } } }
  }
  return true
})

setUnauthorizedHandler(() => {
  state.me = null
  stopRealtime()
  router.replace('/login')
})

get('/api/public/branding').then((b) => { state.branding = b; applyBranding(b) }).catch(() => {})

createApp(App).use(router).mount('#app')
