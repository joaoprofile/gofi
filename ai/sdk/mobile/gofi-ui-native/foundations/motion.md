# Motion — mobile

Durações de [design-tokens.md](../../../../knowledge/ui/design-tokens.md)
(`t.motion`). Use **Reanimated**/Animated; anime `transform`/`opacity`.

## Durações
`fast 100` toque/press · `base 200` transições · `slow 300` entrada de sheet/modal.

## Reduce motion (obrigatório)
```ts
import { AccessibilityInfo } from 'react-native';
// checar isReduceMotionEnabled() e ouvir 'reduceMotionChanged'
// reduzido → trocar transição por fade curto / sem deslocamento
```

## Boas práticas
- Feedback de toque imediato (`Pressable` `pressed` → opacidade/escala leve).
- Gestos de navegação (swipe back) seguem o padrão da plataforma.
- Sem animação infinita/distrativa; respeite `prefers-reduced-motion` do sistema.
- Skeleton/spinner no loading, nunca tela vazia.
