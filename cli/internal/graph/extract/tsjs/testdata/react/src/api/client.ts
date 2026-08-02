export interface Cart {
  items: string[];
}

export async function fetchCart(): Promise<string[]> {
  const res = await fetch('/api/cart');
  return res.json();
}
