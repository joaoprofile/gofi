# Protocolo de Consulta ao Grafo — ler o código sem abrir arquivos

> **Cross-agent, portável.** Vale, sem mudar uma palavra, para qualquer projeto
> com o mesmo SDK. Governa como **todo** agent consulta o grafo de código
> (`.gofi/graph/`) e como o mantém atualizado. Objetivo único: responder
> *"quem chama isso?"*, *"o que quebra se eu mudar?"*, *"onde mora esse
> contexto?"* **sem varrer a árvore** e sem ler código que não vai editar.

## Princípio

O grafo é o **mapa**; o código-fonte é o **território**. Os corpora de
`rag-retrieval-protocol.md` (specs, PRDs, memória) dizem **o que** deve existir;
o grafo diz **o que existe de fato e como está ligado**. Você consulta o mapa
primeiro e só abre o território nos arquivos que o mapa apontou.

O par de entrada é a diretiva **`//gofi:context {contexto}`**: é ela que amarra
um símbolo do grafo de volta a `specs/{contexto}/` e a
`memory/contexts/{contexto}.md`. Sem ela, o grafo sabe o que chama o quê mas não
sabe a que contexto pertence — e o agent volta a adivinhar por nome de pacote.

## Onde o grafo mora — um escopo por árvore de código

Um projeto não é uma base de código só. O backend, cada superfície de UI e o
SDK vendorizado são **escopos** independentes, cada um com o seu grafo, porque
cada um é lido por um extractor diferente. As pastas saem do `.gofi.yaml` —
nunca de convenção.

| Escopo | Pasta do grafo | O que contém |
|--------|----------------|--------------|
| `project` | `.gofi/graph/` | o backend declarado em `backend.path` |
| `frontend`, `mobile` e cada chave de `surfaces:` | `.gofi/graph/{nome-do-escopo}/` | a árvore daquele `path`, lida pelo extractor TS/JS |
| `sdk` | `.gofi/graph/sdk/` | o SDK vendorizado em `.gofi/gofi-sdk-<lang>/` |
| linguagem construída à parte (`gofi graph build --lang java`) | `.gofi/graph/{lang}/` | índice e escopos próprios |

Cada pasta dessas carrega o **mesmo trio**: `gofi_graph.json`,
`gofi_graph_report.md` e `gofi_graph.html`. Quem diz onde cada uma está é o
índice, e é por isso que ele é o primeiro arquivo a abrir.

> **`--lang` não é o seletor de superfície.** Superfície declarada no
> `.gofi.yaml` é escopo do índice principal: `gofi graph explain <símbolo>`
> **sem flag** já procura em todos os escopos, na ordem do índice (projeto
> primeiro, SDK depois). `--lang` só serve para uma linguagem com índice próprio
> (extractor instalado + `build --lang`); apontá-lo para uma superfície busca
> numa pasta que não existe e o comando responde *"grafo ausente"*.

## Os artefatos e seu custo

Gerados por `gofi graph build` — **nunca** editados à mão.

| Artefato | O que responde | Custo | Ler? |
|----------|----------------|-------|------|
| `gofi_graph_index.json` | Quais escopos existem, a pasta de cada um, linguagem, framework, **modo do scan**, tamanho | dezenas de linhas | **Sempre** — é o ponto de entrada |
| `gofi_graph_report.md` (do escopo relevante) | Resumo, pacotes, pontos centrais, comunidades, ciclos, conexões inesperadas | ~1 tela (limitado por construção) | **Quase sempre** — o panorama |
| `gofi graph explain <símbolo>` | Origem, assinatura, doc, o que chama, quem chama, tipos que toca, **contexto** | ~30 linhas por chamada | **Sob demanda**, um símbolo por vez |
| `gofi graph explain <termo> <termo>` (≥2 palavras) | **Busca**: cada símbolo que casa com o termo, com a vizinhança de cada um | ~30 linhas por resultado (`--limit` corta) | Quando você **não sabe o nome exato** — é esta a substituta do `grep -r` |
| `gofi graph explain <A> --to <B>` | O caminho de A até B, aresta por aresta | poucas linhas | Quando a pergunta é *"como A alcança B"* |
| `gofi_graph.json` | O grafo cru | **milhares de linhas** | ❌ **Nunca** |
| `gofi_graph.html` | Visualização para humano | — | ❌ Nunca (é para o usuário, via `gofi graph open`) |

Nomes aceitam forma curta: `NewServer`, `api.NewServer` e o ID completo
resolvem igual. Quando o termo casa com vários, o `explain` mostra o mais
central e lista os demais candidatos.

## Leitura — a escada de 3 degraus (sempre nesta ordem)

1. **`index.json` → orientação.** `.gofi/graph/gofi_graph_index.json`. Descubra
   **quais escopos existem, em que pasta** e a linguagem de cada um antes de
   qualquer pergunta — é o `dir` de cada escopo que diz onde está o `report.md`
   que você vai ler no passo 2. Ele também traz o **modo** do último scan de
   cada escopo (`fast`/`deep`), que determina o quanto você pode concluir das
   arestas (ver *O modo do scan* abaixo). Se não existir, o último build foi
   feito com caminho explícito (`gofi graph build ./algum/path`), que não
   escreve índice — siga pelo `report.md`.
2. **`report.md` → panorama.** Leia o relatório **do escopo relevante**
   (`.gofi/graph/{dir-do-escopo}/gofi_graph_report.md`). Ele já entrega os
   pacotes, os pontos centrais (o que muita coisa depende), os ciclos e as
   **conexões inesperadas** — que é exatamente onde uma mudança costuma quebrar
   algo distante. Não precisa de `grep` para isso.
3. **`explain` → precisão.** Para cada símbolo que a tarefa realmente toca, uma
   chamada de `gofi graph explain`. É ela que responde *quem chama* — a pergunta
   que antes exigia varrer o módulo inteiro com `grep`. Não sabe o nome exato?
   **Busque no próprio grafo** com dois ou mais termos
   (`gofi graph explain criar pedido`), que devolve cada símbolo que casa com a
   vizinhança de cada um.

Só então abra os arquivos — **apenas** os que os passos 1–3 apontaram.

> **Regra de ouro:** procurar símbolo é `gofi graph explain`, não `grep -r`.
> O `grep` é o **fallback**, não o primeiro movimento, e quando você cai nele
> **diga que caiu**. Ele é o certo em três casos, todos declaráveis: não há
> grafo para aquela linguagem; o alvo não é símbolo (string literal, chave de
> config, SQL, tag de struct, nome de rota montado em runtime); ou o grafo está
> em `fast` e a resposta precisa ser exata — e nesse último caso a saída
> melhor é reconstruir com `--deep`, não varrer a árvore.

## O modo do scan — valide antes de concluir

O grafo tem dois modos, e **eles não respondem a mesma pergunta**:

| Modo | Como resolve as chamadas | O que você pode afirmar |
|------|--------------------------|--------------------------|
| `fast` (padrão) | heurística sintática; chamada ambígua **não vira aresta** — é contada e reportada | "isto chama aquilo" (aresta presente é evidência) |
| `deep` | type-checker: cada chamada resolvida, implementações de interface visíveis | também "**nada mais** chama isto" (ausência de aresta vira evidência) |

**Onde ler o modo:** o campo `mode` de cada escopo no
`gofi_graph_index.json`, e o cabeçalho do `report.md` (*"Gerado por gofi graph
(…, modo `fast`)"* + a linha do §Resumo com a contagem de ambíguas).

**Quem decide o modo:** `.gofi.yaml` → `graph: deep:`. `gofi init` escreve o
bloco e `gofi update` o semeia em quem não o tem, sempre com o valor que a
ausência já significava — **`deep: false`, ou seja `fast`**. Projeto antigo
ainda pode não ter o bloco; ausência lê igual a `fast`. Consequência prática, e
é a armadilha:

- `gofi init` e `gofi update` reconstroem **no modo configurado** — nunca forçam
  `deep`. Os **hooks de git** (`pre-commit`, `post-checkout`, `post-merge`)
  reconstroem **sempre em `fast`** (`--fast`), mesmo num projeto que declarou
  `deep: true`: o commit não espera o type-checker.
- Logo, com o default (`deep: false`) **todo grafo que você encontra pronto é
  `fast`**, por mais recente que seja; e mesmo com `deep: true`, o grafo que o
  último commit deixou no disco é `fast`. **Encontrar o grafo atualizado não é o
  mesmo que encontrá-lo exato.** O `mode` do índice é a resposta; o `.gofi.yaml`
  diz só o que o próximo build deliberado fará.
- Portanto **`deep` é sempre um pedido explícito** — de `gofi update` ou de
  `gofi graph build --deep` rodado por quem está prestes a concluir algo que o
  grafo `fast` não sustenta. Quando isso é, na seção seguinte.
- Se o projeto quer exatidão por padrão nos builds deliberados, o lugar é o
  `.gofi.yaml` (`graph:` → `deep: true`). Sugira ao dev; a escolha é dele.

## Quando rodar `--deep` — e quando não

`deep` custa: exige que o projeto compile e paga o type-checker. Não é o modo de
trabalho, é o **modo de prova**. A regra que decide é uma só:

> **Você vai afirmar uma ausência?** Então precisa de `deep`. Vai apenas
> navegar? `fast` basta — e já está no disco.

**Rode `gofi graph build --deep` antes de:**

- afirmar que **nada mais** usa um símbolo — "ninguém mais chama isto", "esta
  função ficou órfã", "pode remover";
- afirmar que **uma camada não alcança outra** — "handler não fala com
  repository", "o domínio não importa infra". Item de checklist que prova um
  negativo é ausência disfarçada;
- **remover ou renomear símbolo exportado**, ou mudar assinatura de artefato
  compartilhado (struct de `model/`, interface, enum, helper comum) — a análise
  de impacto que fecha a entrega só vale se enxergar todas as chamadas;
- responder qualquer pergunta sobre **implementação de interface** — "quem
  implementa `Repository`", "esta struct satisfaz aquele contrato". `fast` não
  enxerga implementação **de jeito nenhum**, não é questão de grau;
- concluir a partir de um relatório cujo §Resumo acusa **muitas chamadas
  ambíguas** — cada ambígua é uma aresta que `fast` viu e não ligou.

**Não rode `deep` para:** localizar um símbolo, ler a vizinhança de quem você já
vai editar, levantar o panorama de um pacote, descobrir por onde começar. Isso é
navegação, `fast` responde, e reconstruir só atrasa a tarefa.

**Quando não der para rodar** (o projeto não compila no meio da refatoração, a
linguagem não tem extractor nativo), a saída **não** é concluir mesmo assim: é
**declarar a limitação** onde a conclusão aparece — "auditado sobre grafo `fast`;
ausência de aresta não é prova de ausência de uso".

> **`deep` só muda a varredura Go.** O extractor TS/JS é sintático por
> construção: uma superfície de UI registra `mode: fast` no índice **mesmo depois
> de um `--deep`**. Isso não é build falhado — é o teto do extractor. Sobre front
> e mobile, ausência de aresta nunca é prova, e a limitação se declara sempre.

## Frescor — o grafo tem que refletir o que você acabou de escrever

O hook de pre-commit reconstrói o grafo **no commit**. O agent trabalha **antes**
do commit, e um pipeline encadeado (`/gofi-full`: `eng → qa`) não commita entre
as fases — então, sem ação explícita, o QA auditaria contra um mapa que **não
contém a implementação que acabou de ser feita**.

Por isso: **ao fechar uma implementação, antes de atualizar memória e spec**,
rode

```sh
gofi graph build --update
```

`--update` pula o scan inteiro quando nenhum arquivo-fonte mudou, então rodar é
barato e rodar sempre é seguro. A próxima skill da cadeia lê um mapa atualizado.
Ele reconstrói **no modo configurado**. Se a entrega mexeu em artefato
compartilhado, ou a conclusão que você vai escrever é uma ausência, é aqui que
entra `--deep` — ver *Quando rodar `--deep`*.

## Limites (declare-os, não os esconda)

- **Extractor nativo só para Go, TypeScript e JavaScript.** Java, C#, Rust e
  demais exigem um extractor instalado (`gofi graph install <lang>`). **Sem ele
  não há grafo** para aquela linguagem — caia no `grep` e diga que caiu.
- **Só o extractor Go lê `//gofi:context`.** No grafo de TS/JS o campo
  `contexto` vem vazio; a ponte grafo→spec, ali, ainda é o nome da pasta.
- **`fast` é heurística sintática** e é o que os hooks de git produzem sempre, e
  o que `init`/`update` produzem enquanto o `.gofi.yaml` não disser o contrário
  — ver *O modo do scan*. Confira o `mode` antes de tirar conclusão de ausência.
- **`deep` não alcança TS/JS.** O extractor de superfície é sintático; front e
  mobile ficam em `fast` mesmo sob `--deep`. Ali, ausência nunca é prova.
- **Superfície não se consulta com `--lang`.** Front e mobile são escopos do
  índice principal; `explain` sem flag já procura neles. `--lang` é só para
  linguagem com índice próprio (extractor instalado).
- **Grafo é estrutura, não comportamento.** Ele não sabe de reflexão, injeção
  por string, handler registrado por tabela nem SQL. Nessas, o `grep` continua
  soberano.

## Escrita — deixar o código legível pelo grafo

- **Todo pacote de um contexto nasce com a diretiva**, na cláusula `package`.
  Como os pacotes costumam ser nomeados pela **camada** (`model`, `service`,
  `repository`, `handler`), é a diretiva — não o nome do pacote — que carrega o
  contexto:

  ```go
  //gofi:context billing
  package model
  ```

  Na cláusula `package` ela basta em **um** arquivo do pacote e vale como
  default para **todos** os símbolos dele; uma declaração isolada pode
  sobrescrever com a sua própria — é assim que um símbolo que serve outro
  contexto se declara.
- **Contexto = o nome usado em `specs/{contexto}/` e
  `memory/contexts/{contexto}.md`.** Mesmo nome, kebab-case, sem inventar
  variação — é a chave de junção entre os três.
- **Não crie pacote-guarda-chuva** só para caber no grafo. O grafo descreve a
  estrutura que o DDD já decidiu; não é ele que decide a estrutura.
- **Repositório adotado** (`gofi init` sobre base existente) nasce sem diretiva
  nenhuma: o grafo enxerga as chamadas e o `contexto` vem vazio em tudo. A ponte
  se constrói em duas fases — `/gofi-spec` levanta o mapa pacote→contexto a
  partir das **comunidades** do relatório e o confirma com o dev; `/gofi-eng`
  grava as diretivas e reconstrói o grafo. A fronteira de contexto é decisão do
  dev; o grafo só entrega a evidência.

## Anti-padrões (não faça)

- ❌ Ler `gofi_graph.json` (ou pedir para o usuário colar) — o `report.md` e o
  `explain` existem exatamente para isso.
- ❌ Começar por `grep -r` no módulo quando a pergunta é *"quem chama X"* e
  existe grafo da linguagem. Nem para **achar** o símbolo: `gofi graph explain
  <termo> <termo>` busca por nome parcial dentro do próprio grafo.
- ❌ Concluir de um scan `fast` que **nada** quebra — ausência de aresta ali não
  é prova de ausência de uso. Sem checar o `mode`, você não sabe em que modo
  está o grafo que acabou de ler.
- ❌ Ler o `report.md` da raiz achando que ele cobre o front: cada escopo tem o
  seu, na pasta que o índice aponta.
- ❌ Fechar uma implementação sem `gofi graph build --update` (a próxima fase
  herda um mapa desatualizado).
- ❌ Editar qualquer arquivo de `.gofi/graph/` à mão — é artefato derivado, é
  sobrescrito no próximo build.
- ❌ Criar pacote de contexto sem `//gofi:context` (fica invisível à ponte
  grafo→spec).
