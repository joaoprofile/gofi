# Boilerplate — Repository

```go
package repository

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"

	"github.com/joaoprofile/examples/api/src/person/model"
	"github.com/joaoprofile/gofi/obs/logging"
	"github.com/joaoprofile/gofi/sqln"
	"github.com/joaoprofile/gofi/sqln/criteria"
)

// ErrNoRowsAffected is kept for compatibility but Update methods no longer return it.
// Service-layer not-found is handled by pre-checking FindByID before calling Update.

const (
	personSelectFields = "p.id, p.name, p.email, p.cpf, p.age, p.created_at"

	personInsertQuery = `INSERT INTO persons (name, email, cpf, age) VALUES ($1, $2, $3, $4)`
	personUpdateQuery = `UPDATE persons SET name=$1, email=$2, cpf=$3, age=$4 WHERE id=$5`
	personDeleteQuery = `DELETE FROM persons WHERE id = $1`
)

type PersonRepository interface {
	Save(ctx context.Context, person model.Person) error
	Update(ctx context.Context, id string, req model.UpdatePersonRequest) error
	Delete(ctx context.Context, id string) error
	FindByFilter(ctx context.Context, filter model.PersonFilter) (model.PersonResponse, error)
	FindByID(ctx context.Context, id string) (*model.Person, error)
	Close() error
}

type personRepository struct {
	stmCreate *sql.Stmt
	stmUpdate *sql.Stmt
	stmDelete *sql.Stmt
}

func NewPersonRepository(ctx context.Context) PersonRepository {
	stmCreate, err := sqln.NewStatement().Prepare(ctx, personInsertQuery)
	if err != nil {
		logging.Fatal("error on NewPersonRepository: stmCreate", slog.Any("error", err))
	}
	stmUpdate, err := sqln.NewStatement().Prepare(ctx, personUpdateQuery)
	if err != nil {
		logging.Fatal("error on NewPersonRepository: stmUpdate", slog.Any("error", err))
	}
	stmDelete, err := sqln.NewStatement().Prepare(ctx, personDeleteQuery)
	if err != nil {
		logging.Fatal("error on NewPersonRepository: stmDelete", slog.Any("error", err))
	}
	return &personRepository{
		stmCreate: stmCreate,
		stmUpdate: stmUpdate,
		stmDelete: stmDelete,
	}
}

func (r *personRepository) Save(ctx context.Context, person model.Person) error {
	_, err := r.stmCreate.ExecContext(ctx, person.Name, person.Email, person.CPF, person.Age)
	return err
}

func (r *personRepository) Update(ctx context.Context, id string, req model.UpdatePersonRequest) error {
	_, err := r.stmUpdate.ExecContext(ctx, req.Name, req.Email, req.CPF, req.Age, id)
	return err
}

func (r *personRepository) Delete(ctx context.Context, id string) error {
	_, err := r.stmDelete.ExecContext(ctx, id)
	return err
}

func (r *personRepository) FindByFilter(ctx context.Context, f model.PersonFilter) (model.PersonResponse, error) {
	q := criteria.From("persons", "p").Select(personSelectFields)

	switch {
	case f.CPF != "":
		q = q.Where(criteria.Eq("p.cpf", f.CPF))
	case f.Name != "":
		q = q.Where(criteria.Contains("p.name", "%"+f.Name+"%"))
	}

	page := sqln.NewPageRequest(f.Page, f.Limit, []sqln.Sort{
		sqln.NewSort("p.created_at", sqln.DESC),
	})

	return sqln.FindFromCriteria[model.Person](ctx, q).WithPage(page).PagedList()
}

func (r *personRepository) FindByID(ctx context.Context, id string) (*model.Person, error) {
	return sqln.FindFromCriteria[model.Person](ctx,
		criteria.From("persons", "p").
			Select(personSelectFields).
			Where(criteria.Eq("p.id", id)),
	).Execute()
}

func (r *personRepository) Close() error {
	if err := r.stmCreate.Close(); err != nil {
		return err
	}
	if err := r.stmUpdate.Close(); err != nil {
		return err
	}
	return r.stmDelete.Close()
}
```

## Filtro Dinâmico — FindByDynamicQuery

Adicionado à interface e implementação quando o contexto usa filtro dinâmico. **Não usa statements preparados** — não há `stmDynamicQuery` no construtor.

```go
// Na interface
FindByDynamicQuery(ctx context.Context, filters *sqln.Filters) (model.PersonQueryResponse, error)

// Constante separada para o read model de query (pode ter campos diferentes da entidade)
const personQuerySelectFields = "p.id, p.name, p.email, p.cpf, p.age, p.created_at"

// Na implementação
func (r *personRepository) FindByDynamicQuery(ctx context.Context, f *sqln.Filters) (model.PersonQueryResponse, error) {
    // WHERE 1=1 obrigatório — NewQueryBuild anexa AND (...), nunca WHERE
    query := "SELECT " + personQuerySelectFields + " FROM person WHERE 1=1"

    return sqln.FindWithFilter[model.PersonQuery](ctx,
        sqln.NewQueryBuild(query, f),
    ).WithPage(
        sqln.NewPageRequestFilter(f),
    ).PagedList()
}
```

**Regras:**
- `FindWithFilter` — não `FindFromCriteria` (aceita `*QueryParam`, não `*criteria.Query`)
- `NewQueryBuild(query, f)` — PostgreSQL por default. Para outros bancos: `NewQueryBuildWithDialect(query, f, dialect)`
- `NewPageRequestFilter(f)` — extrai page/limit/sort de `f.Params` (defaults: page=0, limit=15, sort="id ASC")
- A query base **deve terminar com `WHERE 1=1`** (ou condição real) — `NewQueryBuild` anexa `AND ( conditions )`, nunca `WHERE`
- Usar `personQuerySelectFields` (constante separada) quando read model tem campos diferentes da entidade
- **Não** criar `*sql.Stmt` para query dinâmica — ela é construída em tempo de execução

## Cache — Padrão com WithCache

Quando o contexto usa Redis, cache de listas fica no repository. Inclua `cacheTTL`, `WithCache` em `FindByFilter` e `InvalidateListCache` na interface:

```go
const cacheTTL = 10 * time.Minute

type PersonRepository interface {
    // ...
    InvalidateListCache(ctx context.Context, tenantID string)
    Close() error
}

func (r *personRepository) FindByFilter(ctx context.Context, f model.PersonFilter) (model.PersonResponse, error) {
    q := criteria.From("persons", "p").Select(personSelectFields)
    // ... where clauses ...

    page := sqln.NewPageRequest(f.Page, f.Limit, []sqln.Sort{
        sqln.NewSort("p.created_at", sqln.DESC),
    })

    cacheKey := fmt.Sprintf("person:%s:list:%s:%d:%d", f.TenantID, f.Name, f.Page, f.Limit)
    cache := sqln.NewCache[model.Person](cacheKey, cacheTTL)

    return sqln.FindFromCriteria[model.Person](ctx, q).
        WithCache(cache).
        WithPage(page).PagedList()
}

func (r *personRepository) InvalidateListCache(ctx context.Context, tenantID string) {
    pattern := fmt.Sprintf("*person:%s:list:*", tenantID)
    keys, err := sqln.InstanceRedis().Keys(ctx, pattern).Result()
    if err != nil || len(keys) == 0 {
        return
    }
    sqln.InstanceRedis().Del(ctx, keys...)
}
```

O service chama `repo.InvalidateListCache(ctx, tenantID)` após Save e Update. **Nunca** gerencia cache ou acessa Redis diretamente no service.

## Padrões Obrigatórios

- `Update` retorna apenas erros estruturais do banco — nunca checa `RowsAffected`. Not-found é detectado pelo `FindByID` que o service chama antes de `Update`
- **Statements preparados no construtor — nunca inline por chamada.** `*sql.Stmt` em campo do struct (`stmCreate`, `stmUpdate`, `stmDelete`), preparado **uma única vez** em `New{Contexto}Repository(ctx)`. `sqln.NewStatement().Execute(ctx, sql, args...)` inline em mutation (prepara + executa + descarta a cada chamada) é **MAJOR**. Exceção: SQL dinâmico montado em runtime (filtro dinâmico) não pode ser preparado.
- **Helpers de persistência são métodos do receiver.** Nenhuma função no arquivo do repo com assinatura `func xxx(ctx context.Context, ...) error` executando SQL — todas são `func (r *{contexto}Repository) ...`. Helper solto no pacote (sem receiver) é **MAJOR** — perde acesso aos stmts preparados, borra a fronteira de encapsulamento e convida outros pacotes a importarem.
- **Exceção pra função de pacote**: transformações **puras** sem `ctx`/I/O (ex.: `configArgs(e *Config) []any`, `groupArgs(e *Group) []any`) podem ficar como funções privadas no pacote — não tocam banco, não precisam de `r`, ajudam a não duplicar listas longas de campos.
- `logging.Fatal` quando prepare falha — aplicação não deve iniciar com statement inválido
- Queries como constantes de pacote — nunca magic strings inline
- `criteria.From(table, alias).Select(fields)` para queries dinâmicas
- `sqln.NewStatement().Prepare(ctx, query)` para writes (INSERT/UPDATE/DELETE)
- Parâmetros posicionais `$1, $2, ...` — nunca concatenação de string
- `Close() error` **obrigatório** na interface e implementação — fecha todos os `*sql.Stmt` em sequência, retornando o primeiro erro encontrado
- **Aggregate com tx**: quando o repo tem mutação multi-tabela atômica, helpers dentro de `r.tx.Execute(...)` fazem rebind do stmt à conexão da tx via `ctx.Value(connection.SqlTxContextKey).(*sql.Tx).Stmt(r.stmXxx).ExecContext(...)`. Chamar `r.stmXxx.ExecContext(ctx, ...)` direto **não participa** da transação (pega outra conexão do pool) — **BLOCKER**. Padrão completo em `.claude/sdk/go/knowledge/repository-aggregate-pattern.md`.

## FindByID — Comportamento nil

```go
// nil, nil  → não encontrado (service deve retornar not found)
// nil, err  → erro de banco
// obj, nil  → encontrado com sucesso
person, err := r.FindByID(ctx, id)
```

## Consulta de Presença — `(*T, error)` para primitivos

Métodos que consultam um único valor primitivo (ex: `ExistsByEmailAndTenant`) devolvem **`(*T, error)`**. O retorno de `sqln.FindFromCriteria[T](...).Execute()` já tem a semântica certa: `nil` quando não encontrado, ponteiro quando encontrado. **Não converta** manualmente para `(T, error)`.

```go
// Interface
type UserRepository interface {
    ExistsByEmailAndTenant(ctx context.Context, email string, tenantID int64) (*bool, error)
    // ...
}

// Implementação — retorna direto o resultado de Execute()
func (r *userRepository) ExistsByEmailAndTenant(ctx context.Context, email string, tenantID int64) (*bool, error) {
    return sqln.FindFromCriteria[bool](ctx,
        criteria.From(`"user"`, "u").
            Select("u.id").
            Where(criteria.Eq("u.email", email)).
            Where(criteria.Eq("u.tenant_id", tenantID)),
    ).Execute()
}
```

**No service** o valor apontado é irrelevante — o que importa é `nil` vs não-nil:

```go
exists, err := s.repo.ExistsByEmailAndTenant(ctx, req.Email, req.TenantID)
if err != nil   { return ErrUserCreate.Wrap(err) }
if exists != nil { return ErrUserConflict.New() }
```

**Anti-padrão** (não usar):
```go
result, err := sqln.FindFromCriteria[bool](ctx, q).Execute()
if err != nil { return false, err }
return result != nil, nil   // duplica a checagem que o SDK já entrega
```

Escopo: aplica-se a `Exists*` e a qualquer consulta de **presença** de um único valor primitivo. Listas e contagens numéricas reais seguem seus próprios padrões. Detalhes em `.claude/knowledge/repository-primitive-return.md`.
