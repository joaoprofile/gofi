# Acessibilidade — mobile (RN)

A11y é default. Espelha [web/accessibility.md](../../../web/gofi-ui/foundations/accessibility.md)
com a API do React Native.

## Checklist por componente interativo
| Item | RN |
|------|----|
| Papel | `accessibilityRole` ('button','link','header','image','switch','adjustable'…) |
| Rótulo | `accessibilityLabel` (toda ação sem texto visível) |
| Estado | `accessibilityState={{ disabled, selected, checked, expanded, busy }}` |
| Valor | `accessibilityValue` (slider/progress) |
| Agrupar | `accessible` no container + `accessibilityLabel` consolidado |
| Alvo | mínimo **44×44pt** (`hitSlop` quando o visual é menor) |
| Foco | `accessibilityRole="header"` para títulos; ordem lógica |

## Padrões
- **Leitor de tela** (VoiceOver/TalkBack): teste a tela inteira navegando por gestos.
- **Dynamic Type**: não desligue `allowFontScaling`; teste fonte grande.
- **Reduce motion**: `AccessibilityInfo.isReduceMotionEnabled()` ([motion.md](motion.md)).
- **Anúncios**: `AccessibilityInfo.announceForAccessibility('Salvo')` para feedback
  que não tem foco (ex.: toast).
- **Foco programático** ao abrir modal/sheet (`setAccessibilityFocus` via ref).
- Toque: `Pressable` com `accessibilityRole="button"`, nunca `View` com `onTouchEnd`.

## Conteúdo
- Não depender só de cor. Imagens informativas com label; decorativas ocultas do leitor.
- Campos com label associado ([components/field.md](../components/field.md));
  erro via `accessibilityState={{ invalid }}` + texto.
