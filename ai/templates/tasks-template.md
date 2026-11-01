---
tipo: tarefas
formato: 3                        # versão do formato do corpus — não invente outro valor
contexto: {contexto}
versao: "1.0"
status: derivado                  # gerado pelo gofi-plan de deltas + design + grafo; nunca editado à mão
keywords: [{até 12 termos}]
atualizado: {YYYY-MM-DD}
---
# Tarefas — {mudanca}

> Derivado: regenerado quando um delta ou o design mudam, e marcado pelo
> executor conforme as tarefas fecham. Uma tarefa é o que um papel entrega e
> um sensor verifica, em ordem de dependência, agrupada por camada.
>
> Formato: `- [ ] N.n [P]? [{camada}] {o quê} — {de onde: design §, REQ-NN}`.
> `[P]` marca a tarefa paralelizável — arquivos disjuntos das vizinhas,
> segundo o grafo. A camada é a do pack (`data`, `domain`, `api`, `adapter`,
> `ui`, `infra`, `test`).

## 1. {Camada}

- [ ] 1.1 [{camada}] {O que fazer} — design §{seção} ({REQ-NN})
