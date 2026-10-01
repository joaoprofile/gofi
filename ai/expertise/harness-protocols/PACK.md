---
pack: harness-protocols
title: Protocolos do harness
summary: como todo papel lê o índice e o grafo, grava memória, versiona documentos e aprende com correções
applies_when:
  always: true
serves: [pd, spec, eng, ui, ops, qa, doc]
---

# Protocolos do harness

Valem para toda tarefa, de qualquer papel: são o contrato de como o agente lê e escreve no projeto.

| Seção | Quando |
|---|---|
| `rag-retrieval.md` | ler specs, PRDs e memória gastando poucos tokens; criar documento com frontmatter e keywords |
| `graph-retrieval.md` | ler o código pelo grafo antes de abrir arquivo; frescor depois de escrever |
| `memory.md` | o que cada fase grava em `memory/contexts/{contexto}.md` ao concluir |
| `document-versioning.md` | quando subir a versão de PRD e spec |
| `learning.md` | correção, ensino ou validação do usuário: onde registrar e o que oferecer ao upstream |
| `llm-scoring-prompt.md` | pontuar relevância com LLM perguntando identidade, não semelhança |
