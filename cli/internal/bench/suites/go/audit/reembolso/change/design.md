---
tipo: design
formato: 3
contexto: benchreembolso
versao: "1.0"
status: proposto
entidades: []
keywords: [reembolso, taxa, prazo, centavos]
---
# Design — bench-auditar-reembolso

## Abordagem

Um contexto novo, o pacote `domain/benchreembolso/service`, com a função `Reembolso` em `benchreembolso_service.go`; sem banco, sem rota.

| Decisão | Valor |
|---|---|
| **Cache** | não |
| **Mensageria** | não |

## Erros

| Erro | Situação | Requisito |
|---|---|---|
| `ErrValorInvalido` | valor pago negativo | REQ-02 |
| `ErrPrazoExpirado` | pedido de mais de 30 dias | REQ-03 |

## Mapa de requisitos

| Requisito | Componente |
|---|---|
| REQ-01 | `service.Reembolso` em `backend/domain/benchreembolso/service/benchreembolso_service.go` |
| REQ-02 | `service.Reembolso` em `backend/domain/benchreembolso/service/benchreembolso_service.go` |
| REQ-03 | `service.Reembolso` em `backend/domain/benchreembolso/service/benchreembolso_service.go` |
