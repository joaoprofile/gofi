# List Item — mobile

Linha rica. Use dentro de `FlatList`/`SectionList` (virtualizada), nunca `map` em
`ScrollView` para coleções.

## Anatomia
`[ leading: avatar/ícone ] [ título + subtítulo ] [ trailing: meta/chevron/ação ]`
Altura ≥ 56pt; divisor `surfaceBorder`; `Pressable` se clicável.

## Props
```ts
interface ListItemProps { leading?: ReactNode; title: string; subtitle?: string;
  trailing?: ReactNode; onPress?: () => void; selected?: boolean; }
```

## Acessibilidade
- `accessible` no item + `accessibilityLabel` consolidado (título + subtítulo + meta).
- `accessibilityRole="button"` quando clicável; `accessibilityState={{ selected }}`.

## FlatList
```tsx
<FlatList data={items} keyExtractor={i => i.id}
  renderItem={({ item }) => <ListItem title={item.name} subtitle={item.role}
    leading={<Avatar name={item.name} />} trailing={<Chevron />} onPress={() => open(item)} />}
  ItemSeparatorComponent={Divider}
  ListEmptyComponent={<EmptyState .../>}
  contentContainerStyle={{ paddingBottom: t.space[8] }} />
```

## Do / Don't
- ✅ `FlatList` p/ performance + `ListEmptyComponent`/footer de loading.
- ❌ Linha clicável com múltiplos sub-botões (alvo ambíguo).
