<script>
// Stack of open modals: Escape only closes the top one.
const openModals = []
</script>

<script setup>
import { onMounted, onBeforeUnmount } from 'vue'
import Icon from './Icon.vue'
const props = defineProps({ title: String, wide: Boolean, closable: { type: Boolean, default: true } })
const emit = defineEmits(['close'])
const self = {}
function onKey(e) {
  if (e.key === 'Escape' && props.closable && openModals[openModals.length - 1] === self) emit('close')
}
onMounted(() => { openModals.push(self); document.addEventListener('keydown', onKey) })
onBeforeUnmount(() => {
  openModals.splice(openModals.indexOf(self), 1)
  document.removeEventListener('keydown', onKey)
})
</script>

<template>
  <Teleport to="body">
    <div class="overlay" @mousedown.self="closable && emit('close')">
      <div class="modal" :class="{ wide }" role="dialog" aria-modal="true">
        <div class="modal-head">
          <h2 class="grow">{{ title }}</h2>
          <button v-if="closable" class="icon-btn" @click="emit('close')" aria-label="Close"><Icon name="close" /></button>
        </div>
        <div class="modal-body"><slot /></div>
        <div v-if="$slots.foot" class="modal-foot"><slot name="foot" /></div>
      </div>
    </div>
  </Teleport>
</template>
