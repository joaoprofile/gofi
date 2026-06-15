# Bottom Sheet — mobile

Painel que sobe da base — padrão mobile para ações contextuais, seleção e
mini-formulários (equivale a Drawer/Popover do web).

## Anatomia
`[ handle (grabber) ] [ título? ] [ conteúdo ] [ ações ]` — `surfaceCard`,
`radius.xl` no topo, `shadow.lg`. Backdrop escurece o fundo.

## Variantes
- Por altura: `auto` (conteúdo) · snap points (ex.: 40%/90%) · full.
- `modal` (bloqueia fundo) · `non-modal` (persistente).
- Confirmação destrutiva pode ser um sheet de ação (action sheet).

## Estados
`opening (motion.slow) · open · dragging · closing`. Arrastar para baixo fecha.

## Props
```ts
interface BottomSheetProps { open: boolean; onClose: () => void; title?: string;
  snapPoints?: (number|string)[]; children: ReactNode; }
```

## Acessibilidade
- Foco move para dentro ao abrir (`setAccessibilityFocus`), retorna ao gatilho ao
  fechar. `accessibilityViewIsModal` no painel (iOS).
- Handle com `accessibilityLabel`; fechar também por botão (não só gesto).
- Respeita safe area inferior.

## Do / Don't
- ✅ Preferir bottom sheet a modal central no mobile.
- ✅ Snap points para conteúdo variável. ❌ Sheet que só fecha por gesto (inacessível).

## Exemplo
```tsx
<BottomSheet open={open} onClose={close} title="Ordenar por" snapPoints={['40%']}>
  {options.map(o => <ListItem key={o.id} title={o.label} onPress={() => pick(o)} />)}
</BottomSheet>
```
