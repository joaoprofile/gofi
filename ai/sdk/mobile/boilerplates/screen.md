# Boilerplate — screen mobile (UI + hook + api + teste)

Esqueletos domínio-neutros. Substitua `{contexto}`, `<Feature>`, `Entity`. Importa o
DS de `gofi-ui-native` (npm). Cobre os **4 estados** + safe-area + a11y.

## `lib/api/{contexto}.ts`
```ts
import { type Entity } from './types';

export async function listEntities(): Promise<Entity[]> {
  const res = await fetch('https://api.exemplo/{contexto}');
  if (!res.ok) throw new Error('Falha ao carregar');
  return res.json();
}
```

## `features/{contexto}/use<Feature>.ts`
```ts
import { useQuery } from '@tanstack/react-query';
import { listEntities } from '../../lib/api/{contexto}';

export function use<Feature>() {
  return useQuery({ queryKey: ['{contexto}'], queryFn: listEntities });
}
```

## `features/{contexto}/<Feature>.tsx`
```tsx
import { FlatList } from 'react-native';
import { ListItem, Spinner, EmptyState, Banner, Button, useTheme } from 'gofi-ui-native';
import { use<Feature> } from './use<Feature>';

export function <Feature>() {
  const t = useTheme();
  const { data, isLoading, isError, refetch } = use<Feature>();

  if (isLoading) return <Spinner accessibilityLabel="Carregando" />;          // loading
  if (isError)
    return <Banner variant="danger">Não foi possível carregar.
      <Button onPress={() => refetch()}>Tentar de novo</Button></Banner>;     // error
  if (!data?.length)
    return <EmptyState title="Nada por aqui" description="Crie o primeiro." />; // empty

  return (                                                                     // success
    <FlatList
      data={data}
      keyExtractor={(e) => e.id}
      contentContainerStyle={{ padding: t.space[4], gap: t.space[3] }}
      renderItem={({ item }) => <ListItem title={item.name} />}
    />
  );
}
```

## `screens/<Feature>Screen.tsx`
```tsx
import { Screen, Header } from 'gofi-ui-native';
import { <Feature> } from '../features/{contexto}/<Feature>';

export function <Feature>Screen() {
  return (
    <Screen>                          {/* safe-area */}
      <Header title="Título" />
      <<Feature> />
    </Screen>
  );
}
```

> I/O só em `lib/api` + camada de dados; lista longa com `FlatList`; medidas/cores via
> `useTheme()`. Ver [knowledge/absolute-rules.md](../knowledge/absolute-rules.md).
</content>
