<script setup lang="ts">
import type { CartItem } from '@app/composables/useCart';

defineProps<{ item: CartItem }>();
</script>

<template>
  <article class="card">{{ item.title }}</article>
</template>

<style scoped>
.card {
  border: 1px solid #ccc;
}
</style>
