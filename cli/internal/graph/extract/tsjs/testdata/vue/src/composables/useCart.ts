import { ref } from 'vue';

export interface CartItem {
  id: string;
  title: string;
}

export async function fetchCart(): Promise<CartItem[]> {
  const res = await fetch('/api/cart');
  return res.json();
}

export function useCart() {
  const items = ref<CartItem[]>([]);
  const load = async () => {
    items.value = await fetchCart();
  };
  return { items, load };
}
