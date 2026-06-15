# Pattern — Hero / Onboarding (cor de marca dominante)

Tela de destaque do mobile (referência fiel do mockup): um **card de marca grande**
preenche a superfície, com título, subtítulo, lista de features e CTA em pílula.
A marca **é** a superfície.

## Anatomia
```
SafeArea (top)
┌─────────────────────────────────────┐
│  Card variant="brand" (radius.xl)    │
│   Text display (textOnBrand)         │  ← título do produto
│   Text body   (textOnBrand)          │  ← subtítulo/descrição
│                                      │
│   FeatureList (onBrand)              │  ← [✓] feature  [✓] feature  [✓] feature
│                                      │
│   Button primary full  ────────────  │  ← CTA principal (pílula)
│   Button ghost/secondary (claro)     │  ← ação secundária ("saber mais")
└─────────────────────────────────────┘
SafeArea (bottom)
```

## Regras visuais
- Fundo do card = `colorBrand` (ex. `#AAD7FF`); **todo** texto/ícone = `textOnBrand`
  (navy, ex. `#0B2942`). **Nunca branco** sobre a superfície de marca clara — use `textOnBrand`.
- Raio `xl` (24), respiro generoso, uma única ação primária.
- CTA primário em pílula, largura total; secundária discreta.
- Fundo da tela atrás do card: `surfacePage` (branco/claro), bastante respiro.

## Variações
- **Onboarding multi-tela**: sequência de heros + indicador de página (dots) +
  "pular"/"próximo". Último passo leva à ação principal.
- **Produto/detalhe**: card de marca no topo + conteúdo (preço, features) abaixo,
  CTA fixo no rodapé respeitando safe area.

## Acessibilidade
- Título com `accessibilityRole="header"`. FeatureList como lista; ícones decorativos
  ocultos do leitor. Contraste `textOnBrand`/`colorBrand` ≥ 4.5:1 (validado).
- CTA com label-verbo PT-BR ("começar", "quero saber mais"). Alvos ≥ 44pt.
- Respeita Dynamic Type (texto cresce sem cortar) e safe area.

## Do / Don't
- ✅ Uma mensagem, uma ação primária por tela. ✅ Features curtas e escaneáveis.
- ❌ Texto branco sobre a marca clara. ❌ Encher o hero de ações concorrentes.

## Exemplo
```tsx
<Screen>
  <Card variant="brand">
    <Text variant="display" color="onBrand">{product.title}</Text>
    <Text variant="body" color="onBrand">{product.subtitle}</Text>
    <FeatureList onBrand items={product.features.map(label => ({ label }))} />
    <Button variant="primary" full onPress={onPrimary}>Quero saber mais</Button>
  </Card>
</Screen>
```
