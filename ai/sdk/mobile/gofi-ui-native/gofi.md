# GOFI Design System — Mobile (React Native)

Ponto de entrada para construir interfaces **mobile nativas**. Mesma marca e os
**mesmos tokens** do web ([design-tokens.md](../../../knowledge/ui/design-tokens.md))
— muda a **forma** (objeto TS, não CSS), os componentes (RN-nativos, sem DOM) e os
padrões (navegação por stack/tab, safe-area, gestos, toque).

> **Stack:** React Native + TypeScript. Tokens como objeto TS via
> `makeTheme(brand, mode)`, expostos por `<ThemeProvider>` + `useTheme()`; modo
> segue `useColorScheme`. Navegação com React Navigation. Sem Shadow DOM, sem CSS.

> **Lib de design system:** o pacote é **`gofi-ui-native`** (contraparte React
> Native do `gofi-ui`). Ao gerar código: envolva a app no `<ThemeProvider>` e
> importe da lib — `import { Button, Card, useTheme } from 'gofi-ui-native'`. Estes
> docs são a especificação domínio-neutra que a lib implementa no mobile.

## Filosofia

1. **Acessibilidade desde o início** — `accessibilityRole`/`accessibilityLabel`,
   leitor de tela (TalkBack/VoiceOver), Dynamic Type, alvos ≥ 44pt.
   Ver [foundations/accessibility.md](foundations/accessibility.md).
2. **Cor de marca dominante em superfícies grandes** — o **card de marca** preenche
   a tela de destaque (referência: mockup de onboarding), texto navy
   (`textOnBrand`), nunca branco sobre a superfície de marca clara — use `textOnBrand`.
3. **Tokens/modes** — claro/escuro do mesmo objeto de tokens.
4. **Geometria arredondada e arejada** — raio generoso, botões em pílula, respiro.

## Como o agente usa
```
1. Ler design-tokens.md (fonte única) + tokens-mobile.md (forma TS)
2. Mapear a tela com patterns/ (navigation, page-templates, hero-onboarding, states)
3. Compor com components/ existentes antes de criar novo (variant, não clone)
4. Garantir 4 estados + a11y + safe-area
```

## Diferenças-chave vs Web
| Tema | Web | Mobile (RN) |
|------|-----|-------------|
| Estilo | CSS vars | `StyleSheet.create` + objeto de tokens |
| Tema | `[data-theme]` | `useColorScheme()` + provider |
| Layout | grid/flex + media query | Flexbox + `useWindowDimensions` |
| Toque | hover/click | `Pressable` (pressed), gestos |
| Sombra | `box-shadow` | `elevation` (Android) + `shadow*` (iOS) |
| Navegação | rotas/URL | React Navigation (stack/tab) |
| Borda da tela | viewport | **Safe area** (notch, home indicator) |

## Índice
- **Foundations:** [tokens-mobile.md](foundations/tokens-mobile.md) ·
  [color.md](foundations/color.md) · [typography.md](foundations/typography.md) ·
  [spacing-layout.md](foundations/spacing-layout.md) ·
  [radius-elevation.md](foundations/radius-elevation.md) ·
  [motion.md](foundations/motion.md) · [iconography.md](foundations/iconography.md) ·
  [accessibility.md](foundations/accessibility.md)
- **Components:** [components/_index.md](components/_index.md)
- **Patterns:** [navigation.md](patterns/navigation.md) ·
  [hero-onboarding.md](patterns/hero-onboarding.md) ·
  [safe-area.md](patterns/safe-area.md) · [states.md](patterns/states.md) ·
  [page-templates.md](patterns/page-templates.md) · [forms.md](patterns/forms.md) ·
  [feedback.md](patterns/feedback.md)
