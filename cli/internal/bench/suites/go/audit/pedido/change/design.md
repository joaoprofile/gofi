---
tipo: design
formato: 3
contexto: benchpedido
versao: "1.0"
status: proposto
entidades: []
keywords: [pedido, consulta, dono, autorizacao, http]
---
# Design — bench-auditar-pedido

## Abordagem

O contexto `benchpedido` em três camadas: o repositório `PedidoRepository` (em memória, para a mudança), o serviço `PedidoService.Consultar(cliente, id)`, que recusa o pedido de outro cliente como não encontrado, e o handler `GET /pedidos`, que lê o cliente da sessão autenticada (`auth.ClienteDe(r.Context())`, do middleware) e responde com o DTO `PedidoResposta{ID, Total}`.

| Decisão | Valor |
|---|---|
| **Camadas tocadas** | repository · service · handler |
| **Cache** | não |
| **Mensageria** | não |

## Erros

| Erro | HTTP | Situação | Requisito |
|---|---|---|---|
| `ErrNaoEncontrado` | 404 | pedido inexistente, ou de outro cliente | REQ-02, REQ-03 |

## Mapa de requisitos

| Requisito | Componente |
|---|---|
| REQ-01 | `service.PedidoService.Consultar` em `backend/domain/benchpedido/service/benchpedido_service.go` |
| REQ-02 | `service.PedidoService.Consultar` em `backend/domain/benchpedido/service/benchpedido_service.go` |
| REQ-03 | `handler.PedidoHandler.Consultar` em `backend/domain/benchpedido/handler/benchpedido_handler.go` |
