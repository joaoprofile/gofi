# Empty State — mobile

Vazio é tela de produto. Mesmos tipos do [web](../../../web/gofi-ui/components/empty-state.md):
primeiro-uso · busca-sem-resultado · tudo-resolvido (erro usa estado de **error**).

## Anatomia
Ilustração/ícone (decorativo) + título + descrição curta + CTA pílula. Centralizado,
respiro generoso. Como `ListEmptyComponent` da `FlatList`.

## Props
```ts
interface EmptyStateProps { icon?: ReactNode; title: string; description?: string;
  action?: ReactNode; variant?: 'first-use'|'no-results'|'all-done'; }
```

## Acessibilidade
- Ilustração decorativa oculta do leitor; título com `accessibilityRole="header"`.
- CTA é `Pressable`/Button real.

## Exemplo
```tsx
<EmptyState variant="no-results" title="Nenhum resultado para «{termo}»"
  description="Tente outro termo." action={<Button variant="secondary" onPress={clear}>Limpar</Button>} />
```
