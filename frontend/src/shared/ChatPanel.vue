<script setup>
// Conversation list + composer. Comments come from the parent (which also handles realtime updates).
import { ref, watch, nextTick, onMounted } from 'vue'
import Icon from './Icon.vue'
import { fmtTime, imgUrl } from './fmt.js'

const props = defineProps({
  comments: Array,
  canPost: { type: Boolean, default: true },
  placeholder: { type: String, default: 'Nhập tin nhắn…' },
  emptyText: { type: String, default: 'Chưa có tin nhắn' },
  showPhotoRef: { type: Boolean, default: true },
  lang: { type: String, default: 'vi' },
  loading: Boolean,
})
const emit = defineEmits(['send', 'open-photo'])

const text = ref('')
const box = ref(null)
const ta = ref(null)

function scrollBottom() { nextTick(() => { if (box.value) box.value.scrollTop = box.value.scrollHeight }) }
watch(() => props.comments?.length, scrollBottom)
onMounted(scrollBottom)

function send() {
  const t = text.value.trim()
  if (!t) return
  emit('send', t)
  text.value = ''
  autosize()
}
function onKey(e) {
  if (e.key === 'Enter' && !e.shiftKey && !e.isComposing) { e.preventDefault(); send() }
}
function autosize() {
  nextTick(() => {
    const el = ta.value
    if (!el) return
    el.style.height = '40px'
    el.style.height = Math.min(120, el.scrollHeight + 2) + 'px'
  })
}
</script>

<template>
  <div class="msgs" ref="box">
    <div v-if="loading" class="empty"><div class="spinner"></div></div>
    <div v-else-if="!comments?.length" class="empty"><Icon name="chat" :size="32" />{{ emptyText }}</div>
    <div v-for="c in comments" :key="c.id" class="msg" :class="{ mine: c.mine }">
      <div v-if="showPhotoRef && c.photo_id" class="ref" @click="emit('open-photo', c.photo_id)">
        <img :src="imgUrl(c.photo_id, 's')" alt="" />{{ c.photo_name }}
      </div>
      <div class="bubble">{{ c.body }}</div>
      <div class="meta">{{ c.mine ? '' : c.author_name + ' · ' }}{{ fmtTime(c.created_at, lang) }}</div>
    </div>
  </div>
  <div v-if="canPost" class="composer">
    <textarea ref="ta" class="input" v-model="text" :placeholder="placeholder" rows="1" @keydown="onKey" @input="autosize"></textarea>
    <button class="btn primary" style="width:44px;padding:0" @click="send" :disabled="!text.trim()" aria-label="Send"><Icon name="send" :size="18" /></button>
  </div>
</template>
