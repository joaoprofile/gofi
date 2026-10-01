# Input e Discovery — do pedido aos handlers

## Input

Tipos de pedido que você sabe atender:

1. **Arquivo aberto no IDE** — handler específico (ex.: `{aggregate}_handler.go`)
2. **Path explícito** — `services/domain/{contexto}/handler/...`
3. **Contexto inteiro** — nome do bounded context (descoberto da memória/projeto)
4. **Descrição funcional** — "endpoint de criação de configuração de X",
   "todos os endpoints públicos do contexto Y", "endpoint que devolve a
   listagem de Z"

Caso 4 exige **fase de Discovery** (§Discovery) — você precisa traduzir a
descrição em handlers concretos antes de documentar.

Sem input → arquivo aberto no IDE.


## Discovery — quando o input é descrição funcional

Pedido vago como "documente o endpoint de criação de configuração de X"?
Você não tem path. Procedimento — **pergunte ao humano antes de adivinhar**:

0. **Pergunte o contexto se o usuário não citou.** "Esse endpoint é de qual
   contexto?" — uma resposta curta resolve 80% da Discovery. Só prossiga
   sem perguntar se a descrição é inequívoca (cita o nome do contexto
   explicitamente). Os nomes de contexto válidos vêm de
   `.claude/memory/contexts/` (ou `/gofi-status`).
1. **Identifique o contexto candidato.** Cruze a descrição com a lista de
   contextos existentes (de `.claude/memory/contexts/` ou `/gofi-status`).
   Sem certeza, **pergunte ao grafo**: `gofi find --in code "<termo> <termo>"`
   devolve os símbolos que casam e o contexto de cada um. Só depois disso,
   `keywords` nos INDEX de `specs/`/`prd/`.
2. **Liste handlers do contexto.** `gofi show {contexto}/handler -n 40`
   lista os símbolos do pacote sem abrir arquivo (`ls` na pasta é o fallback) —
   em geral um handler por agregado.
3. **Mapeie verbos + paths.** Para cada handler, abra `Handlers()` e liste
   todas as rotas. Confronte com a descrição: "criação" → `POST`;
   "configuração" → handler de config; "listar" → `GET` plural; "detalhe" →
   `GET /{id}`.
4. **Cross-check no composition root.** Confirme em `services/{api-service}/wire.go`
   (nome real vem do passo 3 da pré-execução, `project-discovery.md`) que o handler está montado e
   se é público vs privado.
5. **Confirme se houver ambiguidade.** "Encontrei dois candidatos:
   `POST /v1/{contexto}/{id}/config` e `POST /v1/{contexto}/rule`. Qual você
   quer documentar (ou ambos)?" — se inequívoco, prossiga sem perguntar.
6. **Se a descrição não bate com nenhum handler existente**, NÃO invente:
   responda ao usuário "não encontrei endpoint que case com '{descrição}'
   em `services/domain/`. Talvez seja outro contexto, ou ainda não esteja
   implementado. Você tem um arquivo/path mais específico?"

Em qualquer passo onde a Discovery falhe, **pergunte mais detalhes antes de
gerar doc** — doc inventada é pior do que doc ausente.

---
