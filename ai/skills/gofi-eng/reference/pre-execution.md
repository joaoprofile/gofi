# Pré-execução do gofi-eng — passos completos

## Pré-execução obrigatória

Antes de qualquer linha de código:

1. Ler `.gofi.yaml` (raiz) — extrair `project.language`, `project.name` e demais campos
2. O `AGENTS.md` da raiz (já carregado) — mapa de paths físicos do projeto
3. Ler `.claude/memory/project.md` — visão global, serviços e convenções (sem estado por-contexto; rode `/gofi-status` para o índice de contextos)
4. Ler `.claude/memory/contexts/{contexto}.md` se existir — frontmatter + handoff do gofi-spec
5. Ler a spec — **fonte da verdade**. **Procurar documento é `gofi find`, não carregar índice:** `gofi find "<o que você precisa saber>"` — devolve o documento, a §seção e a **faixa de linhas**; leia com `Read(offset, limit)`, nunca o arquivo inteiro. Vazio quase sempre é vocabulário, não documento faltando: tente o termo técnico e registre o par que faltou em `.claude/lexicon/sinonimos.md`. Os `INDEX.md` servem para **navegar** um contexto, não para **achar** um assunto. Para o contexto inteiro de uma vez, `gofi show ctx:{contexto}` mostra spec, PRD, memória, tabelas, pacotes de código e as lacunas da corrente. Nunca leia a spec inteira por reflexo. Protocolo: `.claude/expertise/harness-protocols/rag-retrieval.md`
5a. **Procurar código é `gofi find --in code` (achar) e `gofi show` (entender quem chama), não `grep -r`.** Se `.gofi/index/code/` existir: `gofi_graph_index.json` (que escopos existem, **em que pasta** — backend em `.`, cada superfície em `{nome}/`, SDK em `sdk/` — e **em que modo** cada um foi varrido) → o `gofi_graph_report.md` **daquele escopo** (pacotes, pontos centrais, conexões inesperadas) → `gofi show <símbolo>` só nos símbolos que a tarefa toca. Não sabe o nome exato? `gofi find --in code "<termo> <termo>"` (≥2 palavras) busca por nome parcial dentro do grafo — é isso que substitui o `grep` por símbolo. Superfície declarada no `.gofi.yaml` **não** usa `--lang`: já é escopo do índice principal. **Nunca** abra `gofi_graph.json`. A busca literal (`gofi find --text`/`--regex`) é fallback **declarado**, e o gate dela **não** é "isto é símbolo?" — essa pergunta você responde de cabeça, sem consultar nada, e é por ela que o reflexo de varrer o repositório volta. O gate é "**eu já consultei o grafo (`gofi find --in code`/`gofi show`)?**": uma chamada antes da primeira busca literal, sempre, mesmo quando o alvo parece `const`/`var`/comentário/string (nesses, o movimento melhor costuma ser `gofi show` no **símbolo concreto que os referencia** — o DTO, o service, o tipo). Motivos legítimos de fallback: linguagem sem extractor, a consulta voltou vazia, ou alvo que de fato não é nó (string, chave de config, SQL). A busca literal lê todo arquivo do projeto (qualquer linguagem, inclusive o SDK vendorizado) e agrupa cada linha sob o `◆ Símbolo`/`§Seção` que a contém; `grep -r`/`Glob` puro só se o `gofi` estiver indisponível. Protocolo: `.claude/expertise/harness-protocols/graph-retrieval.md`
6. Ler **knowledge cross-agent**: `.claude/knowledge/INDEX.md` (núcleo ⬤ + só os módulos que a tarefa pede) (inclui `expertise/diagramming/conventions.md` — qualquer diagrama de fluxo em ADR/comentário deve ser PlantUML)
7. Ler **knowledge per-agent**: `.claude/knowledge/eng/*.md` (user-treinado)
8. Para `project.language` (a partir do `.gofi.yaml`):
   - Ler **regras absolutas** e **estrutura**: `.claude/sdk/<lang>/knowledge/{absolute-rules,structure,layers,naming}.md`
   - Ler armadilhas relevantes: os módulos de `sdk/<lang>/knowledge/` que o `.claude/knowledge/INDEX.md` indicar (inclui
     `lookup-endpoints.md` se o contexto declarar campos `search-multiple`
     no `{Ctx}FilterMapping` — define `FilterType`/`SearchType`/`Content` no
     próprio mapping, sem endpoint `/status`;
     inclui `bridge-factory-adapter-pattern.md` se o contexto integra com
     N implementações intercambiáveis de dimensão externa — marketplaces,
     gateways, shippers)
   - Ler API do SDK (gerada do código): `.claude/sdk/<lang>/api/INDEX.md` primeiro, depois só os arquivos dos pacotes pertinentes (ou `gofi find --in sdk "<símbolo>"`); o *como usar* cada módulo está em `.claude/sdk/<lang>/knowledge/*.md`
   - Ler boilerplates por camada: `.claude/sdk/<lang>/boilerplates/*.md`
9. Confirmar com o dev se há ambiguidades **antes** de escrever código
10. **Perguntar onde está o código legado/base** sempre que a tarefa for refactor, migração de formato, reescrita ou reestruturação (mover camadas, trocar padrão, eng. reversa). O código existente é a **base de referência** do novo formato — peça o caminho (pasta/arquivo/binário legado) e leia antes de gerar. Não reescreva do zero quando há legado: o objetivo é preservar comportamento e migrar para o formato-alvo da spec.
11. **Se a tarefa edita um contexto já implementado** (e não cria do zero), fazer **análise de impacto detalhada antes de fechar**: toda mudança em artefato compartilhado (struct de `model/`, enum/`kafka.Type*`, interface, coluna de migration, helper comum) tem contrato implícito com **todos** os consumidores. **Levante os consumidores pelo grafo, não varrendo o módulo:** `gofi show <símbolo>` lista quem chama; `gofi path <A> <B>` mostra o caminho. Como a conclusão aqui é "quebra / não quebra", exija um grafo **`deep`** — no modo `fast` a chamada ambígua não vira aresta, e ausência de aresta **não** é prova de ausência de uso. **Confira o `mode` do escopo no `gofi_graph_index.json` antes**: `gofi index code` reconstrói no modo do `.gofi.yaml`, que é `fast` a menos que o projeto declare `graph.deep: true` — grafo recente não quer dizer grafo exato. Se vier `fast`, rode `gofi index code --deep` você mesmo; os demais gatilhos de `deep` (e o limite de que ele não alcança TS/JS) estão em *Quando rodar `--deep`* do protocolo do grafo. Sem grafo da linguagem (ou para alvo que não é símbolo — SQL, string de config, registro por tabela), caia na busca literal (`gofi find --text`/`--regex --in code`) e diga que caiu. Classifique cada consumidor (válido/ajuste/quebra), ajuste todos na mesma entrega, e `build`+`test` dos pacotes **consumidores** — não só o editado. Procedimento e casos de quebra silenciosa (scan posicional do `sqln`, coluna de `JOIN`, enum sem destino) em `.claude/expertise/ddd-architecture/impact-analysis-on-change.md`.

> Se algo na spec for ambíguo ou contradizer um padrão do `sdk-knowledge`,
> pare e pergunte. Nunca infira.

> **Execução sempre step by step.** Trabalhe em passos pequenos e verificáveis,
> confirmando cada etapa com o dev antes de seguir — especialmente em
> refactor/migração. Não despeje a mudança inteira de uma vez: faça um passo,
> mostre o resultado, valide (`go build`/`go test` quando aplicável), e só
> então avance para o próximo.
