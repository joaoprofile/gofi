# Value Objects Aninhados — Padrão gofi/sqln

## Contexto

O mapper em `gofi/sqln/mapping/mapper.go` expande recursivamente structs aninhadas com tag `db` — transformando value objects no domínio Go em colunas simples na query SQL, sem precisar implementar `sql.Scanner` manualmente.

Use quando:
- Um grupo de atributos faz sentido como VO no domínio (`Money`, `Address`, `Pricing`, `Coordinates`)
- O banco guarda os valores em **colunas separadas** (uma coluna por sub-campo)
- Não há necessidade de (de)serializar o VO inteiro como um blob

## Regras do mapper

1. **Tag `db` no campo externo é obrigatória** — sem ela o campo inteiro é ignorado
2. **Ordem dos sub-campos internos define o mapeamento posicional** com as colunas do `SELECT` — não há "resolução por nome"
3. **Múltiplos níveis suportados** — `A.B.C.valor` funciona enquanto houver tag `db` em cada nível
4. **`time.Time` e `sql.Scanner` não recursam** — são tratados como primitivos (o driver sabe escanear)
5. **Slices com tag `db`** usam `pq.Array` automaticamente — não recursam
6. **Sem tag `db` em sub-campo** → esse sub-campo é ignorado pelo scan
7. **`db:"-"` NÃO exclui o campo do scan** — o critério de inclusão é
   `f.Tag.Lookup("db")` (a *presença* da tag, qualquer valor, inclusive `"-"`),
   não o valor. Um campo **computado/derivado** (preenchido pelo service após o
   scan — ex.: `SupportedTypes []string` resolvido por capability, flags
   calculadas, agregados montados em memória) deve ter **apenas a tag `json`,
   sem `db` nenhuma**. Pôr `db:"-"` nele faz o mapper contá-lo como coluna
   (e, se for slice, vira destino `pq.Array`), gerando em runtime
   `sql: expected N destination arguments in Scan, not N+1` quando o `SELECT`
   tem N colunas. Regra: *campo que o banco não devolve = sem tag `db`*.

## Exemplo — VO em colunas separadas (padrão recursivo)

```go
type Pricing struct {
    Price float64 `json:"price" db:"price"`
}

type Product struct {
    ID    int64   `json:"id"      db:"id"`
    Name  string  `json:"name"    db:"name"`
    Price Pricing `json:"pricing" db:"price"`  // tag externa é marcador de presença
}
```

Query:
```sql
SELECT id, name, price FROM product
```

Scan mapping:
- coluna 0 `id` → `Product.ID`
- coluna 1 `name` → `Product.Name`
- coluna 2 `price` → `Product.Price.Price` (recursa em `Pricing`)

## Exemplo — VO multi-campo em múltiplas colunas

```go
type Address struct {
    Street  string `json:"street"  db:"street"`
    City    string `json:"city"    db:"city"`
    ZipCode string `json:"zipCode" db:"zip_code"`
}

type Customer struct {
    ID      int64   `json:"id"      db:"id"`
    Name    string  `json:"name"    db:"name"`
    Address Address `json:"address" db:"address"`  // marcador
}
```

Query **deve** selecionar as colunas na ordem dos sub-campos:
```sql
SELECT id, name, street, city, zip_code FROM customer
```

## Exemplo — VO armazenado em coluna única (JSON/bytes)

Quando o VO é serializado inteiro (ex: JSONB no PostgreSQL), **não use struct aninhada recursiva** — implemente `sql.Scanner` e `driver.Valuer`. O mapper trata o campo como primitivo.

```go
type Metadata map[string]any

func (m *Metadata) Scan(src any) error {
    b, ok := src.([]byte)
    if !ok {
        return errors.New("metadata: expected []byte")
    }
    return json.Unmarshal(b, m)
}

func (m Metadata) Value() (driver.Value, error) {
    return json.Marshal(m)
}

type Product struct {
    ID       int64    `json:"id"       db:"id"`
    Metadata Metadata `json:"metadata" db:"metadata"`  // coluna única JSONB
}
```

## Armadilhas comuns

- **Colunas fora de ordem no `SELECT`** — como o mapeamento é posicional, reordenar `SELECT price, name, id` quebra o scan. Sempre alinhar a ordem do `SELECT` com a ordem dos sub-campos da struct
- **Esquecer a tag `db` externa** — o mapper pula o campo e os valores do VO ficam zero sem erro de scan
- **Mix de strategies** — se o VO implementa `sql.Scanner`, ele não recursa, mesmo que tenha sub-campos com `db`. Escolha uma estratégia por VO
- **Adicionar sub-campo no meio** — muda a ordem posicional e quebra queries existentes. Ao estender um VO, adicione o novo sub-campo **no fim** e ajuste todos os `SELECT` correspondentes

## O contrato posicional é do TIPO, não do repo

`FindFromCriteria[T]` escaneia via `rows.Scan(GetMappedCols(&T)...)`
(`mapping/mapper.go`). `GetMappedCols` gera **um destino por folha `db`-tagueada
de `T`, na ordem de declaração** — VO aninhado expande in-place. A string do
`Select(...)` é só a cláusula SQL: **não** participa do mapeamento e **não**
realinha por nome. Logo, **toda** query que escaneia em `T` carrega um contrato
implícito: a lista de colunas do `SELECT` precisa bater com as folhas de `T` em
**contagem E ordem**.

Consequências ao **mudar a struct** (campo novo, remoção, reordenação):

- **Aridade:** `SELECT` com menos/mais colunas que folhas → driver retorna
  `sql: expected N destination arguments in Scan, not M`. Erro de runtime, não
  de compilação — só aparece quando a query roda.
- **Desalinhamento silencioso:** mesma contagem, ordem trocada → cada coluna cai
  no campo errado. Sem erro se os tipos forem compatíveis (string↔*string);
  `Scan` de `NULL` num campo não-pointer ou de não-JSON num `sql.Scanner`
  (JSON map) estoura só em linhas específicas.
- **Raio de alcance = todos os repos que escaneiam `T`.** Quando o **mesmo**
  `model.{Type}` é materializado por mais de um repository (o repo dono +
  repos de outros contextos que leem a mesma tabela), **cada** `SELECT` que cai
  em `T` tem que ser atualizado junto. Um deles ficar para trás = quebra só
  naquele caminho (ex.: contexto consumidor quebra, o dono continua passando).

Regras ao estender/alterar uma struct escaneada:

1. Adicione o campo **na posição certa** da struct e replique a **mesma ordem**
   em **todos** os `SELECT` que escaneiam `T` — não só no repo que você abriu.
2. Localize os consumidores antes de fechar:
   `grep -rn "FindFromCriteria\[.*{Type}\]"` + `grep -rn "{Type}SelectFields"`
   + `grep -rn "model.{Type}\b"`.
3. Campo que vem de tabela joinada (ex.: nome resolvido de outra tabela) exige
   o `.Join(...)` correspondente em **cada** query — senão a coluna não existe
   no resultset e a aridade quebra.
4. `go build` + `go test` de **todos** os pacotes consumidores, não só do que
   você editou.

## Onde documentar

- **Spec** §3.3 Value Objects Aninhados — tabela com sub-campos, estratégia de persistência, justificativa
- **entity.go** — struct do VO como tipo separado, referenciada pela entidade raiz
- **migration** — colunas do VO na ordem esperada pela struct
