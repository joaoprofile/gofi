# Charts — mobile

Gráficos nativos leves com os tokens do DS (cores reagem à marca via `chartColors`).
Mais enxuto que o web: os casos comuns no mobile são barra, rosca e sparkline.

## Variantes
| Componente | Para |
|-----------|------|
| `BarChart` | comparação por categoria |
| `DonutChart` | proporção (com rótulo central opcional) |
| `Sparkline` | mini-tendência inline (em cards/linhas) |

## Props (TS)
```ts
interface BarDatum   { label: string; value: number; }
interface BarChartProps   { data: BarDatum[]; height?: number; ariaLabel: string; color?: string; }

interface DonutSlice { label: string; value: number; color?: string; }
interface DonutChartProps { data: DonutSlice[]; size?: number; ariaLabel: string; centerLabel?: string; }

interface SparklineProps  { data: number[]; width?: number; height?: number; ariaLabel: string; color?: string; area?: boolean; }
```

## Estados
Sem dados → `EmptyState` (nunca gráfico vazio). Carregando → `Skeleton` na altura do
gráfico.

## a11y
- `ariaLabel` **obrigatório** (`accessibilityLabel` da view do gráfico).
- Cor não é o único canal: rótulo/legenda sempre. Número crítico também em texto.

## Do / Don't
- ✅ `chartColors`/`color` por token; ✅ `centerLabel` no donut para o total.
- ❌ hex fixo; ❌ gráfico denso demais para tela pequena (prefira o essencial).

## Exemplo
```tsx
import { BarChart, DonutChart, Sparkline } from 'gofi-ui-native';

<BarChart data={[{ label: 'Seg', value: 12 }]} ariaLabel="Vendas por dia" />
<DonutChart data={fatias} ariaLabel="Distribuição" centerLabel="R$ 1.2k" />
<Sparkline data={[3,5,4,8,7]} ariaLabel="Tendência 7 dias" />
```
</content>
