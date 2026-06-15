# Table / DataTable

Dados tabulares densos. Núcleo do template de dashboard (referência visual web).

## Anatomia
```
[ toolbar: busca · filtros · ordenação · ações em massa ]
[ thead: colunas (ordenáveis?) ]
[ tbody: linhas — uma célula pode conter avatar+texto, progress, badge, ação ]
[ footer: paginação ]
```
Linhas com hover `--sf-hover`, divisores `--sf-border`, números tabulares.

## Recursos
| Recurso | Nota |
|---------|------|
| Ordenar por coluna | `aria-sort` no `<th>` ativo |
| Seleção (checkbox) | header com "selecionar tudo" (indeterminate) |
| Célula rica | avatar+nome, [Progress](progress.md), [Badge](badge-tag.md), ação [Button](button.md) |
| Densidade | `comfortable` / `compact` |
| Paginação | [Pagination](pagination.md) no rodapé |
| Responsivo | no mobile vira uma lista de cards (cada linha = um card) |

## Estados (obrigatórios)
`loading (skeleton de linha) · empty ([EmptyState](empty-state.md)) · error (retry) · success`.

## Props
```ts
interface Column<T> { key: keyof T; header: string; sortable?: boolean; align?: 'start'|'end';
  render?: (row: T) => ReactNode; }
interface TableProps<T> { columns: Column<T>[]; rows: T[]; loading?: boolean;
  sort?: { key: string; dir: 'asc'|'desc' }; onSort?: (key: string) => void;
  selectable?: boolean; rowKey: (row: T) => string; emptyState?: ReactNode; }
```

## Acessibilidade
- `<table>` semântica: `<thead>/<tbody>/<th scope="col">`. Nunca uma grade de `<div>`
  sem semântica.
- Coluna ordenável: `<th aria-sort="ascending|descending|none">` + um botão no header.
- Loading: região `aria-busy`; skeleton `aria-hidden`.
- Ação por linha alcançável pelo teclado; para ações em massa, anuncie quantas estão selecionadas.

## Do / Don't
- ✅ No mobile, **colapse em cards** (uma coluna), não scroll horizontal infinito.
- ✅ A coluna de progresso usa [Progress](progress.md) + um label textual ("14/22").
- ❌ Ordenar sem indicador visual/`aria-sort`. ❌ Uma tabela sem estados empty/error.

## Exemplo
```tsx
<Table rowKey={r => r.id} columns={cols} rows={data} loading={isLoading}
       sort={sort} onSort={setSort} emptyState={<EmptyState /* … */ />} />
```
