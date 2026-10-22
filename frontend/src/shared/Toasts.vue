<script setup>
import { toasts } from './ui.js'
</script>

<template>
  <div class="toasts">
    <div v-for="t in toasts" :key="t.id" class="toast" :class="t.type">{{ t.text }}</div>
  </div>
</template>
