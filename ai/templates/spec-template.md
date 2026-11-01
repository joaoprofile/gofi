---
tipo: spec
formato: 3                        # versão do formato do corpus — não invente outro valor
contexto: {contexto}
versao: "1.0"
status: proposto                  # aprovado só por `gofi spec approve` (baseline) ou pelo arquivamento de uma mudança
depende_de: []                    # contextos cujo código este consome — ordena o loop (gofi spec order)
entidades: [{tabelas do schema que este contexto possui ou usa}]
operacoes: [{processos do léxico — .claude/lexicon/operacoes.md}]
keywords: [{até 12 termos que ainda não couberam nas facetas acima}]
---
# Spec — {contexto}

> **A spec viva diz o quê o contexto faz hoje — nunca o como.** Nada de
> linguagem, framework, tabela física, endpoint ou nome de arquivo: o como
> está no código, e o grafo o responde.
>
> **Ela não é editada à mão.** Nasce do arquivamento da primeira mudança
> (`gofi change archive`), ou do modo baseline do gofi-spec num contexto que
> já existe, e muda só por mudanças arquivadas. Editada fora delas, o selo
> quebra e o `gofi spec check` reprova.
>
> **Requisito:** `### Requisito: REQ-NN — título`, com DEVE / NÃO DEVE (EARS
> quando há gatilho: "QUANDO {evento}, o sistema DEVE {resposta}").
> **Cenário:** `#### Cenário: REQ-NN.n — título`, com `- DADO`, `- QUANDO`,
> `- ENTÃO` (e `- E` para continuar). Todo requisito tem ao menos um. Os IDs
> são estáveis: requisito removido deixa o número vago, nunca reaproveitado.

## Propósito

{O que este contexto é, o problema que resolve e onde ele termina — 2 a 4 frases.}

## Requisitos

### Requisito: REQ-01 — {Título do requisito}
O sistema DEVE {capacidade}.

#### Cenário: REQ-01.1 — {Título do cenário}
- DADO {estado}
- QUANDO {ação}
- ENTÃO {resultado observável}
