# Iconografia — mobile

## Estilo
- Um único set por app (line/duotone), traço consistente. Tamanhos 20/24/28
  (alvo de toque do icon button ≥ 44pt mesmo com ícone menor).
- Cor herda do tema (`color={t.textColor}`/`t.colorAction`), nunca avulsa.

## Acessibilidade
| Caso | RN |
|------|----|
| Decorativo (ao lado de texto) | `accessibilityElementsHidden` / `importantForAccessibility="no"` |
| Informativo sozinho | `accessibilityRole="image"` + `accessibilityLabel` |
| Icon button | `accessibilityRole="button"` + `accessibilityLabel` (ação) |

- Status nunca só por ícone/cor — acompanhe texto.
- Contraste de ícone informativo ≥ 3:1.
