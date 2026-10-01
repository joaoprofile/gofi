---
name: value-objects
description: Mapeamento linha→struct do sqln (tags db) — por nome ou por posição, value objects aninhados, coluna JSON, arrays
sdk: v0.8.2
keywords: [mapping, db-tag, value-object, scan, nested-struct, sql.Scanner, jsonb, array]
---

# Mapeamento de linhas e Value Objects — `sqln/mapping`

API: `.claude/sdk/go/api/sqln-mapping.md`. Todo `sqln.Find*` escaneia em `T`
por este mapper; `mapping.GetMappedCols(&T{})` expõe os destinos (útil em
teste).

## Como o mapper decide

1. **Folhas** de `T` = campos com tag `db`, na ordem de declaração; struct
   aninhada com tag `db` é expandida (ver VO abaixo). `time.Time`, tipos que
   implementam `sql.Scanner` e `[]byte` são folha única; outros slices são
   lidos como **array PostgreSQL** (via pgx).
2. **Por nome**, quando as colunas do resultado cobrem **todas** as folhas:
   nome da coluna (ou alias) = nome da tag, sem diferenciar maiúsculas. Ordem
   das colunas não importa e colunas extras são ignoradas.
3. **Por posição** (fallback) quando algum nome falta, ou quando duas folhas
   têm a mesma tag, ou quando alguma folha tem `db:"-"`: coluna *i* → folha
   *i*, e a contagem precisa bater.

**Regra:** escreva o `SELECT` para casar **por nome** — cada folha com
coluna ou alias de mesmo nome (`c.slug AS category` para `db:"category"`).
Assim reordenar colunas ou juntar tabelas não quebra o scan. Posição é só
fallback; não dependa dele.

## Value Object em colunas separadas (recursivo)

```go
type Address struct {
    Street  string `json:"street"  db:"street"`
    City    string `json:"city"    db:"city"`
    ZipCode string `json:"zipCode" db:"zip_code"`
}

type Customer struct {
    ID      int64   `json:"id"      db:"id"`
    Name    string  `json:"name"    db:"name"`
    Address Address `json:"address" db:"address"` // tag externa = marcador; não é coluna
}
```

`SELECT c.id, c.name, c.street, c.city, c.zip_code FROM customer c` — as
folhas são `id, name, street, city, zip_code` (nomes das tags **internas**).
Vários níveis funcionam desde que cada nível tenha tag `db`.

## Value Object em coluna única (JSON/bytes)

Implemente `sql.Scanner` + `driver.Valuer`; o mapper trata como folha única:

```go
type Metadata map[string]any

func (m *Metadata) Scan(src any) error {
    b, ok := src.([]byte)
    if !ok {
        return errors.New("metadata: expected []byte")
    }
    return json.Unmarshal(b, m)
}

func (m Metadata) Value() (driver.Value, error) { return json.Marshal(m) }
```

## Armadilhas

- **Struct para coluna JSONB sem `sql.Scanner`** (sub-campos só com `json`):
  o mapper tenta recursar, acha **zero** folhas e o campo some do scan →
  `sql: expected N destination arguments in Scan, not N-1` em runtime, em
  toda leitura do tipo (a escrita segue funcionando). Coluna JSON = tipo com
  `Scan`/`Value`.
- **Campo calculado** (preenchido pelo service depois do scan): só tag
  `json`, **sem** `db`. `db:"-"` **não** exclui — a presença da tag conta
  como folha (e, por não ter nome, derruba o casamento por nome).
- **Sem tag `db` no campo externo** do VO → o VO inteiro é ignorado sem erro.
- **Tag duplicada** (duas folhas `db:"id"` em VOs diferentes) → cai para
  posição. Dê nomes únicos (alias no SQL + tag distinta).
- `Scan` de `NULL` em campo não-ponteiro falha só nas linhas com `NULL`: use
  `*T`/`sql.Null*` ou `COALESCE` onde a coluna aceita nulo.
- Tipo primitivo (`FindFromCriteria[bool]`, `[int64]`) escaneia a **primeira
  coluna** direto — selecione exatamente uma coluna compatível.

## Mudou a struct? Raio de alcance

Todo repositório que escaneia o mesmo `model.{Tipo}` é afetado. Antes de
fechar:

1. Localize os consumidores (`gofi show {Tipo}`,
   `gofi find --regex "Find(FromCriteria|WithFilter)?\[.*{Tipo}\]" --in code`,
   `gofi find --text "{Tipo}SelectFields" --in code`).
2. Cada `SELECT` que cai em `T` precisa trazer a coluna nova com o nome da
   tag (e o `Join` que a produz).
3. `go build`/`go test` sem banco **não** pegam quebra de scan: exercite a
   query contra um banco (teste de integração) ou compare
   `len(mapping.GetMappedCols(&T{}))` com as colunas de cada
   `*SelectFields`.

## Onde documentar

- Spec §3.3 Value Objects — sub-campos, estratégia (colunas separadas vs
  coluna única), justificativa.
- `model/entity.go` — VO como tipo próprio, referenciado pela entidade raiz.
- Migration — colunas do VO com os nomes das tags.
