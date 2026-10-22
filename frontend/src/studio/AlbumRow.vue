<script setup>
import Icon from '../shared/Icon.vue'
import { imgUrl, fmtDate, daysLeft } from '../shared/fmt.js'
import { statusLabel } from './store.js'
defineProps({ a: Object })
</script>

<template>
  <router-link :to="`/albums/${a.id}`" class="arow">
    <img v-if="a.cover" class="thumb" :src="imgUrl(a.cover, 's')" alt="" loading="lazy" />
    <span v-else class="thumb"></span>
    <div class="grow" style="min-width:0">
      <div class="row"><b class="ellipsis">{{ a.title }}</b><span v-if="a.unread" class="dot" title="Tin nhắn chưa đọc">{{ a.unread }}</span></div>
      <div class="small muted ellipsis">{{ a.customers.join(', ') || 'Chưa gán khách' }}</div>
      <div class="row wrap small" style="margin-top:3px">
        <span class="badge" :class="a.status">{{ statusLabel[a.status] }}</span>
        <span class="muted"><Icon name="image" :size="13" /> {{ a.photo_count }}</span>
        <span class="muted"><Icon name="heart" :size="13" /> {{ a.fav_count }}</span>
        <span class="muted"><Icon name="edit" :size="13" /> {{ a.edit_count }}<template v-if="a.max_edit">/{{ a.max_edit }}</template>
          <template v-if="a.edit_count && (a.status === 'editing' || a.done_count)"> · xong {{ a.done_count }}</template></span>
        <span v-if="a.due_at && a.status !== 'delivered'" :class="daysLeft(a.due_at) < 0 ? 'badge danger' : daysLeft(a.due_at) <= 3 ? 'badge submitted' : 'muted'">
          <Icon name="calendar" :size="13" /> Hạn {{ fmtDate(a.due_at) }}</span>
        <span v-if="a.editors?.length" class="muted hidden-sm">· Edit: {{ a.editors.join(', ') }}</span>
      </div>
    </div>
    <Icon name="right" class="muted" />
  </router-link>
</template>
