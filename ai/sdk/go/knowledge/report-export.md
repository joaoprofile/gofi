---
name: report-export
description: Export XLSX/CSV a partir do filtro dinâmico — mesma allowlist, leitura em streaming com .All() do manager, escrita no handler de download
sdk: v0.8.2
keywords: [report, export, xlsx, csv, dynamic-filter, FilterMapping, All, streaming, download]
---

# Report/Export (XLSX/CSV)

Export de listagem = **o mesmo filtro dinâmico** do contexto
(`dynamic-filter.md`), sem paginação, lido em streaming e escrito como
arquivo. Se o projeto tem um pacote comum de relatórios (registry de tipos,
helpers de download), ele é do projeto — use-o e registre-o em
`.claude/memory/project.md`; aqui fica só o que vale para qualquer projeto
com o SDK.

## Regras

- **Mesma allowlist:** o export aceita o mesmo `sqln.Filters` e valida pelo
  mesmo `model.{Ctx}FilterMapping` — nunca uma segunda lista de colunas, nunca
  `field` cru do cliente.
- **Tenant como argumento** da base query (`$1`), igual à listagem. Export
  administrativo cross-tenant é endpoint separado, com autorização própria e
  base sem o predicado de tenant — nunca o mesmo endpoint com tenant opcional.
- **Streaming com `.All()`:** export não pagina e pode ter muitas linhas;
  `.All()` entrega linha a linha sem montar o slice (sem cache). Ordene com
  `ORDER BY` na base (não há `PageRequest`).
- **Formato fora do repository:** o repository devolve o iterador de linhas;
  a montagem XLSX/CSV fica no handler ou num pacote `report` do contexto.

## Repository

```go
const {ctx}ExportQueryBase = `SELECT ` + {ctx}ExportSelectFields + `
FROM {tabela} e
WHERE e.tenant_id = $1`

func (r *{ctx}Repository) Export{Ctx}(ctx context.Context, tenantID string, f *sqln.Filters) (iter.Seq2[model.{Ctx}ExportRow, error], error) {
    q, err := sqln.BuildQuery({ctx}ExportQueryBase, []any{tenantID}, f, model.{Ctx}FilterMapping, nil)
    if err != nil {
        return nil, err // embrulha sqln.ErrInvalidFilter → 400 no service
    }
    q.Query += " ORDER BY e.created_at"
    return sqln.FindWithFilter[model.{Ctx}ExportRow](ctx, q).All(), nil
}
```

`model.{Ctx}ExportRow` tem tags `db` com os nomes/aliases das colunas
(`value-objects.md`).

## Escrita

```go
rows, err := repo.Export{Ctx}(ctx, tenantID, filters)
// ... erro → 400/500 antes de escrever qualquer byte
w.Header().Set("Content-Type", "text/csv")
w.Header().Set("Content-Disposition", `attachment; filename="{ctx}.csv"`)
cw := csv.NewWriter(w)
for row, err := range rows {
    if err != nil {
        logging.Error("{ctx} export failed", slog.Any("error", err)) // cabeçalho já enviado
        return
    }
    _ = cw.Write(row.CSV())
}
cw.Flush()
```

- CSV via `encoding/csv`, escrevendo direto no `http.ResponseWriter`.
- XLSX (lib de planilha adotada pelo projeto) monta em memória: imponha o teto
  de linhas declarado na spec, ou gere de forma assíncrona (job + bucket)
  acima dele.
- Erro no meio do stream não vira status HTTP (o cabeçalho já saiu): logue e
  encerre. Valide filtros **antes** de escrever.

## Anti-padrões

- Export com `.List()`/`PagedList()` de limite gigante (memória e `COUNT(*)`
  à toa).
- Mapping de export diferente do da listagem.
- Tenant formatado na string (`fmt.Sprintf`) ou lido de `filters.Tenant`.
- Geração de planilha dentro do repository.
