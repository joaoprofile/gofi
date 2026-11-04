---
tipo: design
formato: 3
contexto: benchpreco
versao: "1.0"
status: proposto
entidades: []
keywords: [preço, consulta, http, rota]
---
# Design — bench-consultar-preco

## Abordagem

Um contexto novo em duas camadas: o serviço `PrecoFinal` em `domain/benchpreco/service` e o handler `GET /precos` em `domain/benchpreco/handler`, sem banco. Registrar a rota no binário fica fora.

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
| REQ-01 | `service.PrecoFinal` em `backend/domain/benchpreco/service/benchpreco_service.go` |
| REQ-02 | o handler `GET /precos` em `backend/domain/benchpreco/handler/benchpreco_handler.go` |
