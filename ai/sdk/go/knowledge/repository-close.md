---
name: repository-close
description: Ciclo de vida no repository — sem Close de conexão nem prepared statements em campo; quem fecha o pool é o componente database
sdk: v0.8.2
keywords: [repository, close, lifecycle, prepared-statement, sql.Stmt, shutdown]
---

# Repository — ciclo de vida (sem `Close()` de recurso)

## Regra

O repository **não é dono de recurso de banco**: não guarda `*sql.Stmt` nem
`*sql.DB` em campo e, portanto, **não expõe `Close()`** na interface. O pool é
aberto e fechado pelo componente `database` do gofi (`Shutdown`) —
`database-connection.md`.

## Por quê

- Escrita usa `statement.Statement.Execute` (sem prepared statement nomeado;
  entra na tx do `ctx`). Não há statement a liberar.
- O construtor não toca o banco (a conexão global só existe depois do
  `Build`), então também não há nada aberto na construção.
- `Close()` vazio na interface é contrato morto: todo mock precisa
  implementá-lo e ninguém o chama.

## Única exceção — `*sql.Stmt` local num bulk

Prepared statement só existe **dentro** de uma transação, num laço de bulk, e
morre ali:

```go
return r.tx.Execute(ctx, func(ctx context.Context) error {
    stmt, err := r.stm.Prepare(ctx, {ctx}InsertItemQuery) // preparado na tx do ctx
    if err != nil {
        return err
    }
    defer stmt.Close()
    for _, it := range items {
        if _, err := stmt.ExecContext(ctx, itemArgs(it)...); err != nil {
            return err
        }
    }
    return nil
})
```

## Legado

Repository antigo com `stmXxx *sql.Stmt` preparados no construtor + `Close()`:
ao tocar no arquivo, migre para `r.stm.Execute` e remova `Close()` da
interface, do mock e do wiring (`defer repo.Close()` no `main`).

## Checklist

- [ ] Nenhum campo `*sql.Stmt`/`*sql.DB` no struct do repository
- [ ] Sem `Close()` na interface do repository
- [ ] `Prepare` só dentro de `r.tx.Execute`, com `defer stmt.Close()`
