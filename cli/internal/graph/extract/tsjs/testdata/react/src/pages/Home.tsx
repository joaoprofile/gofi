import { useEffect } from 'react';
import { Card } from '@app/components/Card';
import { useCart } from '@app/hooks/useCart';

export default function Home() {
  const { items, load } = useCart();
  useEffect(() => {
    load();
  }, [load]);
  return (
    <main>
      {items.map((item) => (
        <Card key={item} title={item} />
      ))}
    </main>
  );
}
