---
name: layer-model
description: Resumo da camada model em Go — entidade com tags db/json, DTOs com Validate, filtro de listagem, FilterMapping, Value Objects e as armadilhas do mapper
sdk: v0.2.18
keywords: [camada, model, entity, dto, query_dto, tags db, tags json, validate, form, FilterMapping, value object, Aggregate]
---

# Camada Model — resumo

Leia este primeiro; o documento completo só na situação da última seção.

## O que faz e o que não faz

**Faz:** declara entidade, DTOs de entrada/saída, filtros, Value Objects,
structs `{Aggregate}Aggregate` e a allowlist do filtro dinâmico.

**Não faz:** I/O, regra de orquestração, leitura de ambiente, acesso a banco.
Depende só de `base/validator` (`dto.go`), `sqln` e `<module>/common/enums`
(`query_dto.go`).

## Arquivos

| Arquivo | Conteúdo | Tags |
|---|---|---|
| `entity.go` | struct mapeada do banco | `db`, `json` |
| `dto.go` | entrada/saída da API, filtro de listagem | `validate`, `json`, `form` |
| `query_dto.go` | só com filtro dinâmico: `FilterMapping` e read model | `db`, `json` |

`//gofi:context {contexto}` abre a cláusula `package` em **um** arquivo do
pacote, com o mesmo nome (kebab-case) de `specs/{contexto}/`.

## Padrões

```go
//gofi:context {contexto}
package model

type Entity struct {
	ID        string    `json:"id"        db:"id"`
	TenantID  string    `json:"-"         db:"tenant_id"`
	CreatedAt time.Time `json:"createdAt" db:"created_at"`
}

var v = validator.New() // singleton de pacote, nunca por request

type CreateEntityRequest struct {
	Email string `json:"email" validate:"required,email"`
}

func (r CreateEntityRequest) Validate() error { return v.ValidateStruct(r) }

type EntityFilter struct {
	TenantID string   `form:"-"` // vem do auth, nunca da query
	Statuses []string `form:"status"`
	Page     uint16   `form:"page"`
	Limit    uint16   `form:"limit" validate:"omitempty,lte=200"`
}
```

```go
// query_dto.go — Column nunca é serializada
var EntityFilterMapping = sqln.FilterMapping{
	"name":    {Column: "e.name", Ops: sqln.Text, Sortable: true, Label: "NAME", FilterType: "text"},
	"created": {Column: "e.created_at", Ops: sqln.Range, Sortable: true, Label: "CREATED_AT"},
}
```

Listagem paginada usa `*sqln.Page[Entity]` direto nas assinaturas, sem alias de
tipo ponteiro. Read model próprio (`EntityQuery`) só quando a projeção difere
da entidade.

## Regras que mais causam erro

- **Nomes:** campo `PascalCase` em inglês; `db` em `snake_case`, igual à
  coluna; `json` em `camelCase`. Código de moeda é sempre `json:"currencyCode"`
  e de país, `json:"countryCode"`.
- **Tenancy nunca vem do cliente:** `json:"-"` / `form:"-"`. Para o bind,
  `form:"-"` **não** significa "pular": o `httpx` o trata como parâmetro
  chamado `-`. O handler atribui o tenant **depois** do bind.
- **`Validate()`:** obrigatório em Create/Update. Em filtro, só quando há tag
  `validate` (ex.: teto de `limit`). Quem chama é o service.
- **Campo calculado** depois do scan: só `json`, **sem** `db`. `db:"-"` não
  exclui: a tag conta como folha e derruba o casamento por nome.
- **Mapper:** casa por nome quando o resultado cobre todas as tags; a ordem das
  colunas então não importa. Tag duplicada em VOs diferentes faz cair para
  posição: use alias no SQL e tag distinta.
- **VO em colunas separadas:** o campo externo **precisa** de `db`; sem ela o
  VO inteiro é ignorado sem erro.
- **VO em coluna JSON:** tipo com `sql.Scanner` + `driver.Valuer`. Sem isso, a
  leitura quebra em runtime com `expected N destination arguments`, e a escrita
  segue funcionando.
- **`NULL`:** coluna que aceita nulo pede `*T`/`sql.Null*` ou `COALESCE`.
- **IDs de `tenant`/`user` (e FKs):** `string` com UUIDv7 gerado na aplicação;
  DTO usa `validate:"uuid"`. Demais entidades: `BIGINT IDENTITY`.
- **Enums:** o enum mora em `<module>/common/enums`; o mapping
  referencia por `Content`, com `SearchType: "embedded"`.
- **`query_dto.go`** só existe se o contexto expõe `/schemas` + `/query`.

## Quando abrir o detalhe

| Documento | Abra quando |
|---|---|
| `.claude/sdk/go/boilerplates/model.md` | for criar os arquivos do zero |
| `.claude/sdk/go/knowledge/value-objects.md` | houver VO, coluna JSON ou erro de scan |
| `.claude/sdk/go/knowledge/validation.md` | precisar de tag cross-field, `details` do 400 ou config de motor de decisão |
| `.claude/sdk/go/knowledge/dynamic-filter.md` | o contexto expuser `/schemas` + `/query` |
| `.claude/sdk/go/knowledge/lookup-endpoints.md` | o filtro tiver dropdown (`FilterType`, `SearchType`, `Content`) |
| `.claude/sdk/go/knowledge/repository-aggregate-pattern.md` | for modelar mutação multi-tabela |
| `.claude/sdk/go/knowledge/naming.md` | restar dúvida de nome de arquivo, tipo ou tag |
