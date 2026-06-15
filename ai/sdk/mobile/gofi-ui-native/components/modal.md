# Modal · Confirm — mobile

Sobreposição central para confirmação/decisão. No mobile, **prefira
[Bottom Sheet](bottom-sheet.md)** para a maioria dos casos; reserve modal central
para confirmações curtas e críticas.

## Anatomia
`[ backdrop ] [ painel central: título + corpo + ações (cancelar · confirmar) ]`
`surfaceCard`, `radius.lg`, `shadow.lg`. Usa `Modal` do RN.

## Confirm destrutivo
- Título com o impacto; confirmar `variant="danger"` com verbo específico
  ("Excluir"), cancelar seguro. Destrutivo exige **confirmação ou undo**
  ([patterns/feedback.md](../patterns/feedback.md)).
- iOS: pode usar Alert/ActionSheet nativo para destrutivo simples.

## Props
```ts
interface ModalProps { open: boolean; onClose: () => void; title: string;
  footer?: ReactNode; dismissable?: boolean; children: ReactNode; }
```

## Acessibilidade
- `Modal` RN: foco entra no painel, `accessibilityViewIsModal`, retorna ao gatilho.
- Fechar por botão (não só backdrop); destrutivo não fecha por toque-fora sem confirmar.
- Bloqueia interação com o fundo.

## Do / Don't
- ✅ Bottom sheet > modal central no mobile. ✅ Verbo específico no destrutivo.
- ❌ Empilhar modais. ❌ Conteúdo longo (use tela).
