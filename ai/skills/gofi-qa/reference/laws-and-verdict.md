# gofi-qa — leis, severidade e saída

O que toda auditoria segue: o `gofi-qa` e cada uma das suas lentes.

## Leis

**Leis comuns** (AGENTS.md §Leis comuns): 1 portável · 2 índice primeiro · 3 fechar reindexa · 4 versão = produção · 7 só o combinado.

1. **Bug fix ou melhoria sem teste de regressão → MAJOR (LEI absoluta).** O QA
   audita isso explicitamente; fix confirmado sem teste que o trave é
   reprovação. → `.claude/expertise/code-review/checklist-cross-language.md` §Lei do teste de regressão (MAJOR)
2. **Auditar é consultar o grafo (LEI absoluta).** Todo achado nasce de `gofi find --in code`/`gofi show`;
   o gate da busca literal é *"eu já consultei o grafo?"*, e **o laudo declara** cada
   queda. → `reference/graph-audit.md` §Lei: auditar é consultar o grafo
3. **Ausência só se prova em `deep`.** Nunca apresente ausência de aresta em
   `fast` como prova. → `reference/graph-audit.md` §Validar o modo do grafo

---

## Severidade de problemas

| Nível | Descrição | Exemplo |
|-------|-----------|---------|
| **BLOCKER** | Impede funcionamento correto | SQL injection, panic em produção, retorno errado |
| **MAJOR** | Viola padrão do SDK ou introduz bug latente | Service retornando `error` puro, stmt não preparado |
| **MINOR** | Desvio de convenção sem impacto funcional | Import desordenado, nome fora de padrão |
| **SUGGESTION** | Melhoria opcional | Extrair constante, refinar mensagem |

---

## Saída e memória

- **Veredicto como dado:** `gofi verdict write {contexto}` com o JSON pela entrada padrão — `result` (`aprovado` | `reprovado`) e um achado por problema: `severity` (blocker, major, minor, suggestion), `rule` (o cenário `{ctx}/REQ-NN.n`, o princípio ou a regra do checklist), `symbol` quando há um nó do grafo, `file`/`line`, `owner` e `message`. O `owner` diz quem corrige: `eng` (o código não cumpre a mudança), `spec` (o contrato está errado ou incompleto — cenário, requisito, design) ou `pd` (lacuna de negócio). O `file` é **onde a correção vai**: achado que se corrige com um teste (falta o teste de um cenário, a regressão de um defeito corrigido, uma asserção que não prova) aponta o arquivo de teste — existente ou o que deveria existir —, nunca o código testado; o loop leva achado de teste a quem escreve os testes, e de código a quem escreveu o código. O comando confere a coerência, calcula a impressão digital e grava `status` e `ressalvas` na memória — é o que o condutor lê. → `reference/memory-and-spec-update.md` §3
- **Memória do contexto:** refresh do `## Estado atual` + 1 linha no `## Histórico de versões` (`gofi memory write`), **antes** do veredicto — ele grava `status`, `ressalvas` e `atualizado` por cima.

  | Veredicto | `status` | `ressalvas` |
  |---|---|---|
  | ✅ Aprovado | `aprovado` | `0` |
  | ⚠️ Aprovado com ressalvas | `aprovado` | nº de MAJOR + MINOR pendentes |
  | ❌ Reprovado | `reprovado` | nº de MAJOR + MINOR pendentes |

  Só `aprovado` com `ressalvas: 0` encerra o ciclo; SUGGESTION não conta.
- **Spec viva e mudança: não se editam.** Ambas são seladas (decisão 0003). O
  resultado da auditoria vive na memória do contexto; a versão da spec viva sobe
  quando a mudança aceita é arquivada (`gofi change archive`). Drift entre o
  código e a mudança é **achado**, com a severidade dele — nunca uma correção na
  spec. Achado que é lacuna do contrato (cenário errado, requisito faltando) diz
  isso no laudo: quem corrige é o `/gofi-spec`, numa mudança.
- Padrão novo → `.claude/sdk/<lang>/knowledge/<topico>.md`, para o `gofi-eng` evitar reincidência.
- Detalhe completo → `reference/memory-and-spec-update.md`; aprendizado → `.claude/expertise/harness-protocols/learning.md`.

### Output esperado

```
