# Progress

Mostra o avanço. Linear (barra) ou circular. Determinado (% conhecido) ou indeterminado
(loading sem fim previsível).

## Variantes
| Variante | Uso |
|---------|-----|
| `linear` | progresso numa linha/célula de tabela (ref. dashboard: "14/22") |
| `circular` | progresso compacto, upload de avatar |
| `indeterminate` | operação sem % conhecido (prefira [Skeleton](skeleton-spinner.md) para carregar conteúdo) |

Trilha `--sf-border`/`--sf-hover`; preenchimento `--action` (ou `--success`
quando completo).

## Props
```ts
interface ProgressProps { value?: number; max?: number; // ausência = indeterminado
  variant?: 'linear' | 'circular'; label?: string; showValue?: boolean; }
```

## Acessibilidade
- `role="progressbar"` + `aria-valuenow/min/max` (determinado) ou `role` sem
  `valuenow` (indeterminado). `aria-label` descrevendo o que está progredindo.
- **Sempre** acompanhe de um label textual quando o número importa ("14 de 22"), não a
  barra sozinha (não dependa de cor/forma).

## Do / Don't
- ✅ Cor de conclusão (`--success`) ao atingir 100%.
- ❌ Uma barra sem label numérico num contexto de dados. ❌ Um indeterminado
  eterno sem mensagem de status.

## Exemplo
```tsx
<Progress variant="linear" value={14} max={22} showValue label="Progresso" />
```
