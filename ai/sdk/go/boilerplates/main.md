# Boilerplate — main.go

```go
package main

import (
	"context"

	"github.com/joaoprofile/examples/api/src/person/handler"
	"github.com/joaoprofile/examples/api/src/person/repository"
	"github.com/joaoprofile/examples/api/src/person/service"
	"github.com/joaoprofile/gofi"
	"github.com/joaoprofile/gofi/netx"
)

const (
	APP_NAME = "my-service"
	APP_PORT = ":8080"
)

var AllowedOrigins = []string{
	"http://localhost:5173",
}

func main() {
	api := gofi.New(APP_NAME).
		NewHttpServer(APP_PORT,
			&netx.WSConfig{AllowedOrigins: AllowedOrigins}).
		AddDatabase().
		Build()

	// Wiring manual: repository → service → handler
	personRepo := repository.NewPersonRepository(context.Background())
	personSvc := service.NewPersonService(personRepo)
	personHandler := handler.NewPersonHandler(personSvc)

	api.HttpServer().AddHandlers(
		personHandler,
		// adicionar outros handlers aqui
	)

	api.ListenAndServe()
}
```

## Padrão de Wiring

```
repository.New(ctx) → service.New(repo) → handler.New(svc)
```

- Repository precisa de `context.Background()` para preparar statements
- Todos os contextos são registrados em `AddHandlers`
- `gofi.New().NewHttpServer().AddDatabase().Build()` é o padrão mínimo

## Com IAM

```go
import "github.com/joaoprofile/gofi/iam"

iamSvc, err := iam.New(iam.Config{...})
if err != nil {
    log.Fatal(err)
}

// Injetar nas rotas que precisam de autenticação
```

## Com Messaging

```go
import "github.com/joaoprofile/gofi/msq"

broker, err := msq.Config{BrokerType: msq.BrokerKafka}.Factory.Build(ctx)
mgr := msq.NewConsumerManager(broker)
mgr.Register(...)
go mgr.Start(ctx)
```
