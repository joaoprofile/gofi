# Progress — mobile

Barra de progresso linear. Use junto de um rótulo de texto quando o número importa.
Espelha o [web](../../../web/gofi-ui/components/progress.md), aqui só linear.

## Props (TS)
```ts
interface ProgressProps {
  value: number;          // 0..max
  max?: number;           // default 100
  label?: string;         // accessibilityLabel
  color?: string;         // default: colorAction (ou success quando 100%)
  height?: number;        // default 8
}
```

## Estados
`em progresso` (preenche por `value/max`) · `completo` (100% → cor `success`). Trilho
usa `surfaceBorder`; preenchimento `colorAction`.

## a11y
- `accessibilityRole="progressbar"` + `accessibilityValue` (now/min/max) já embutidos.
- Passe `label` para o leitor de tela anunciar o que progride.

## Do / Don't
- ✅ rótulo de texto ao lado quando o valor é informativo.
- ❌ usar como spinner indeterminado (para isso, `Skeleton`/`Spinner`).

## Exemplo
```tsx
import { Progress } from 'gofi-ui-native';

<Progress value={enviados} max={total} label={`${enviados} de ${total}`} />
```
</content>
