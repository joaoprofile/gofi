---
tipo: mudanca
formato: 3
contextos: [benchconta]
versao: "1.0"
status: proposto
depende_de: []
superficies: [backend]
keywords: [transferencia, saldo, conta, transacao]
---
# Mudança — bench-auditar-transferencia

## Por quê

A loja move crédito entre contas de clientes e não pode perder nem criar dinheiro no caminho.

## O que muda

- benchconta: REQ-01 novo (transferência atômica); REQ-02 novo (recusa de valor inválido e de saldo insuficiente)

## Impacto

Contexto novo, sem dependentes: o repositório e o serviço do pacote `domain/benchconta`, sobre a tabela `benchconta_conta`.

## Fora desta mudança

- Registrar a rota no binário e subir o servidor — fica para outra mudança.
