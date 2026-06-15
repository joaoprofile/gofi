# Raio e elevação — mobile

Raio de [design-tokens.md](../../../../knowledge/ui/design-tokens.md) via `t.radius`.
Geometria arredondada e generosa; card de marca usa `xl` (24).

## Raio por componente
`sm 8` inputs/badges · `md 12` botões/cards pequenos · `lg 16` cards/sheets ·
`xl 24` card de marca/bottom sheet · `pill 999` botões em pílula/chips/avatar.

## Elevação (cross-platform)
RN não tem `box-shadow` unificado: combine `elevation` (Android) + `shadow*` (iOS).

```ts
export const shadow = {
  sm: { elevation: 1, shadowColor:'#101828', shadowOpacity:.06, shadowRadius:2, shadowOffset:{width:0,height:1} },
  md: { elevation: 3, shadowColor:'#101828', shadowOpacity:.08, shadowRadius:12, shadowOffset:{width:0,height:4} },
  lg: { elevation: 8, shadowColor:'#101828', shadowOpacity:.12, shadowRadius:24, shadowOffset:{width:0,height:12} },
};
```

- Card padrão: `surfaceCard` + `surfaceBorder` (StyleSheet.hairlineWidth ok) + `shadow.sm`.
- No dark, sombra some → use `surfaceBorder` para separar superfícies.
- Sombra precisa de `backgroundColor` no iOS (sem fundo não renderiza).
