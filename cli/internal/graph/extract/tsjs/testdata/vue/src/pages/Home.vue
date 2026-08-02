<script setup lang="ts">
import Card from '@app/components/Card.vue';
import EmptyState from '@app/components/EmptyState.vue';
import { useCart, fetchCart } from '@app/composables/useCart';

const cart = useCart();

function reload() {
  fetchCart();
}
</script>

<template>
  <main>
    <Card v-for="item in cart.items" :key="item.id" :item="item" />
    <empty-state v-if="!cart.items.length" @retry="reload" />
  </main>
</template>
