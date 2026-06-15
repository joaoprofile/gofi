# Boilerplate — feature web (UI + hook + api + teste)

Esqueletos domínio-neutros. Substitua `{contexto}`, `<Feature>`, `Entity`. Importa o
DS de `gofi-ui` (npm). Cobre os **4 estados** e a11y por padrão.

## `lib/api/{contexto}.ts`
```ts
import { type Entity } from './types';

const BASE = '/api/{contexto}';

export async function listEntities(): Promise<Entity[]> {
  const res = await fetch(BASE);
  if (!res.ok) throw new Error('Falha ao carregar');
  return res.json();
}
```

## `features/{contexto}/use<Feature>.ts`
```ts
import { useQuery } from '@tanstack/react-query';
import { listEntities } from '../../lib/api/{contexto}';

export function use<Feature>() {
  const query = useQuery({ queryKey: ['{contexto}'], queryFn: listEntities });
  return query; // { data, isLoading, isError, refetch }
}
```

## `features/{contexto}/<Feature>.tsx`
```tsx
import { Card, Spinner, EmptyState, Banner, Button } from 'gofi-ui';
import { use<Feature> } from './use<Feature>';

export function <Feature>() {
  const { data, isLoading, isError, refetch } = use<Feature>();

  if (isLoading) return <Spinner aria-label="Carregando" />;                 // loading
  if (isError)
    return <Banner variant="danger" action={<Button onClick={() => refetch()}>Tentar de novo</Button>}>
      Não foi possível carregar.
    </Banner>;                                                                // error
  if (!data?.length)
    return <EmptyState title="Nada por aqui" description="Comece criando o primeiro." />; // empty

  return (                                                                    // success
    <ul className="grid gap-4">
      {data.map((e) => (
        <li key={e.id}><Card>{e.name}</Card></li>
      ))}
    </ul>
  );
}
```

## `features/{contexto}/__tests__/<Feature>.test.tsx`
```tsx
import { render, screen } from '@testing-library/react';
import { <Feature> } from '../<Feature>';
// renderize com um QueryClientProvider de teste + mock de listEntities

test('mostra os itens carregados', async () => {
  render(<TestProviders><<Feature> /></TestProviders>);
  expect(await screen.findByText('Item 1')).toBeInTheDocument();
});
```

> Regra: I/O só em `lib/api` + TanStack Query; UI só compõe DS; nunca `getByTestId`
> como primeira escolha. Ver [knowledge/absolute-rules.md](../knowledge/absolute-rules.md).
</content>
