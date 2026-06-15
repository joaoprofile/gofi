# Text

Encapsula tipografia ([foundations/typography.md](../foundations/typography.md)).
Todo texto da UI passa por aqui — nunca `<Text>` cru com cor/tamanho avulso.

## Props
```ts
interface TextProps extends RNTextProps {
  variant?: 'display'|'h1'|'h2'|'h3'|'body'|'bodySm'|'caption'; // default body
  color?: 'default' | 'secondary' | 'onBrand' | 'action' | 'danger';
  numberOfLines?: number;
  maxFontSizeMultiplier?: number; // limita Dynamic Type onde quebra layout
}
```

## Regras
- Cor por papel (mapeada do tema), não hex.
- `allowFontScaling` fica **true** (Dynamic Type); limite só com `maxFontSizeMultiplier`.
- `accessibilityRole="header"` quando for título (h1–h3).

## Exemplo
```tsx
<Text variant="h1">{title}</Text>
<Text variant="bodySm" color="secondary">{subtitle}</Text>
<Text variant="display" color="onBrand">{heroTitle}</Text>
```
