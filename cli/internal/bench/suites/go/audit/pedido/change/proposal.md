---
tipo: mudanca
formato: 3
contextos: [benchpedido]
versao: "1.0"
status: proposto
depende_de: []
superficies: [backend]
keywords: [pedido, consulta, dono, autorizacao, http]
---
# Mudança — bench-auditar-pedido

## Por quê

O cliente consulta pelo aplicativo um pedido seu, e só um seu.

## O que muda

- benchpedido: REQ-01 novo (consulta do pedido); REQ-02 novo (só o dono vê); REQ-03 novo (rota HTTP da consulta)

## Impacto

Contexto novo, sem dependentes: o repositório, o serviço e o handler do pacote `domain/benchpedido`, sem banco.

## Fora desta mudança

- Registrar a rota no binário e subir o servidor — fica para outra mudança.
