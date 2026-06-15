# Pattern — Navegação

Como o usuário se move pela aplicação e se mantém orientado.

## Hierarquia de navegação
| Nível | Componente |
|-------|-----------|
| Global (destinos da aplicação) | sidebar / bottom nav ([app-shell](app-shell.md)) |
| Local (dentro de uma seção) | [Tabs](../components/tabs.md) / [Segmented Control](../components/segmented-control.md) |
| Localização (onde estou) | Breadcrumbs |
| Contextual (ações) | [Menu](../components/menu-popover.md) |

## Breadcrumbs
- `<nav aria-label="Breadcrumb">` → `Início / Seção / Página atual`
  (`aria-current="page"` no último, sem link). Truncar no meio em trilhas longas,
  não no fim.
- Aparecem em páginas de detalhe profundas, não na home.

## Rotas
- A URL é a fonte da verdade para o estado navegável (tab ativa, página,
  filtros deep-linkáveis quando fizer sentido).
- **Lazy-load** de rotas fora do caminho crítico (> ~100kb) com um
  [Skeleton](../components/skeleton-spinner.md) de fallback.
- O estado da rota ativa se reflete na sidebar/tab (`aria-current`).

## Voltar e contexto
- A ação de "voltar" preserva o scroll e o estado da lista anterior.
- Em um fluxo de múltiplas etapas use um [Stepper](../components/stepper.md), não tabs.

## Acessibilidade
- Landmarks consistentes e `aria-current`. O foco vai para o topo do novo conteúdo
  (ou para o heading) na troca de rota — anuncie a mudança de página.
- Skip link para `<main>`.

## Do / Don't
- ✅ O usuário sempre sabe **onde está** e **como voltar**.
- ❌ Navegação que perde trabalho não salvo sem aviso (princípio 3 — lapsos).
