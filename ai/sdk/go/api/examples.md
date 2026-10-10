# Exemplos executáveis do SDK — SDK gofi/v0.2.4 (310ba1a)

> Gerado do SDK — não edite. Programas completos, compilados e testados na CI do SDK:
> leia o código no checkout, na versão que o projeto fixou.

## netx/grpcx — gRPC example

`https://github.com/joaoprofile/gofi-sdk-go/tree/gofi/v0.2.4/examples/grpc/`

A `people.v1.PeopleService` that moves a `Person` entity and its photo through every gRPC call shape, with two servers to choose from and one client that works with both:

- `https://github.com/joaoprofile/gofi-sdk-go/tree/gofi/v0.2.4/examples/grpc/generate.go`

## iam — login example

`https://github.com/joaoprofile/gofi-sdk-go/tree/gofi/v0.2.4/examples/iam/login/`

Login with the `iam` package in its three usual shapes, with **no database**: users, sessions and tokens live in memory.

- `https://github.com/joaoprofile/gofi-sdk-go/tree/gofi/v0.2.4/examples/iam/login/auth.go`
- `https://github.com/joaoprofile/gofi-sdk-go/tree/gofi/v0.2.4/examples/iam/login/handlers.go`
- `https://github.com/joaoprofile/gofi-sdk-go/tree/gofi/v0.2.4/examples/iam/login/main.go`
- `https://github.com/joaoprofile/gofi-sdk-go/tree/gofi/v0.2.4/examples/iam/login/users.go`

## Kafka consumer

`https://github.com/joaoprofile/gofi-sdk-go/tree/gofi/v0.2.4/examples/msq/kafka/consumer/`

Processes the `OrderCreated` events from `orders`. Invalid orders are retried and then dead-lettered to `orders-dlq`, which a second handler logs. Part of the [Kafka example](../README.md).

- `https://github.com/joaoprofile/gofi-sdk-go/tree/gofi/v0.2.4/examples/msq/kafka/consumer/main.go`

## Kafka producer

`https://github.com/joaoprofile/gofi-sdk-go/tree/gofi/v0.2.4/examples/msq/kafka/producer/`

Publishes an `OrderCreated` event to `orders` every 2 seconds until `Ctrl+C`. Part of the [Kafka example](../README.md); run the consumer too to see the messages processed.

- `https://github.com/joaoprofile/gofi-sdk-go/tree/gofi/v0.2.4/examples/msq/kafka/producer/main.go`

## RabbitMQ consumer

`https://github.com/joaoprofile/gofi-sdk-go/tree/gofi/v0.2.4/examples/msq/rabbitmq/consumer/`

Processes the `OrderCreated` events from `orders`. Invalid orders are retried and then dead-lettered to `orders-dlq`, which a second handler logs. Part of the [RabbitMQ example](../README.md).

- `https://github.com/joaoprofile/gofi-sdk-go/tree/gofi/v0.2.4/examples/msq/rabbitmq/consumer/main.go`

## RabbitMQ producer

`https://github.com/joaoprofile/gofi-sdk-go/tree/gofi/v0.2.4/examples/msq/rabbitmq/producer/`

Publishes an `OrderCreated` event to `orders` every 2 seconds until `Ctrl+C`. Part of the [RabbitMQ example](../README.md); run the consumer too to see the messages processed.

- `https://github.com/joaoprofile/gofi-sdk-go/tree/gofi/v0.2.4/examples/msq/rabbitmq/producer/main.go`

## Amazon SQS consumer

`https://github.com/joaoprofile/gofi-sdk-go/tree/gofi/v0.2.4/examples/msq/sqs/consumer/`

Processes the `OrderCreated` events from `orders`. Invalid orders are retried and then dead-lettered to `orders-dlq`, which a second handler logs. Part of the [Amazon SQS example](../README.md).

- `https://github.com/joaoprofile/gofi-sdk-go/tree/gofi/v0.2.4/examples/msq/sqs/consumer/main.go`

## Amazon SQS producer

`https://github.com/joaoprofile/gofi-sdk-go/tree/gofi/v0.2.4/examples/msq/sqs/producer/`

Publishes an `OrderCreated` event to `orders` every 2 seconds until `Ctrl+C`. Part of the [Amazon SQS example](../README.md); run the consumer too to see the messages processed.

- `https://github.com/joaoprofile/gofi-sdk-go/tree/gofi/v0.2.4/examples/msq/sqs/producer/main.go`

## netx/httpx — API example

`https://github.com/joaoprofile/gofi-sdk-go/tree/gofi/v0.2.4/examples/netx/api/`

A minimal HTTP API built with `netx/httpx`: several handlers, public and private routes, a global middleware and an auth middleware, all wired in `main.go` through the gofi `httpserver` component.

- `https://github.com/joaoprofile/gofi-sdk-go/tree/gofi/v0.2.4/examples/netx/api/main.go`

## obs — observability example

`https://github.com/joaoprofile/gofi-sdk-go/tree/gofi/v0.2.4/examples/obs/`

An instrumented service that exports **traces, metrics and logs** over OTLP to a local Grafana stack. It covers what a real service has to instrument:

- `https://github.com/joaoprofile/gofi-sdk-go/tree/gofi/v0.2.4/examples/obs/loadgen.go`
- `https://github.com/joaoprofile/gofi-sdk-go/tree/gofi/v0.2.4/examples/obs/main.go`
- `https://github.com/joaoprofile/gofi-sdk-go/tree/gofi/v0.2.4/examples/obs/runner.go`

## sqln — dynamic filter API

`https://github.com/joaoprofile/gofi-sdk-go/tree/gofi/v0.2.4/examples/sqln/filter-api/`

An HTTP API where the **client chooses the filters**: the JSON body is turned into SQL by `sqln`, through an allowlist (`sqln.FilterMapping`). Fields, operators or sort columns outside the allowlist are rejected with `400`.

- `https://github.com/joaoprofile/gofi-sdk-go/tree/gofi/v0.2.4/examples/sqln/filter-api/main.go`

## sqln — search job

`https://github.com/joaoprofile/gofi-sdk-go/tree/gofi/v0.2.4/examples/sqln/search/`

A **job** (no HTTP server): it opens PostgreSQL through `gofi`, applies the migrations, runs a few `sqln` criteria queries, prints the results and exits.

- `https://github.com/joaoprofile/gofi-sdk-go/tree/gofi/v0.2.4/examples/sqln/search/main.go`

