<script setup>
// Shows a customer's (new) password once, with a ready-to-send message.
import { computed } from 'vue'
import Modal from '../shared/Modal.vue'
import Icon from '../shared/Icon.vue'
import { copyText } from '../shared/fmt.js'
import { toast } from '../shared/ui.js'
import { state } from './store.js'

const props = defineProps({ info: Object }) // {phone, password, name}
const emit = defineEmits(['close'])

const link = computed(() => state.branding?.public_url || location.origin)
const message = computed(() =>
  `${state.branding?.name || 'Studio'} gửi ${props.info.name || 'bạn'} link xem & chọn ảnh:\n${link.value}\nĐăng nhập: SĐT ${props.info.phone}\nMật khẩu: ${props.info.password}`)

async function copy(text) {
  if (await copyText(text)) toast('Đã sao chép')
}
</script>

<template>
  <Modal title="Thông tin đăng nhập của khách" @close="emit('close')">
    <div class="alert warn"><Icon name="info" :size="18" />Mật khẩu chỉ hiển thị một lần. Hãy gửi cho khách ngay.</div>
    <div class="kv"><div>Số điện thoại</div><div class="mono">{{ info.phone }}</div></div>
    <div class="secret">{{ info.password }}</div>
    <div class="field"><label>Tin nhắn mẫu gửi khách (Zalo/SMS)</label><textarea class="input" rows="5" readonly :value="message"></textarea></div>
    <template #foot>
      <button class="btn" @click="copy(info.password)"><Icon name="copy" :size="16" />Chép mật khẩu</button>
      <button class="btn primary" @click="copy(message)"><Icon name="copy" :size="16" />Chép tin nhắn</button>
    </template>
  </Modal>
</template>
