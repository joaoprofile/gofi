# Repository Update() — Padrão Simplificado

## Regra

Métodos `Update` de repository **não devem** checar `RowsAffected`. Retorne apenas erros estruturais do banco.

## Por quê

O service já chama `FindByID` antes de `Update` para obter dados imutáveis (ex: `tenant_id` para checar conflito de CPF). Se `FindByID` retorna `nil, nil` o service já retorna not-found antes de chamar `Update`. Checar `RowsAffected == 0` no repository seria redundante e criaria um segundo caminho de not-found inconsistente.

## Padrão

```go
func (r *personRepository) Update(ctx context.Context, id int64, req model.UpdatePersonRequest) error {
    _, err := r.stmUpdate.ExecContext(ctx,
        req.Name, req.CPF, req.Email, req.Phone, req.Active,
        req.City, req.State, req.Address, req.ZipCode, req.Complement,
        id,
    )
    return err
}
```

## Anti-padrão (não usar)

```go
// NÃO FAZER — complexidade desnecessária
res, err := r.stmUpdate.ExecContext(ctx, ...)
if err != nil { return err }
n, err := res.RowsAffected()
if err != nil { return err }
if n == 0 { return ErrNoRowsAffected }
return nil
```

## Consequência no Service

O service **não** precisa de `errors.Is(err, repository.ErrNoRowsAffected)` no handler de Update — o not-found é detectado pelo `FindByID` que precede o `Update`:

```go
func (s *personService) Update(ctx context.Context, id int64, req model.UpdatePersonRequest) errs.AppError {
    existing, err := s.repo.FindByID(ctx, id)
    if err != nil { return ErrPersonUpdate.Wrap(err) }
    if existing == nil { return ErrPersonNotFound.New() }  // ← not-found aqui, não no repo

    // ... CPF conflict check ...

    if err := s.repo.Update(ctx, id, req); err != nil {
        return ErrPersonUpdate.Wrap(err)  // apenas erros estruturais
    }
    return errs.AppError{}
}
```

## ErrNoRowsAffected

`ErrNoRowsAffected` pode ser mantido no pacote `repository` para outros casos de uso (ex: Delete com verificação explícita), mas **Update nunca o retorna**.
