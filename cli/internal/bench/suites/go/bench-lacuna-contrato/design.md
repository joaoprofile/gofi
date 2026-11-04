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
| erro de validação registrado em `errors.go` do contexto | entrada inválida | REQ-02 |

## Mapa de requisitos

| Requisito | Componente |
|---|---|
| REQ-01 | `service.Saldo` em `backend/domain/benchestoque/service/benchestoque_service.go` |
| REQ-02 | `service.Saldo` em `backend/domain/benchestoque/service/benchestoque_service.go` |
