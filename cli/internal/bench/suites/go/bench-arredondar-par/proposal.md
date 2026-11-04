---
tipo: mudanca
formato: 3
contextos: [benchrateio]
versao: "1.0"
status: proposto
depende_de: []
superficies: [backend]
keywords: [arredondamento, par, milesimos, centavos]
---
# Mudança — bench-arredondar-par

## Por quê

A loja recebe valores de parceiros em milésimos de real e precisa convertê-los em centavos sem viés de arredondamento.

## O que muda

- benchrateio: REQ-01 novo (conversão para centavos, meio para o par); REQ-02 novo (recusa de valor negativo)

## Impacto

Contexto novo, sem dependentes: o pacote `domain/benchrateio/service` no backend, sem banco, sem rota.

## Fora desta mudança

- Expor numa rota HTTP — fica para outra mudança.
