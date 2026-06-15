# Espaçamento e layout

Escala 4/8 em [design-tokens.md](../../../../knowledge/ui/design-tokens.md).

## Escala — sem valores avulsos

`margin: 13px` é um defeito. Use a **escala de espaçamento do Tailwind** (`p-4`,
`gap-6`, `px-5`, `mt-8`) — base 4px, o número coincide com a escala da fonte única
(`space-4` = `p-4` = 16px). O web não define `--space-*`. O espaçamento entre
seções é maior do que dentro de um componente (ritmo vertical previsível).

## Mobile-first (inegociável)

Escreva as classes **mobile** primeiro (base, sem prefixo). Expanda com `md:`
(≥768px) e `lg:` (≥1024px) — apenas refinamento.

```tsx
<div className="grid grid-cols-1 gap-4 md:grid-cols-2 lg:grid-cols-3"> … </div>
```

## Primitivos de layout

| Primitivo | Faz | Implementação |
|-----------|------|----------------|
| `Stack` | empilha com gap consistente | `display:flex; flex-direction:column; gap` |
| `Inline` | linha com gap + wrap | `display:flex; gap; flex-wrap:wrap` |
| `Grid` | grid responsivo | `grid` + `minmax()` / `auto-fit` |
| `Container` | largura máxima + centralização | `max-width:1280px; margin-inline:auto; padding-inline` |

- **Sem CSS inline para layout** — apenas um token calculado em runtime.
- **`100dvh`/`100svh`**, nunca `100vh` (a barra do mobile corta).
- **Container queries** (`@container`) quando o componente reage ao espaço do
  **pai**, não da viewport.
- O grid do app-shell (sidebar + conteúdo) em
  [patterns/app-shell.md](../patterns/app-shell.md).
