# Badge · Tag · Chip

Rótulos curtos. Distinção de papel:

| Componente | Papel | Interativo? |
|-----------|-------|-------------|
| **Badge** | status/contagem (ex. "3", "Ativo") | não |
| **Tag** | categoria/metadado (ex. a categoria do card) | não (ou um link) |
| **Chip** | seleção/filtro removível | sim (remover/alternar) |

## Anatomia
Pill (`--radius-pill`) ou `--radius-sm`, padding `p-1`/`p-2`,
`--text-caption`. Chip: `[ label · ✕ ]`.

## Variantes (cor por status/categoria)
- Status: `success · warning · danger · info · neutral` — usa `--{status}-bg` como
  fundo e `--{status}` como texto/ícone (tom claro, texto escuro).
- Contagem: círculo `--action` + texto branco (badge numérico de notificação).

## Estados (Chip)
`default · selected (--action) · focus · disabled`.

## Props
```ts
interface BadgeProps { tone?: 'success'|'warning'|'danger'|'info'|'neutral'; children: ReactNode; }
interface ChipProps  { selected?: boolean; onRemove?: () => void; onClick?: () => void; children: ReactNode; }
```

## Acessibilidade
- Badge de status: não dependa só da cor — inclua texto/ícone.
- Badge numérico de notificação: `aria-label="3 não lidas"` (não apenas "3").
- Chip removível: botão ✕ com `aria-label="Remover {label}"`.

## Do / Don't
- ✅ Tom de status consistente com [color.md](../foundations/color.md).
- ✅ Tag de categoria colorida nos cards de recomendação (ref. dashboard).
- ❌ Badge como botão (use Button). ❌ Texto longo num badge (é um rótulo, não uma frase).

## Exemplo
```tsx
<Badge tone="success">Ativo</Badge>
<Tag>{category}</Tag>
<Chip selected={active} onRemove={() => remove(id)}>{filterLabel}</Chip>
```
