---
tipo: design
formato: 3
contexto: benchrateio
versao: "1.0"
status: proposto
entidades: []
keywords: [arredondamento, par, milesimos, centavos]
---
# Design — bench-arredondar-par

## Abordagem

Um contexto novo, o pacote `domain/benchrateio/service`, com a função `Centavos` em `benchrateio_service.go`; sem banco, sem rota.

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
| REQ-01 | `service.Centavos` em `backend/domain/benchrateio/service/benchrateio_service.go` |
| REQ-02 | `service.Centavos` em `backend/domain/benchrateio/service/benchrateio_service.go` |
