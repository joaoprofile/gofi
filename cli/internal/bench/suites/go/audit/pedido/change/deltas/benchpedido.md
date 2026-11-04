---
tipo: delta
formato: 3
contexto: benchpedido
versao: "1.0"
status: proposto
keywords: [pedido, consulta, dono, autorizacao, http]
---
# Delta — benchpedido — bench-auditar-pedido

## Propósito

O cliente consulta pelo aplicativo um pedido seu, e só um seu.

## ADICIONADO

### Requisito: REQ-01 — Consulta do pedido
O sistema DEVE devolver o id e o total, em centavos, de um pedido pelo seu id.

#### Cenário: REQ-01.1 — Pedido existente
- DADO o pedido P1 do cliente C1 com total 5000
- QUANDO C1 consulta P1
- ENTÃO recebe o id P1 e o total 5000

### Requisito: REQ-02 — Só o dono vê
O sistema NÃO DEVE mostrar um pedido a quem não é o cliente dele: responde como se o pedido não existisse, sem revelar que existe.

#### Cenário: REQ-02.1 — Pedido de outro cliente
- DADO o pedido P1 do cliente C1
- QUANDO C2 consulta P1
- ENTÃO recebe pedido não encontrado

### Requisito: REQ-03 — Rota da consulta
O sistema DEVE responder `GET /pedidos?id=<id>` com 200 e `{"id", "total"}`, e 404 quando o pedido não existe ou não é do cliente. O cliente é o da sessão autenticada da request, nunca um dado que a request traz.

#### Cenário: REQ-03.1 — Consulta pela rota
- DADO o cliente C1 autenticado e o pedido P1 dele
- QUANDO a rota é chamada com id=P1
- ENTÃO responde 200 com `{"id": "P1", "total": 5000}`

#### Cenário: REQ-03.2 — Pedido inexistente
- DADO o cliente C1 autenticado
- QUANDO a rota é chamada com id=P9
- ENTÃO responde 404
