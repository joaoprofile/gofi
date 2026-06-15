# Skeleton · Spinner — mobile

Loading sem tela vazia. Skeleton para conteúdo de layout conhecido (cards/linhas);
`ActivityIndicator` para ação pontual.

## Props
```ts
interface SkeletonProps { width?: number|string; height?: number; radius?: keyof Theme['radius']; }
```

## Regras
- Skeleton no **shape** do conteúdo (mesma altura/raio); shimmer respeita reduce-motion.
- `ActivityIndicator` dentro de botão `loading` ou overlay curto.
- Região carregando: `accessibilityState={{ busy: true }}`; skeleton oculto do leitor.
- Evite "flash": só mostre loading após ~150ms.

## Exemplo
```tsx
{isLoading
  ? <Stack>{[...Array(5)].map((_,i) => <Skeleton key={i} height={56} radius="md" />)}</Stack>
  : <FlatList data={data} .../>}
```
