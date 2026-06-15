# Pattern — Exibição de dados

Escolher a estrutura certa para coleções.

## Table vs List vs Cards vs Grid
| Estrutura | Quando | Mobile |
|-----------|------|--------|
| [Table](../components/table.md) | muitos atributos comparáveis por linha, ordenar/filtrar | colapsa em cards |
| [List](../components/list.md) | item = "linha rica" (avatar + título + meta) | natural |
| Cards ([Card](../components/card.md)) | item visual/destacável, leitura exploratória | grid 1→2→3 cols |
| Grid | itens visuais homogêneos (mídia) | `auto-fit minmax()` |

## Componentes de apoio
- Toolbar: busca + filtros ([Chip](../components/badge-tag.md)) + ordenação +
  um [Segmented Control](../components/segmented-control.md) de visualização.
- Por linha/card: [Avatar stack](../components/avatar.md), [Progress](../components/progress.md),
  [Badge](../components/badge-tag.md), uma ação ([Button](../components/button.md)/[Menu](../components/menu-popover.md)).
- Footer: [Pagination](../components/pagination.md) **ou** "carregar mais"/infinito.

## Os 4 estados (sempre)
`loading` (skeleton no formato) · `empty` ([EmptyState](../components/empty-state.md):
distinguir primeiro uso de busca-sem-resultado) · `error` (Banner + tentar de novo) · `success`.
Detalhe em [states.md](states.md).

## Densidade e leitura
- Números tabulares, alinhados à direita; texto alinhado à esquerda.
- Coluna de status: ícone + texto + cor (nunca só a cor).
- Ordenação atual visível (`aria-sort`); filtros aplicados visíveis como chips removíveis.

## Acessibilidade
- Semântica correta por estrutura (`<table>` real; `<ul>` para listas).
- Seleção em massa anuncia a contagem; ações por item alcançáveis pelo teclado.

## Do / Don't
- ✅ A mesma coleção pode oferecer uma troca de visualização (table/cards) preservando filtros.
- ❌ Scroll horizontal de tabela como único fallback mobile.
