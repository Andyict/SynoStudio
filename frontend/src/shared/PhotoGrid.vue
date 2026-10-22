<script setup>
// Square-tile photo grid. mode: 'customer' (heart button), 'staff' (badges only), 'guest' (plain).
import Icon from './Icon.vue'
import { imgUrl } from './fmt.js'

const props = defineProps({
  photos: Array,
  mode: { type: String, default: 'customer' },
  guest: String,
  locked: Boolean,
  showNew: Boolean,
  editIndex: Object, // photo id -> 1-based order among edit picks
  editToggle: Boolean, // customer: show a pencil button to pick/unpick for editing
})
const emit = defineEmits(['open', 'heart', 'edit'])

function loaded(e) { e.target.classList.add('ld') }
</script>

<template>
  <div class="pgrid">
    <div v-for="(p, i) in photos" :key="p.id" class="tile" :class="{ sel: p.edit && mode !== 'guest' }" @click="emit('open', i)">
      <img :src="imgUrl(p.id, 's', guest)" loading="lazy" decoding="async" :alt="p.name" @load="loaded" />
      <button v-if="editToggle && mode === 'customer' && p.kind === 'orig'" class="t-editbtn" :class="{ on: p.edit }" :disabled="locked"
        @click.stop="emit('edit', p)" aria-label="Edit pick">
        <Icon :name="p.edit ? 'check' : 'edit'" :size="15" /><span v-if="p.edit">{{ editIndex?.[p.id] || '' }}</span>
      </button>
      <span v-else-if="p.edit && mode !== 'guest'" class="t-edit"><Icon name="edit" :size="12" />{{ editIndex?.[p.id] || '' }}</span>
      <span v-if="mode === 'staff' && p.done" class="t-done"><Icon name="check" :size="14" /></span>
      <span v-if="p.cm && mode !== 'guest'" class="t-cm"><Icon name="comment" :size="14" fill />{{ p.cm }}</span>
      <span v-if="showNew && !p.viewed && p.kind === 'orig'" class="t-new"></span>
      <button v-if="mode === 'customer' && p.kind === 'orig'" class="t-heart" :class="{ on: p.fav }" :disabled="locked"
        @click.stop="emit('heart', p)" :aria-label="p.fav ? 'Unlike' : 'Like'">
        <Icon name="heart" :size="24" :fill="p.fav" />
      </button>
      <span v-else-if="mode === 'staff' && p.fav" class="t-heart on" style="pointer-events:none"><Icon name="heart" :size="20" fill /></span>
    </div>
  </div>
</template>
