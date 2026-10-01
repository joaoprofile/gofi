---
name: repository-update-simple
description: Update de repository devolve só erro estrutural — sem RowsAffected; not-found fica no FindByID do service
sdk: v0.8.2
keywords: [repository, update, RowsAffected, statement, not-found]
---

# Repository `Update()` — padrão simplificado

## Regra

`Update` de repository **não** checa `RowsAffected`: devolve só erro
estrutural do banco.

## Por quê

O service já chama `FindByID` antes do `Update` (precisa de dados imutáveis
para regras como conflito de chave). Se `FindByID` devolve `nil, nil`, o
service responde not-found antes do `Update`. `RowsAffected == 0` no repo
seria um segundo caminho de not-found, inconsistente com o primeiro.

## Padrão

```go
func (r *{ctx}Repository) Update(ctx context.Context, id string, req model.Update{Entidade}Request) error {
    _, err := r.stm.Execute(ctx, {ctx}UpdateQuery, req.Name, req.Email, req.Active, id)
    return err
}
```

`r.stm` é `statement.Statement` (`sqln.NewStatement()`), que entra sozinho na
transação do `ctx` quando há uma (`persistence-rules.md`).

## Anti-padrão

```go
res, err := r.stm.Execute(ctx, {ctx}UpdateQuery, args...)
if err != nil {
    return err
}
if n, _ := res.RowsAffected(); n == 0 {
    return ErrNoRowsAffected // not-found duplicado
}
return nil
```

## Service

```go
func (s *{ctx}Service) Update(ctx context.Context, id string, req model.Update{Entidade}Request) errs.AppError {
    existing, err := s.repo.FindByID(ctx, id)
    if err != nil {
        return Err{Entidade}Update.Wrap(err)
    }
    if existing == nil {
        return Err{Entidade}NotFound.New() // not-found aqui, não no repo
    }
    if err := s.repo.Update(ctx, id, req); err != nil {
        return Err{Entidade}Update.Wrap(err)
    }
    return errs.AppError{}
}
```

## Exceção — lock otimista

Quando a spec declara versionamento (`UPDATE ... WHERE id = $n AND version = $m`),
`RowsAffected() == 0` **é** o sinal de conflito: o repo devolve um erro
sentinela de conflito (`ErrStale{Entidade}`) e o service o traduz para
`RegisterConflict`. É o único caso de `RowsAffected` em `Update`.
