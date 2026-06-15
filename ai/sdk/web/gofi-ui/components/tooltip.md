# Tooltip

Uma dica curta no hover/focus. Complementa, **nunca** carrega informação
essencial (ela desaparece, e não existe em toque puro).

## Anatomia
Balão `--sf-sunken`/escuro + texto curto, seta opcional, `--z-dropdown`.
Posicionado com tratamento de colisão no viewport.

## Estados
`hidden · visible`. Atraso de entrada ~300–500ms; some ao sair/blur/Esc.

## Props
```ts
interface TooltipProps { label: string; side?: 'top'|'right'|'bottom'|'left'; children: ReactElement; }
```

## Acessibilidade
- Aparece no **hover e no focus** (teclado), não só no hover.
- `aria-describedby` ligando o trigger ao tooltip; `role="tooltip"`.
- Texto **curto**. Para conteúdo rico/interativo use um [Popover](menu-popover.md).
- Não use um tooltip como o **único** label de um icon button — esse precisa do seu
  próprio `aria-label`.

## Do / Don't
- ✅ Esclarecer um ícone/abreviação. ✅ Dispensar com `Esc`.
- ❌ Info crítica só no tooltip (toque e leitor podem não alcançar).
- ❌ Tooltip num elemento não focável.

## Exemplo
```tsx
<Tooltip label="Sincronizado há 2 min">
  <IconButton aria-label="Status de sincronização"><SyncIcon/></IconButton>
</Tooltip>
```
