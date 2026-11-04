---
tipo: mudanca
formato: 3
contextos: [benchestoque]
versao: "1.0"
status: proposto
depende_de: []
superficies: [backend]
keywords: [estoque, reserva, quantidade]
---
# Mudança — bench-lacuna-contrato

## Por quê

A loja reserva unidades do estoque ao montar um pedido e precisa saber o saldo depois da reserva.

## O que muda

- benchestoque: REQ-01 novo (saldo após a reserva); REQ-02 novo (recusa de entrada inválida)

## Impacto

Contexto novo, sem dependentes: o pacote `domain/benchestoque/service` no backend, sem banco, sem rota.

## Fora desta mudança

- Expor numa rota HTTP — fica para outra mudança.
