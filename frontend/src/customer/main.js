import { createApp } from 'vue'
import { createRouter, createWebHistory } from 'vue-router'
import '../shared/base.css'
import App from './App.vue'
import Login from './Login.vue'
import Albums from './Albums.vue'
import Album from './Album.vue'
import Guest from './Guest.vue'
import { get, setUnauthorizedHandler } from '../shared/api.js'
import { applyBranding } from '../shared/ui.js'
import { setLang, lang, hasLangPref } from './i18n.js'

import { state } from './store.js'

const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/login', component: Login, meta: { public: true } },
    { path: '/', component: Albums },
    { path: '/a/:id', component: Album, props: (r) => ({ id: Number(r.params.id) }) },
    { path: '/s/:token', component: Guest, props: true, meta: { public: true } },
    { path: '/:p(.*)*', redirect: '/' },
  ],
  scrollBehavior: (to, from, saved) => saved || { top: 0 },
})

router.beforeEach(async (to) => {
  if (to.meta.public) return true
  if (!state.me) {
    try {
      state.me = await get('/api/c/me')
      if (state.me.lang && !hasLangPref()) setLang(state.me.lang)
    } catch {
      return { path: '/login', query: to.fullPath !== '/' ? { next: to.fullPath } : {} }
    }
  }
  return true
})

setUnauthorizedHandler(() => {
  state.me = null
  if (!location.pathname.startsWith('/s/')) router.replace('/login')
})

get('/api/public/branding').then((b) => {
  state.branding = b
  applyBranding(b)
  document.title = b.name
  if (!hasLangPref() && b.lang) setLang(b.lang)
}).catch(() => {})
document.documentElement.lang = lang.value

if ('serviceWorker' in navigator && location.protocol === 'https:') {
  navigator.serviceWorker.register('/sw.js').catch(() => {})
}

createApp(App).use(router).mount('#app')
