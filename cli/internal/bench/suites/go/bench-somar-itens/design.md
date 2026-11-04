---
tipo: design
formato: 3
contexto: benchcarrinho
versao: "1.0"
status: proposto
entidades: []
keywords: [carrinho, soma, itens, centavos]
---
# Design — bench-somar-itens

## Abordagem

Um contexto novo, o pacote `domain/benchcarrinho/service`, com a função `Subtotal` em `benchcarrinho_service.go`; sem banco, sem rota.

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
| REQ-01 | `service.Subtotal` em `backend/domain/benchcarrinho/service/benchcarrinho_service.go` |
| REQ-02 | `service.Subtotal` em `backend/domain/benchcarrinho/service/benchcarrinho_service.go` |
