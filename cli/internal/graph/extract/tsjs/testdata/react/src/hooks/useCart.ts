import { useEffect, useState } from 'react';
import { fetchCart } from '@app/api/client';

export function useCart() {
  const [items, setItems] = useState<string[]>([]);
  const load = async () => {
    setItems(await fetchCart());
  };
  useEffect(() => {
    load();
  }, []);
  return { items, load };
}
