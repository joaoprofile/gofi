# Modal · Drawer · Confirm Dialog

Overlays que capturam o foco para uma tarefa/decisão.

| Componente | Posição | Uso |
|-----------|---------|-----|
| **Modal** | centro | tarefa curta e focada, confirmação rica |
| **Drawer** | lateral (ou bottom sheet) | conteúdo contextual, formulário, filtros |
| **Confirm Dialog** | centro (modal pequeno) | confirmar uma ação **destrutiva** |

## Anatomia
```
[ backdrop (--z-overlay, escurece o fundo) ]
[ painel (--sf-card, --radius-lg/xl, --shadow-lg, --z-modal)
   header: título + fechar
   body
   footer: ações (cancelar · confirmar) ]
```

## Estados
`opening (duration-300) · open · closing`. Trava o scroll do fundo enquanto aberto.

## Props
```ts
interface OverlayProps { open: boolean; onClose: () => void; title: string;
  size?: 'sm'|'md'|'lg'; side?: 'right'|'left'|'bottom'; // drawer
  footer?: ReactNode; dismissable?: boolean; children: ReactNode; }
```

## Acessibilidade (crítica)
- `role="dialog"` + `aria-modal="true"` + `aria-labelledby` (título).
- O **foco** entra no painel ao abrir, fica **preso** enquanto aberto, e **volta** ao
  trigger ao fechar. `Esc` fecha (se `dismissable`). Clique no backdrop fecha (não
  em um fluxo destrutivo não confirmado).
- Botão de fechar com `aria-label="Fechar"`.

## Destrutivo (Confirm Dialog)
- Um título claro do impacto; botão de confirmar `variant="danger"` com um **verbo
  específico** ("Excluir", não "OK"); a ação de cancelar num lugar seguro.
- Toda mutação destrutiva exige **confirmação ou undo** — nunca ambos ausentes
  ([feedback.md](../patterns/feedback.md)).

## Do / Don't
- ✅ Mobile: Drawer/bottom sheet em vez de um Modal apertado.
- ❌ Empilhar modais. ❌ Modal para conteúdo longo/navegável (use uma página).

## Exemplo
```tsx
<ConfirmDialog open={open} onClose={close} title="Excluir registro?"
  confirmLabel="Excluir" tone="danger" onConfirm={remove}>
  Esta ação não pode ser desfeita.
</ConfirmDialog>
```
