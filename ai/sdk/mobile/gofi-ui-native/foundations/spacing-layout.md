# Espaçamento e layout — mobile

Escala 4/8 ([design-tokens.md](../../../../knowledge/ui/design-tokens.md)),
exposta como `t.space[n]`. Layout por **Flexbox** (default do RN).

## Regras
- Toda medida vem de `t.space` — sem número avulso.
- Responsividade por `useWindowDimensions()` e flex, não media query.
- **Safe area** sempre considerada nas bordas ([patterns/safe-area.md](../patterns/safe-area.md)).
- Densidade **touch** é o default: alvos ≥ 44pt, respiro entre toques.

## Primitivos de layout
| Primitivo | RN |
|-----------|----|
| `Stack` (coluna) | `View` `flexDirection:'column'` + `gap` |
| `Row` (linha) | `View` `flexDirection:'row'` + `gap` + `alignItems` |
| `Screen` | `SafeAreaView` + padding + `surfacePage` |
| Lista longa | `FlatList`/`SectionList` (virtualizada), não `map` em `ScrollView` |

```tsx
<View style={{ gap: t.space[4], padding: t.space[4] }}>…</View>
```

## Scroll
- Conteúdo rolável: `ScrollView` (curto) ou `FlatList` (coleções).
- `contentContainerStyle` com padding inferior para não colar no tab bar/home indicator.
- `keyboardShouldPersistTaps` e `KeyboardAvoidingView` em formulários.
