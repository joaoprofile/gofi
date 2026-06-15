# Avatar — mobile

Imagem/iniciais/ícone em círculo (`radius.pill`). Tamanhos `xs..xl`. Stack **+N**
para grupos. Igual ao [web](../../../web/gofi-ui/components/avatar.md).

## Props
```ts
interface AvatarProps { src?: string; name: string; size?: 'xs'|'sm'|'md'|'lg'|'xl'; status?: 'online'|'offline'; }
interface AvatarStackProps { items: AvatarProps[]; max?: number; }
```

## Acessibilidade
- `accessibilityLabel` = nome (imagem informativa). Fallback de iniciais quando a
  imagem falhar (`onError`).
- Stack: label "{n} pessoas"; status por cor **+** label.

## Exemplo
```tsx
<AvatarStack max={3} items={people.map(p => ({ src: p.photo, name: p.name }))} />
```
