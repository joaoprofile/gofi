# Report/Export (XLSX/CSV) — `services/common/report`

Framework canônico do projeto para gerar **arquivos de export** (XLSX/CSV) a
partir de filtros dinâmicos. **Nunca** usar `excelize` direto no handler nem
recriar gerador de relatório — sempre passar por `services/common/report`.
(Legado `wb_core/report` é proibido em código novo.)

## Peças do framework (`services/common/report`)

- **`Report` interface** (`contract.go`) — o que cada tipo de report implementa:
  ```go
  GenerateReportXLSX(ctx, *sqln.Filters) ([]byte, error)
  GenerateReportCSV(ctx, *sqln.Filters) (*[]byte, error)
  ValidateReport(ctx, *excelize.File, companyID string) []errs.AppError // só p/ import; export-only retorna erro estável NotSupported
  ```
- **`Register(reportType string, ctor Constructor)`** — registry global. `Constructor = func(language string) Report`. Registrar no **wiring do binário** (composition root), não em `init()`.
- **`NewFactory(*Params) (*Factory, error)`** + **`Factory.Generate(ctx, *sqln.Filters) (*[]byte, error)`** — resolve o tipo no registry e gera no formato. `Params{Language, Type, Format, Filters}`.
- **`WriteFile(w, body, filename, contentType)`** + **`Filename(type, lang, format, at)`** + **`ContentType(format)`** + **`Extension`** — helpers HTTP de download.
- **`ParseFormat(v, fallback)` / `ParseLanguage` / `ParseType(v, allowed, fallback)`** — parse de query params. Consts `FormatXLSX/CSV`, `LanguagePT/EN/ES`.
- **`WriteDynamic(w, r, DynamicRequest{Tenant, Type, Format, Language, Mapping, DefaultSort*})`** — atalho HTTP que faz: parse do `*sqln.Filters` do **corpo** (POST) + valida contra `QueryMapping` + injeta tenant + default sort + Factory + WriteFile. **Exige `Tenant != ""`** → serve export **tenant-scoped** (seller exporta o próprio dado).

## Dois caminhos de endpoint

- **Tenant-scoped (POST + body de filtros)** → `report.WriteDynamic(...)`. Tenant obrigatório.
- **Admin cross-tenant (GET + query params)** → **NÃO** usar `WriteDynamic` (rejeita tenant vazio). Montar `*sqln.Filters` à mão e usar `Factory`+`WriteFile`:
  ```go
  filters := sqln.NewFilters()
  if companyID != "" { filters.Add(sqln.NewFilter("n.core_company_id", "=", companyID)) }
  if start != ""     { filters.Add(sqln.NewFilter("n.responded_at", ">=", start+" 00:00:00")) }
  if end != ""       { filters.Add(sqln.NewFilter("n.responded_at", "<=", end+" 23:59:59")) }
  format := report.ParseFormat(r.URL.Query().Get("format"), report.FormatXLSX)
  lang   := report.ParseLanguage(r.URL.Query().Get("language"), report.LanguagePT)
  f, err := report.NewFactory(&report.Params{Language: string(lang), Type: "NPS", Format: string(format), Filters: filters})
  body, err := f.Generate(r.Context(), filters)
  report.WriteFile(w, *body, report.Filename("NPS", lang, format, time.Now()), report.ContentType(format))
  ```

## Implementação do `Report` (por contexto)

Mora em `services/domain/{área}/{ctx}/report/` (pacote `{ctx}report` p/ evitar
conflito com `services/common/report`, importado como `report`). Arquivos:
`{ctx}_report.go` (impl + `New` + `Register`) + `errors.go` (`Err{Ctx}ReportNotSupported`).

A query usa o **filtro dinâmico do sqln** — base query + `NewQueryBuild` (os
filtros entram como `AND (...)` após o `WHERE` da base):
```go
const baseQuery = `SELECT ... FROM {tabela} x INNER JOIN ... WHERE x.<cond_base>`
func (rp *xReport) rows(ctx, filters *sqln.Filters) ([]model.ExportRow, error) {
    return sqln.FindWithFilter[model.ExportRow](ctx, sqln.NewQueryBuild(baseQuery, filters)).List()
}
```
> A base query **já tem `WHERE`** (condição sempre-presente, ex.: `responded_at IS NOT NULL`); `NewQueryBuild` **appenda os filtros do usuário como `AND`**. Tenant-scoped: `fmt.Sprintf(base, filters.Tenant)` injeta o predicado de tenancy na base (ver `product_pim` `FindForReportExport`). `model.ExportRow` precisa de tags `db` na **mesma ordem** do SELECT (scan posicional do sqln).

XLSX via `excelize`: `f := excelize.NewFile(); f.SetSheetName(f.GetSheetName(0), "Aba"); excelize.CoordinatesToCellName(col, row); f.SetCellValue(...); buf,_ := f.WriteToBuffer()`. CSV via `encoding/csv`.

## Wiring

No `wire.go`/`main.go` do binário: chamar `{ctx}report.Register()` uma vez no
boot (antes de servir). Sem registro, `NewFactory` devolve "invalid report type".

## Anti-padrões

- `excelize` direto no handler (sem passar pelo framework) — **MAJOR**.
- Importar `wb_core/report` (legado) em código novo — **proibido**.
- `WriteDynamic` para export admin cross-tenant (quebra no `Tenant == ""`).
- `Register` via `init()` (registro mágico) — registrar no composition root.
- Esquecer a condição-base no `WHERE` da base query → `NewQueryBuild` gera SQL inválido quando há 0 filtros.
