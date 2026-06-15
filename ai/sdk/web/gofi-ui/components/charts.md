# Charts

Wrappers sobre **Recharts** com os tokens do DS (cores reagem a tema/marca). Os
gráficos comuns vêm de `gofi-ui`; as primitivas Recharts cruas, do subpath
`gofi-ui/charts` (para montar qualquer gráfico via `ChartContainer`).

## Anatomia
Gráfico = série(s) de dados + eixos + legenda/tooltip. Cor das séries vem de
`chartColors` (derivado de `--action`/`--accent`/status) → muda com a marca.

## Variantes (componentes prontos)
| Componente | Para |
|-----------|------|
| `AreaChart` · `BarChart` · `LineChart` | séries cartesianas (eixo X categórico) |
| `DonutChart` | proporção (rosca ou pizza) |
| `ChartContainer` | escape hatch: **um** gráfico Recharts qualquer com o tema do DS |

## Props (TS)
```ts
// AreaChart | BarChart | LineChart
interface CartesianChartProps {
  data: ChartDatum[];
  series: ChartSeries[];
  xKey: string;                 // campo da categoria no eixo X
  height?: number;
  showGrid?: boolean; showLegend?: boolean; stacked?: boolean;
  valueFormatter?: (v: number | string) => string;
  ariaLabel: string;            // OBRIGATÓRIO (leitor de tela)
  className?: string;
}
interface DonutChartProps {
  data: PieDatum[]; height?: number;
  donut?: boolean;              // true = rosca; false = pizza
  showLegend?: boolean; valueFormatter?: (v: number | string) => string;
  ariaLabel: string; className?: string;
}
interface ChartContainerProps { height?: number; ariaLabel: string; children: ReactElement; className?: string; }
```

## Estados
Sem dados → **não** renderize um gráfico vazio: use `EmptyState`. Carregando →
`Skeleton` com a altura do gráfico. Erro → `Banner` + retry.

## a11y
- `ariaLabel` é **obrigatório** — descreve o que o gráfico mostra.
- Cor nunca é o único canal: use legenda/rótulo. Tooltip acessível via `ChartTooltip`.

## Do / Don't
- ✅ `chartColors`/`chartSeries` para as cores (reagem à marca).
- ❌ hex fixo nas séries; ❌ gráfico como única forma de ler um número crítico
  (ofereça também o valor em texto).

## Exemplo
```tsx
import { AreaChart, BarChart, DonutChart } from 'gofi-ui';
import { ChartContainer, ComposedChart, Bar, Line } from 'gofi-ui/charts';

<AreaChart data={data} series={[{ key: 'receita', label: 'Receita' }]} xKey="mes"
           ariaLabel="Receita por mês" valueFormatter={brl} />

// qualquer gráfico Recharts com o tema do DS:
<ChartContainer ariaLabel="Receita vs meta">
  <ComposedChart data={data}><Bar dataKey="receita" /><Line dataKey="meta" /></ComposedChart>
</ChartContainer>
```
</content>
