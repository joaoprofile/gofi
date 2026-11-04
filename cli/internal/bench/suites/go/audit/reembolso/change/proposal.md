---
tipo: mudanca
formato: 3
contextos: [benchreembolso]
versao: "1.0"
status: proposto
depende_de: []
superficies: [backend]
keywords: [reembolso, taxa, prazo, centavos]
---
# Mudança — bench-auditar-reembolso

## Por quê

A loja devolve o valor pago de um pedido cancelado, menos uma taxa, dentro de um prazo.

## O que muda

- benchreembolso: REQ-01 novo (reembolso menos a taxa); REQ-02 novo (recusa de valor negativo); REQ-03 novo (prazo do reembolso)

## Impacto

Contexto novo, sem dependentes: o pacote `domain/benchreembolso/service` no backend, sem banco, sem rota.

## Fora desta mudança

- Expor numa rota HTTP — fica para outra mudança.
