# Tab Bar — mobile

Navegação inferior entre **3–5 destinos** de topo do app (Bottom Tabs do React
Navigation). Item ativo em `colorAction`.

## Anatomia
`[ 🏠 ][ 🔍 ][ ➕ ][ 🔔 ][ 👤 ]` — ícone + label curto. Ativo: cor + (opcional) leve
realce. Badge de contagem em itens (ex.: notificações).

## Regras
- 3–5 itens (mais que isso → "Mais"/menu). Destino, não ação efêmera.
- Respeita safe area inferior (home indicator) — `useSafeAreaInsets().bottom`.
- Persiste estado de cada aba (não recarrega tudo ao trocar).

## Props (via React Navigation)
```ts
// configurar tabBarActiveTintColor: theme.colorAction
// tabBarIcon, tabBarLabel, tabBarBadge, tabBarAccessibilityLabel
```

## Acessibilidade
- Cada tab: `accessibilityRole="tab"` + `accessibilityState={{ selected }}` +
  label (não só ícone). Badge com label ("3 não lidas").
- Alvo ≥ 44pt.

## Do / Don't
- ✅ Ícone **+** label (não só ícone). ✅ Item ativo claro (cor + peso).
- ❌ > 5 abas. ❌ Esconder atrás do home indicator.
