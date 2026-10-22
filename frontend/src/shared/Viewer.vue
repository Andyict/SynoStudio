<script setup>
// Full-screen photo viewer: swipe / arrow keys, double-tap zoom, neighbour preloading.
// Slots: #actions="{ photo }" (bottom bar), #panel="{ photo }" (side panel, shown when panelOpen).
import { computed, ref, watch, onMounted, onBeforeUnmount, nextTick } from 'vue'
import Icon from './Icon.vue'
import { imgUrl } from './fmt.js'

const props = defineProps({
  photos: Array,
  index: Number,
  guest: String,
  panelOpen: Boolean,
})
const emit = defineEmits(['update:index', 'close', 'hotkey'])

const photo = computed(() => props.photos[props.index])
const zoomed = ref(false)
const loadedSrc = ref('')
const stage = ref(null)

const src = computed(() => photo.value && imgUrl(photo.value.id, 'm', props.guest))
const thumb = computed(() => photo.value && imgUrl(photo.value.id, 's', props.guest))

function go(d) {
  const n = props.index + d
  if (n < 0 || n >= props.photos.length) return
  zoomed.value = false
  emit('update:index', n)
}

function preload() {
  for (const d of [1, -1, 2]) {
    const p = props.photos[props.index + d]
    if (p) { const im = new Image(); im.src = imgUrl(p.id, 'm', props.guest) }
  }
}
watch(() => props.index, preload)

function onKey(e) {
  if (e.target.closest && e.target.closest('textarea, input')) return
  if (e.key === 'ArrowRight') go(1)
  else if (e.key === 'ArrowLeft') go(-1)
  else if (e.key === 'Escape') emit('close')
  else emit('hotkey', e.key)
}

// touch swipe
let sx = 0, sy = 0, st = 0
function tstart(e) { if (e.touches.length !== 1) return; sx = e.touches[0].clientX; sy = e.touches[0].clientY; st = Date.now() }
function tend(e) {
  if (zoomed.value || !st) return
  const t = e.changedTouches[0]
  const dx = t.clientX - sx, dy = t.clientY - sy
  if (Date.now() - st < 600 && Math.abs(dx) > 50 && Math.abs(dx) > Math.abs(dy) * 1.5) go(dx < 0 ? 1 : -1)
  else if (dy > 120 && Math.abs(dy) > Math.abs(dx) * 2) emit('close')
  st = 0
}

let lastTap = 0
async function toggleZoom(e) {
  const nowT = Date.now()
  if (nowT - lastTap > 300) { lastTap = nowT; return } // wait for the second tap
  lastTap = 0
  const rect = stage.value.getBoundingClientRect()
  const rx = (e.clientX - rect.left) / rect.width, ry = (e.clientY - rect.top) / rect.height
  zoomed.value = !zoomed.value
  if (zoomed.value) {
    await nextTick()
    const el = stage.value
    el.scrollLeft = rx * el.scrollWidth - rect.width / 2
    el.scrollTop = ry * el.scrollHeight - rect.height / 2
  }
}

onMounted(() => {
  document.addEventListener('keydown', onKey)
  document.body.style.overflow = 'hidden'
  preload()
})
onBeforeUnmount(() => {
  document.removeEventListener('keydown', onKey)
  document.body.style.overflow = ''
})
</script>

<template>
  <Teleport to="body">
    <div class="viewer" :class="{ withpanel: panelOpen }">
      <div class="v-main">
        <div class="v-top">
          <button class="icon-btn" @click="emit('close')" aria-label="Close"><Icon name="close" :size="24" /></button>
          <div class="grow v-title">
            <div class="ellipsis">{{ photo?.name }}</div>
            <div class="small" style="opacity:.6">{{ index + 1 }} / {{ photos.length }}</div>
          </div>
          <slot name="top" :photo="photo" />
        </div>
        <div ref="stage" class="v-stage" :class="{ zoomed }" @touchstart.passive="tstart" @touchend="tend" @click="toggleZoom">
          <img v-if="photo" :key="photo.id + '-t'" :src="thumb" class="v-thumb" v-show="loadedSrc !== src && !zoomed" alt="" />
          <img v-if="photo" :key="photo.id" :src="src" class="v-img" :class="{ show: loadedSrc === src }" @load="loadedSrc = src" :alt="photo.name" draggable="false" />
        </div>
        <button v-if="index > 0" class="v-nav l hidden-sm" @click="go(-1)" aria-label="Previous"><Icon name="left" :size="30" /></button>
        <button v-if="index < photos.length - 1" class="v-nav r hidden-sm" @click="go(1)" aria-label="Next"><Icon name="right" :size="30" /></button>
        <div class="v-bottom"><slot name="actions" :photo="photo" /></div>
      </div>
      <aside v-if="panelOpen" class="v-panel"><slot name="panel" :photo="photo" /></aside>
    </div>
  </Teleport>
</template>

<style>
.viewer { position: fixed; inset: 0; z-index: 80; background: #0d0c0b; color: #fff; display: flex; animation: fade .15s; }
.v-main { position: relative; flex: 1; min-width: 0; display: flex; flex-direction: column; }
.v-top { display: flex; align-items: center; gap: 8px; padding: 8px 10px; padding-top: max(8px, env(safe-area-inset-top)); z-index: 2; }
.v-top .icon-btn:hover, .v-bottom .icon-btn:hover { background: rgba(255,255,255,.12); }
.v-title { font-size: 14px; line-height: 1.3; }
.v-stage { flex: 1; position: relative; overflow: hidden; display: flex; align-items: center; justify-content: center; user-select: none; touch-action: pan-y; }
.v-stage.zoomed { overflow: auto; display: block; touch-action: auto; cursor: zoom-out; }
.v-stage:not(.zoomed) { cursor: zoom-in; }
.v-img, .v-thumb { max-width: 100%; max-height: 100%; object-fit: contain; }
.v-thumb { position: absolute; width: 100%; height: 100%; filter: blur(2px); }
.v-img { opacity: 0; transition: opacity .2s; position: relative; }
.v-img.show { opacity: 1; }
.v-stage.zoomed .v-img { max-width: none; max-height: none; }
.v-nav { position: absolute; top: 50%; transform: translateY(-50%); width: 52px; height: 52px; border-radius: 50%; border: 0; background: rgba(255,255,255,.08); color: #fff; cursor: pointer; display: flex; align-items: center; justify-content: center; }
.v-nav:hover { background: rgba(255,255,255,.18); }
.v-nav.l { left: 14px; } .v-nav.r { right: 14px; }
.v-bottom { display: flex; align-items: center; justify-content: center; gap: 10px; padding: 10px 12px; padding-bottom: max(12px, env(safe-area-inset-bottom)); flex-wrap: wrap; }
.v-panel { width: 380px; background: var(--surface); color: var(--text); display: flex; flex-direction: column; border-left: 1px solid #222; }
@media (max-width: 760px) {
  .viewer.withpanel { flex-direction: column; }
  .v-panel { width: 100%; height: 55vh; border-left: 0; border-radius: 16px 16px 0 0; }
}
.vbtn { display: inline-flex; align-items: center; gap: 7px; height: 42px; padding: 0 16px; border-radius: 99px; border: 1px solid rgba(255,255,255,.22); background: rgba(255,255,255,.06); color: #fff; font: inherit; font-weight: 600; cursor: pointer; }
.vbtn:hover { background: rgba(255,255,255,.14); }
.vbtn.on-heart { background: var(--heart); border-color: var(--heart); }
.vbtn.on-edit { background: var(--brand); border-color: var(--brand); color: var(--brand-ink); }
.vbtn:disabled { opacity: .45; cursor: not-allowed; }
.vbtn .dot { margin-left: 2px; }
</style>
