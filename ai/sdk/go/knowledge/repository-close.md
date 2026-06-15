# Repository Close() — Ciclo de vida dos Prepared Statements

## Regra

Todo repository que prepara `*sql.Stmt` no construtor **deve** expor `Close() error` na interface e implementação.

## Por quê

`*sql.Stmt` mantém uma conexão aberta com o banco de dados. Sem `Close()`, os statements ficam abertos indefinidamente — causando leaks de conexão em cenários de shutdown ou hot-reload.

## Padrão

**Interface (obrigatório):**
```go
type PersonRepository interface {
    Save(ctx context.Context, p model.Person) error
    Update(ctx context.Context, id int64, req model.UpdatePersonRequest) error
    // ... demais métodos
    Close() error  // sempre o último método da interface
}
```

**Implementação — fechar em sequência, retornar primeiro erro:**
```go
func (r *personRepository) Close() error {
    if err := r.stmCreate.Close(); err != nil {
        return err
    }
    if err := r.stmUpdate.Close(); err != nil {
        return err
    }
    return r.stmDelete.Close()
    // um bloco por statement preparado no construtor
}
```

**Mock de teste — sempre retorna nil:**
```go
func (m *mockPersonRepository) Close() error { return nil }
```

## Onde chamar Close()

No `main.go`, usar `defer` ou chamar no shutdown do serviço:
```go
personRepo := repository.NewPersonRepository(context.Background())
defer personRepo.Close()
```

## Checklist

- [ ] `Close() error` na interface do repository
- [ ] `Close()` na implementação — um `.Close()` por `*sql.Stmt`
- [ ] Mock de teste implementa `Close() error { return nil }`
