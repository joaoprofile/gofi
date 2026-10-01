# Auditoria pelo grafo

## Lei: auditar é consultar o grafo

**Auditar é consultar o grafo — nunca varrer o código por reflexo (LEI
absoluta).** Todo achado nasce de `gofi find --in code` (achar) e `gofi
show` (entender quem chama); `grep -r`/`Glob`
deliberado atrás de código é violação — a busca literal sancionada é
`gofi find --text`/`--regex`. O gate **não** é *"isto é símbolo?"*
— essa pergunta se responde de cabeça, sem consultar nada, e é exatamente
por ela que a auditoria volta a ser uma varredura de arquivos; o gate é
*"**eu já consultei o grafo (`gofi find --in code`/`gofi show`)?**"*. **Uma** chamada antes da primeira busca literal,
sempre, inclusive quando o alvo parece fora do índice (`const`/`var`,
diretiva em comentário, string) — aí o movimento certo é `gofi show` no
**símbolo concreto que o referencia** (o DTO, o service, o tipo). E
consulta vazia **não autoriza a busca literal automaticamente**: vazio quase sempre
é pergunta mal formulada. A escada é (1) reformular no grafo — dois termos,
ou o vizinho concreto; (2) se o achado é de **ausência** (§"Validar o modo do grafo antes de escrever \"não há violação\""), `gofi index
code --deep` (varrer não substitui: a busca literal acha texto, não resolve dispatch por
interface); (3) grafo stale numa cadeia `eng → qa` (`gofi index status`)?
`gofi index code` e repita; (4) **só então** busca literal — `gofi find
--text`/`--regex` (`-i`, `--in code`) — para o que não é símbolo (string,
SQL, chave de config, tag, mensagem de erro) ou para linguagem sem
extractor (ela lê todo arquivo do projeto, qualquer linguagem); cada linha
vem sob o `◆ Símbolo`/`§Seção` que a contém — e **o laudo declara** cada
queda ("verificado por busca literal porque X"). `grep`/`Glob` puro só se o
`gofi` estiver indisponível. Protocolo:
`.claude/expertise/harness-protocols/graph-retrieval.md`.

## Auditar a corrente do contexto

**Auditar a corrente do contexto.** `gofi show ctx:{contexto}` mostra spec, PRD, memória, tabelas, pacotes de código e as **lacunas** — PRD sem spec, spec que ninguém referencia, contexto sem pacote marcado `//gofi:context`. Lacuna é achado de auditoria, não ruído; um PRD em discovery legitimamente ainda não tem spec, então reporte com o julgamento, não como falha automática.

## Auditar é consultar o grafo, não varrer o código

**Auditar é consultar o grafo, não varrer o código com `grep`.** Rode `gofi index code` **primeiro** (incremental, barato; `gofi index status` diz se o grafo ficou para trás): o hook de pre-commit só o atualiza no commit, que ainda não aconteceu, e se o `gofi-eng` não o reconstruiu você auditaria um mapa sem a implementação. Depois `gofi_graph_index.json` (que escopos existem e **em que pasta** — backend em `.`, cada superfície em `{nome}/`, SDK em `sdk/`) → o `gofi_graph_report.md` **do escopo auditado** (as §"Ciclos de chamada" e §"Conexões inesperadas" são evidência direta de violação de camada) → `gofi show <símbolo>` nos pontos auditados; quando não souber o nome exato, `gofi find --in code "<termo> <termo>"` busca por nome parcial dentro do grafo. **Nunca** abra `gofi_graph.json`. **O gate da busca literal não é "isto é símbolo?", é "eu já consultei o grafo?"** — a pergunta de classificação você responde de cabeça, sem consultar nada, e é por ela que a auditoria volta a ser uma varredura de arquivos. Uma consulta (`gofi show` pelo alvo, ou `gofi find --in code` por dois termos quando não souber o nome) **antes** da primeira busca literal, mesmo quando você tem certeza de que o alvo é `const`/`var`/comentário/string. Item do checklist que é textual por natureza (SQL concatenado, import proibido, string, chave de config) se verifica com `gofi find --text`/`--regex --in code` — cada linha sai sob o `◆ Símbolo` que a contém, que é o que abrir —, nunca com `grep -r`. Caiu na busca literal? **Declare no laudo** que caiu e por quê. Protocolo: `.claude/expertise/harness-protocols/graph-retrieval.md`

## Validar o modo do grafo antes de escrever "não há violação"

**Valide o modo do grafo antes de escrever "não há violação".** Todo item deste checklist que afirma **ausência** ("service NÃO importa X", "handler não acessa repository") é uma prova de negativa, e só o modo `deep` a sustenta: em `fast` a chamada ambígua não vira aresta. Leia o `mode` do escopo no `gofi_graph_index.json` (o `report.md` também o traz no cabeçalho e a contagem de ambíguas no §Resumo). `gofi index code` reconstrói no modo do `.gofi.yaml` — **`fast` por padrão**, a menos que o projeto declare `graph.deep: true` —, então um grafo recém-reconstruído continua sendo `fast` na maioria dos projetos. Se vier `fast`: rode `gofi index code --deep` antes de concluir, ou **registre no laudo** que a verificação foi sintática (e sugira `graph: deep: true` no `.gofi.yaml` se o projeto quiser exatidão nos builds deliberados). Numa superfície de UI o `deep` não existe — o extractor TS/JS é sintático e o escopo fica `fast` de todo jeito, então ali a limitação **sempre** se declara. Nunca apresente ausência de aresta em `fast` como prova. Gatilhos completos em *Quando rodar `--deep`* do protocolo do grafo.
