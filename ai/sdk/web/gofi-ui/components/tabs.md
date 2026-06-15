# Tabs

Organiza conteúdo em painéis paralelos sob um mesmo contexto. Para 2–4 *visões*
curtas e uma troca instantânea, prefira [Segmented Control](segmented-control.md).

## Anatomia
`[ tab · tab · tab ]` (indicador no ativo) + o painel abaixo. Ativo:
peso + sublinhado/preenchimento `--action`.

## Variantes
`underline` (linha inferior) · `pill` (preenchimento no ativo) · `vertical` (lista lateral).
Overflow: scroll horizontal com fade, nunca quebra de linha.

## Estados
`default · active · hover · focus · disabled`.

## Props
```ts
interface TabsProps { value: string; onChange: (id: string) => void;
  tabs: Array<{ id: string; label: string; badge?: number; disabled?: boolean }>; }
```

## Acessibilidade
- Padrão ARIA Tabs: `role="tablist"` / `role="tab"` (`aria-selected`,
  `aria-controls`) / `role="tabpanel"` (`aria-labelledby`).
- Teclado: setas movem entre tabs, `Home/End`, ativação automática ou manual
  (`Enter`). Focus visível.
- Painel inativo: `hidden` (não só uma classe `display:none` quebrada).

## Do / Don't
- ✅ Preserve o estado por tab quando fizer sentido (não recarregue tudo).
- ✅ Deep-link para a tab via URL quando navegável.
- ❌ Tabs para um passo a passo sequencial (use [Stepper](stepper.md)).

## Exemplo
```tsx
<Tabs value={tab} onChange={setTab}
  tabs={[{id:'info',label:'Detalhes'},{id:'hist',label:'Histórico',badge:3}]} />
```
