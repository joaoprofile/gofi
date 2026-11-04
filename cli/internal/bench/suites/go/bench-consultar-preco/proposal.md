---
tipo: mudanca
formato: 3
contextos: [benchpreco]
versao: "1.0"
status: proposto
depende_de: []
superficies: [backend]
keywords: [preço, consulta, http, rota]
---
# Mudança — bench-consultar-preco

## Por quê

O aplicativo precisa consultar por HTTP o preço final de um produto com o desconto da sua categoria.

## O que muda

- benchpreco: REQ-01 novo (preço com desconto da categoria); REQ-02 novo (consulta HTTP do preço)

## Impacto

Contexto novo, sem dependentes: o serviço e o handler do pacote `domain/benchpreco`, sem banco.

## Fora desta mudança

- Registrar a rota no binário e subir o servidor — fica para a mudança do bootstrap HTTP.
