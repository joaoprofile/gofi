# Toast — mobile

Feedback transitório (confirmação de ação). Aparece no topo ou base; some sozinho.
Para aviso persistente, use banner inline na tela.

## Anatomia
`[ ícone de status · mensagem · ação? (Desfazer) ]`. Cor por status. Respeita safe
area do lado em que aparece.

## Props
```ts
type Tone = 'success'|'warning'|'danger'|'info';
interface ToastOptions { tone: Tone; message: string;
  action?: { label: string; onPress: () => void }; duration?: number; }
```

## Acessibilidade
- Anuncie via `AccessibilityInfo.announceForAccessibility(message)` (toast não recebe
  foco). Erro urgente pode usar anúncio assertivo.
- Não comunique só por cor (ícone + texto). Não é a única via de info crítica (some).
- Tempo suficiente para ler; pausa ao tocar.

## Do / Don't
- ✅ "Desfazer" para ações reversíveis (princípio 10).
- ❌ Toast para erro que exige ação (use banner/inline). ❌ Toast atrás do tab bar/notch.

## Exemplo
```tsx
toast({ tone: 'success', message: 'Salvo', action: { label: 'Desfazer', onPress: undo } });
```
