---
tipo: mudanca
formato: 3
contextos: [benchcarrinho]
versao: "1.0"
status: proposto
depende_de: []
superficies: [backend]
keywords: [carrinho, soma, itens, centavos]
---
# Mudança — bench-somar-itens

## Por quê

A loja precisa do subtotal do carrinho a partir do preço e da quantidade de cada item.

## O que muda

- benchcarrinho: REQ-01 novo (subtotal do carrinho); REQ-02 novo (recusa de item inválido)

## Impacto

Contexto novo, sem dependentes: o pacote `domain/benchcarrinho/service` no backend, sem banco, sem rota.

## Fora desta mudança

- Expor numa rota HTTP — fica para outra mudança.
