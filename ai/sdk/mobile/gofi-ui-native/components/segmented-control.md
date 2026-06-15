# Segmented Control — mobile

Troca entre 2–4 visões/segmentos mutuamente exclusivos (uma linha pílula). Espelha o
[web](../../../web/gofi-ui/components/segmented-control.md). Tipado por genérico.

## Anatomia
`[ opção A | opção B | opção C ]` — uma selecionada por vez; o indicador usa
`colorActionSubtle` (tint de ativo).

## Props (TS)
```ts
interface Segment<T extends string> { value: T; label: string }
interface SegmentedControlProps<T extends string> {
  value: T;
  onChange: (value: T) => void;
  options: Segment<T>[];          // 2–4 itens
}
```

## Estados
`selecionado` · `não-selecionado` · `pressed`. Use para **troca de visão**, não para
ação (botão) nem para escolha múltipla (use `Toggle`/checkbox).

## a11y
- `accessibilityRole` de cada segmento como botão/tab; o ativo expõe
  `accessibilityState={{ selected: true }}`.
- Rótulos curtos; alvo de toque ≥ 44pt.

## Do / Don't
- ✅ 2–4 opções curtas; ✅ uma seleção sempre ativa.
- ❌ >4 segmentos (use `Tabs`/`Select`); ❌ para confirmar uma ação.

## Exemplo
```tsx
import { SegmentedControl } from 'gofi-ui-native';

<SegmentedControl
  value={visao}
  onChange={setVisao}
  options={[{ value: 'lista', label: 'Lista' }, { value: 'mapa', label: 'Mapa' }]}
/>
```
</content>
