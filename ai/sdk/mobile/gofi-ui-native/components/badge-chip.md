# Badge · Chip — mobile

Igual ao [web](../../../web/gofi-ui/components/badge-tag.md): Badge (status/
contagem, não interativo) · Chip (filtro/seleção removível, `Pressable`).

## Variantes
Tom por status (`success/warning/danger/info/neutral`) com tint de fundo + texto
escuro. Chip selecionado: `colorAction`.

## Props
```ts
interface BadgeProps { tone?: 'success'|'warning'|'danger'|'info'|'neutral'; children: string; }
interface ChipProps { selected?: boolean; onPress?: () => void; onRemove?: () => void; children: string; }
```

## Acessibilidade
- Badge numérico: `accessibilityLabel="3 não lidas"` (não só "3").
- Chip: `accessibilityRole="button"` + `accessibilityState={{ selected }}`; remover
  com label "Remover {label}". Status nunca só por cor.

## Exemplo
```tsx
<Badge tone="success">Ativo</Badge>
<Chip selected={active} onPress={toggle}>{filterLabel}</Chip>
```
