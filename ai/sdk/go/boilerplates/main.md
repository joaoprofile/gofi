---
name: main
description: Esqueleto do main.go de um serviço gofi — config, componentes, Build, wiring depois do Build, ListenAndServe; variantes job, sem auth, mensageria e worker
sdk: v0.8.2
---

# Boilerplate — main.go

Regras e ordem: `.claude/sdk/go/knowledge/service-bootstrap.md`. Orquestrador:
`gofi-orchestrator.md`. Config: `configuration.md`. Middleware de auth:
`http-auth-middleware.md`.

## Serviço HTTP com IAM (`main.go`)

```go
package main

import (
	"context"
	"errors"
	"log"

	"github.com/joaoprofile/gofi-sdk-go/base/environment"
	"github.com/joaoprofile/gofi-sdk-go/gofi"
	"github.com/joaoprofile/gofi-sdk-go/gofi/component/database"
	"github.com/joaoprofile/gofi-sdk-go/gofi/component/httpserver"
	"github.com/joaoprofile/gofi-sdk-go/gofi/component/observability"
	"github.com/joaoprofile/gofi-sdk-go/netx"
	_ "github.com/joaoprofile/gofi-sdk-go/sqln/driver/postgres" // DATABASE_DRIVER=postgres
)

const serviceName = "{servico}"

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	ctx := context.Background()

	cfg, err := LoadConfig(ctx, environment.Instance())
	if err != nil {
		return err
	}

	identity := newIdentity(cfg) // iam.go
	server := httpserver.New(":8080", &netx.WSConfig{
		AllowedOrigins: cfg.AllowedOrigins,
		Health:         &netx.HealthConfig{},
	})

	svc, err := gofi.New(serviceName).
		With(
			observability.New(),
			database.New(),
			identity,
			server,
		).
		Build()
	if err != nil {
		return err
	}

	// Wiring after Build: identity.Service() and the global database exist from here on.
	if err := wire(identity, server); err != nil {
		return errors.Join(err, svc.Shutdown(ctx))
	}

	return svc.ListenAndServe()
}
```

## `wire.go`

```go
package main

import (
	entityhandler "<module>/domain/{contexto}/handler"
	entityrepo "<module>/domain/{contexto}/repository"
	entitysvc "<module>/domain/{contexto}/service"
	authhandler "<module>/domain/{contexto-auth}/handler"
	"github.com/joaoprofile/gofi-sdk-go/gofi/component/httpserver"
	"github.com/joaoprofile/gofi-sdk-go/gofi/component/iam"
)

func wire(identity *iam.Component, server *httpserver.Component) error {
	iamSvc := identity.Service()

	entityHandler := entityhandler.NewEntityHandler(
		entitysvc.NewEntityService(entityrepo.NewEntityRepository()),
		iamSvc.RBAC(),
	)

	server.UseAuth(authhandler.Middleware(iamSvc, authhandler.NewVault())). // before Handlers
		Handlers(
			entityHandler,
			// one handler per context
		)
	return nil
}
```

## Padrão de wiring

```
LoadConfig → componentes no With → Build → repository → service → handler → UseAuth → Handlers → ListenAndServe
```

- Repository, service e handler montados em `wire.go`, **depois** do `Build`
  (handles de componente só existem a partir dele).
- `UseAuth` **antes** de `Handlers`.
- `wire` devolve `error`; o `run` chama `svc.Shutdown` antes de sair.
- Nada de `log.Fatal` fora de `main()`.
- `observability.New()` sem `OTEL_EXPORTER_OTLP_ENDPOINT` apenas se desliga.

## Variante — sem autenticação

Sem `identity`; handlers só com `netx.PublicRoutes` e `server.Handlers(...)`
direto no `wire`.

## Variante — job (sem HTTP)

```go
func run() error {
	ctx := context.Background()
	svc, err := gofi.New("{job}").With(database.New()).Build()
	if err != nil {
		return err
	}
	jobErr := buildJob().Run(ctx)
	return errors.Join(jobErr, svc.Shutdown(ctx)) // closes the database and flushes logs
}
```

## Variante — mensageria (producer + consumer)

```go
import _ "github.com/joaoprofile/gofi-sdk-go/msq/provider/kafka" // MESSAGING_PROVIDER=kafka

mq := messaging.New()

svc, err := gofi.New(serviceName).
	With(database.New(), mq, newEntityConsumer(mq, cfg), server). // consumer-bootstrap.md
	Build()
// ...
publisher := buildEntityPublisher(mq.Broker()) // wire.go; mq.Broker() is nil before Build
```

Producer (criação, concorrência, `Close`): `messaging-msq.md` § Producer.

## Variante — worker agendado

```go
With(database.New(), cache.New(), newReportCron(cfg), server) // worker-bootstrap.md
```
