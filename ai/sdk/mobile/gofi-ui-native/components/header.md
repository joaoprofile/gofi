# Header — mobile

Cabeçalho de tela (navigation header). Integra com React Navigation
([patterns/navigation.md](../patterns/navigation.md)) e respeita a safe area do topo.

## Anatomia
`[ ‹ voltar ] [ título (centralizado/à esquerda) ] [ ações à direita ]`
Sobre `surfaceCard`/`surfacePage`; título `h3`/`h2`. Large title (iOS) opcional.

## Variantes
`default` · `large` (título grande que colapsa ao rolar) · `brand` (fundo de marca
em telas de destaque) · `transparent` (sobre hero/imagem).

## Props
```ts
interface HeaderProps { title: string; onBack?: () => void;
  actions?: ReactNode; variant?: 'default'|'large'|'brand'|'transparent'; }
```

## Acessibilidade
- Botão voltar: `accessibilityRole="button"` + `accessibilityLabel="Voltar"`; gesto
  de swipe-back nativo preservado.
- Título com `accessibilityRole="header"`. Ações com label. Alvos ≥ 44pt.
- Respeita `useSafeAreaInsets().top`.

## Do / Don't
- ✅ Voltar sempre disponível em telas empilhadas. ✅ Título reflete a tela.
- ❌ Header colando no notch (use safe area). ❌ Muitas ações no topo (use menu).
