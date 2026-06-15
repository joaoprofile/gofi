# Segmented Control

Alterna entre **visões/filtros** mutuamente exclusivos no local (ex. "Em andamento /
Concluído" — ref. dashboard). 2–4 segmentos curtos.

## Anatomia
Trilha arredondada `--sf-hover` (`--radius-pill`/`md`); o segmento ativo
preenchido com `--sf-card` (ou `--action` para ênfase) + uma sombra leve.

## Quando usar (vs Tabs vs Select)
| Caso | Componente |
|------|-----------|
| 2–4 visões curtas, troca instantânea | Segmented Control |
| Muitas seções / conteúdo longo | [Tabs](tabs.md) |
| Escolha num formulário | [Select](select.md)/Radio |

## Estados
`default · selected · focus · disabled`. Efeito **imediato** (troca a visão).

## Props
```ts
interface SegmentedProps<T> { value: T; onChange: (v: T) => void;
  options: Array<{ value: T; label: string; count?: number }>; }
```

## Acessibilidade
- Padrão `role="tablist"`/`tab` **ou** um grupo de `radio` (escolha exclusiva). Setas
  navegam, `aria-selected`/`aria-checked` no ativo.
- Labels curtos; `count` opcional com `aria-label` ("Concluído, 12").

## Do / Don't
- ✅ Indicador de ativo claro (preenchimento + peso), não só cor.
- ❌ Mais de 4 segmentos (vira Tabs/Select). ❌ Labels longos que quebram linha.

## Exemplo
```tsx
<SegmentedControl value={view} onChange={setView}
  options={[{value:'doing',label:'Em andamento'},{value:'done',label:'Concluído',count:12}]} />
```
