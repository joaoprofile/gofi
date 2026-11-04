---
tipo: design
formato: 3
contexto: benchestoque
versao: "1.0"
status: proposto
entidades: []
keywords: [estoque, reserva, quantidade]
---
# Design — bench-lacuna-contrato

## Abordagem

Um contexto novo, o pacote `domain/benchestoque/service`, com a função `Saldo` em `benchestoque_service.go`; sem banco, sem rota.

| Decisão | Valor |
|---|---|
| **Cache** | não |
| **Mensageria** | não |

## Erros

| Erro | Situação | Requisito |
|---|---|---|
| `ErrBenchestoqueInsuficiente`, `errs.RegisterConflict` (409), em `errors.go` | reserva maior que o estoque | REQ-01 |
| `ErrBenchestoqueEntradaInvalida`, `errs.RegisterValidation` (400), em `errors.go` | estoque ou reserva negativos | REQ-02 |

## Mapa de requisitos

| Requisito | Componente |
|---|---|
| REQ-01 | `service.Saldo` em `backend/domain/benchestoque/service/benchestoque_service.go` |
| REQ-02 | `service.Saldo` em `backend/domain/benchestoque/service/benchestoque_service.go` |
