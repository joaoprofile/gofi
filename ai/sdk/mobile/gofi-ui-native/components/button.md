# Button — mobile

`Pressable` em **pílula** (ref. mockup). Mesma taxonomia do
[web/button](../../../web/gofi-ui/components/button.md).

## Variantes
`primary` (`colorAction`, texto branco) · `secondary` (borda `colorAction`) ·
`ghost` · `danger` · `brand` (`colorBrand` + `textOnBrand`). Tamanhos `sm/md/lg`;
`full` comum no mobile (largura total).

## Estados
`default · pressed (opacidade/escala leve, motion.fast) · disabled · loading
(ActivityIndicator + label mantido)`.

## Props
```ts
interface ButtonProps {
  variant?: 'primary'|'secondary'|'ghost'|'danger'|'brand';
  size?: 'sm'|'md'|'lg'; full?: boolean; loading?: boolean; disabled?: boolean;
  iconStart?: ReactNode; iconEnd?: ReactNode; onPress: () => void; children: string;
}
```

## Acessibilidade
- `Pressable` com `accessibilityRole="button"` + `accessibilityLabel` (se só ícone).
- `accessibilityState={{ disabled, busy: loading }}`. Alvo ≥ 44pt (`hitSlop` se preciso).
- Feedback de toque imediato; em `loading` mantém o label, não troca por "…".

## Do / Don't
- ✅ Pílula + largura total para CTA principal de tela. ✅ Verbo PT-BR no label.
- ❌ Texto branco sobre `brand` (superfície de marca clara — use `textOnBrand`). ❌ `TouchableWithoutFeedback` sem feedback visual.

## Exemplo
```tsx
<Button variant="primary" full loading={saving} onPress={save}>Salvar</Button>
<Button variant="brand" full onPress={start}>Quero saber mais</Button>
```
