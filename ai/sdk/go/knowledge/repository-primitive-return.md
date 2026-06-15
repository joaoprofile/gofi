# Repository — Retorno de Tipos Primitivos com `FindFromCriteria[T]`

## Regra

Quando um método de repository **consulta um único valor primitivo** (existência, contagem de ativo, flag), devolva **`(*T, error)`** — o retorno nativo de `sqln.FindFromCriteria[T](...).Execute()`. Nunca converta manualmente para `(T, error)` escrevendo `result != nil` dentro do repository.

## Por quê

`sqln.FindFromCriteria[T](ctx, q).Execute()` já tem semântica canônica:

- `(nil, nil)` → nenhuma linha encontrada
- `(*T, nil)` → encontrada (o ponteiro deixa de ser `nil`)
- `(nil, err)` → erro estrutural de banco

Converter esse retorno dentro do repository (`return result != nil, nil`) duplica a checagem, esconde a semântica do SDK (nil vs presente) e obriga o service a confiar num `bool` derivado em vez do ponteiro que o SDK já oferece. Repassar `*T` mantém o contrato honesto e alinhado com `FindByID` (também `(*Entity, error)`).

## Padrão

```go
// Interface
type UserRepository interface {
    ExistsByEmailAndTenant(ctx context.Context, email string, tenantID int64) (*bool, error)
    // ...
}

// Implementação — sem conversão manual
func (r *userRepository) ExistsByEmailAndTenant(ctx context.Context, email string, tenantID int64) (*bool, error) {
    return sqln.FindFromCriteria[bool](ctx,
        criteria.From(`"user"`, "u").
            Select("u.id").
            Where(criteria.Eq("u.email", email)).
            Where(criteria.Eq("u.tenant_id", tenantID)),
    ).Execute()
}
```

## Consumo no service — checar `!= nil`

O service interpreta o ponteiro como marcador de presença. O **valor apontado é irrelevante** — o que importa é `nil` vs não-nil.

```go
exists, err := s.repo.ExistsByEmailAndTenant(ctx, req.Email, req.TenantID)
if err != nil {
    return ErrUserCreate.Wrap(err)
}
if exists != nil {
    return ErrUserConflict.New()
}
```

## Anti-padrão (não usar)

```go
// NÃO FAZER — boilerplate redundante, obscurece semântica do SDK
func (r *userRepository) ExistsByEmailAndTenant(ctx context.Context, email string, tenantID int64) (bool, error) {
    result, err := sqln.FindFromCriteria[bool](ctx, q).Execute()
    if err != nil {
        return false, err
    }
    return result != nil, nil
}
```

## Escopo da regra

Aplica-se a qualquer consulta que o repository retorne **um único valor primitivo** via `FindFromCriteria[T]`:

- `Exists*` → `(*bool, error)`
- `Count*` via `SELECT 1`/`SELECT id` usado como presença → `(*bool, error)`
- Flags e scalars com semântica "presente ou não" → `(*T, error)` com `T` primitivo

**Não se aplica a:**

- `FindByID` e leituras que já retornam entidade/DTO — já usam `(*Entity, error)` naturalmente
- Listas — sempre `([]T, error)` ou `*sqln.Page[T]`
- Contadores reais (número de linhas) — nesse caso use uma query específica de agregação; o padrão deste documento é para **presença**, não para contagem numérica

## Testes

O mock do repository no `service_test.go` espelha a assinatura do contrato — retorne ponteiro:

```go
type mockUserRepository struct {
    existsByEmailAndTenantFn func(ctx context.Context, email string, tenantID int64) (*bool, error)
}

func (m *mockUserRepository) ExistsByEmailAndTenant(ctx context.Context, email string, tenantID int64) (*bool, error) {
    if m.existsByEmailAndTenantFn != nil {
        return m.existsByEmailAndTenantFn(ctx, email, tenantID)
    }
    return nil, nil
}

// Helper usado nos testes para montar *bool
func boolPtr(b bool) *bool { return &b }

// No teste de conflito — basta devolver qualquer ponteiro não-nil
existsByEmailAndTenantFn: func(_ context.Context, _ string, _ int64) (*bool, error) {
    return boolPtr(true), nil
},
```
