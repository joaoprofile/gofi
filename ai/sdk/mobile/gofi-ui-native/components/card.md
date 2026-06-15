# Card — mobile

Superfície agrupadora. `surfaceCard` + `surfaceBorder` + `shadow.sm` + `radius.lg`.
Variante **brand** é a estrela do mobile (card de marca do onboarding).

## Variantes
| Variante | Uso |
|----------|-----|
| `default` | conteúdo padrão |
| `brand` | superfície de marca (`colorBrand` + `textOnBrand`, `radius.xl`) — hero/destaque |
| `interactive` | `Pressable` (pressed eleva) — card clicável |
| `outlined` | só borda |

## Props
```ts
interface CardProps { variant?: 'default'|'brand'|'interactive'|'outlined';
  onPress?: () => void; children: ReactNode; }
```

## Acessibilidade
- Card clicável → `Pressable` com `accessibilityRole="button"` + label consolidado.
- `brand`: garanta `textOnBrand` (navy), nunca branco sobre a superfície de marca clara — use `textOnBrand`.

## Do / Don't
- ✅ Card de marca grande, arredondado (`xl`), com título + subtítulo + CTA pílula.
- ❌ Sombra sem `backgroundColor` (não renderiza no iOS).

## Exemplo
```tsx
<Card variant="brand">
  <Text variant="display" color="onBrand">{title}</Text>
  <Text variant="body" color="onBrand">{subtitle}</Text>
  <FeatureList items={features} />
  <Button variant="primary" full onPress={cta}>Quero saber mais</Button>
</Card>
```
