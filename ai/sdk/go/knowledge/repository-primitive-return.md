---
name: repository-primitive-return
description: Consulta de presença/valor primitivo devolve (*T, error) direto do UniqueResult do sqln
sdk: v0.8.2
keywords: [repository, primitive, exists, UniqueResult, FindFromCriteria, presence]
---

# Repository — retorno de valor primitivo com `FindFromCriteria[T]`

## Regra

Método que consulta **um único valor primitivo** (existência, flag, escalar
"presente ou não") devolve **`(*T, error)`** — o retorno nativo de
`sqln.FindFromCriteria[T](...).UniqueResult()`. Nunca converta para
`(T, error)` com `result != nil` dentro do repository.

## Por quê

`UniqueResult()` já tem a semântica canônica:

- `(nil, nil)` → nenhuma linha
- `(*T, nil)` → encontrada
- `(nil, err)` → erro de banco

Converter duplica a checagem e esconde a semântica; repassar `*T` mantém o
contrato alinhado com `FindByID` (`(*Entity, error)`).

## Padrão

```go
// Interface
Exists{Entidade}ByKey(ctx context.Context, key string, tenantID string) (*bool, error)

// Implementação
func (r *{ctx}Repository) Exists{Entidade}ByKey(ctx context.Context, key string, tenantID string) (*bool, error) {
    return sqln.FindFromCriteria[bool](ctx,
        criteria.From("{tabela}", "e").
            Select("TRUE").
            Where(criteria.Eq("e.key", key), criteria.Eq("e.tenant_id", tenantID)).
            Limit(1),
    ).UniqueResult()
}
```

- `T` primitivo escaneia a **primeira coluna** direto: selecione **uma**
  coluna do tipo de `T` (`TRUE` para `bool`). `Select("e.id")` em
  `FindFromCriteria[bool]` falha no scan (UUID/inteiro não vira `bool`).
- `Limit(1)`: presença não precisa ler mais de uma linha.

## Consumo no service

```go
exists, err := s.repo.Exists{Entidade}ByKey(ctx, req.Key, req.TenantID)
if err != nil {
    return Err{Entidade}Create.Wrap(err)
}
if exists != nil {
    return Err{Entidade}Conflict.New()
}
```

O valor apontado é irrelevante — importa `nil` vs não-nil. (Para conflito de
chave única na escrita, prefira traduzir o `23505` no `Save` —
`persistence-rules.md`; `Exists*` é para regra de negócio que precisa saber
antes.)

## Anti-padrão

```go
result, err := sqln.FindFromCriteria[bool](ctx, q).UniqueResult()
if err != nil {
    return false, err
}
return result != nil, nil // duplica o que o SDK já entrega
```

## Escopo

- `Exists*` → `(*bool, error)`; flags/escalares de presença → `(*T, error)`.
- **Não se aplica** a `FindByID` e DTOs (já `(*Entity, error)`), listas
  (`[]T` ou `*sqln.Page[T]`) nem contagens numéricas reais (query de
  agregação própria).

## Mock no teste do service

```go
type mock{Ctx}Repository struct {
    existsFn func(ctx context.Context, key, tenantID string) (*bool, error)
}

func (m *mock{Ctx}Repository) Exists{Entidade}ByKey(ctx context.Context, key, tenantID string) (*bool, error) {
    if m.existsFn != nil {
        return m.existsFn(ctx, key, tenantID)
    }
    return nil, nil
}

func boolPtr(b bool) *bool { return &b }
```
