# Protocolo de Consulta ao Grafo — ler o código sem abrir arquivos

> **Cross-agent, portável.** Vale, sem mudar uma palavra, para qualquer projeto
> com o mesmo SDK. Governa como **todo** agent consulta o grafo de código
> (`.gofi/index/code/`) e como o mantém atualizado. Objetivo único: responder
> *"quem chama isso?"*, *"o que quebra se eu mudar?"*, *"onde mora esse
> contexto?"* **sem varrer a árvore** e sem ler código que não vai editar.

## Princípio

O grafo é o **mapa**; o código-fonte é o **território**. Os corpora de
`rag-retrieval.md` (specs, PRDs, memória) dizem **o que** deve existir;
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
| `project` | `.gofi/index/code/` | o backend declarado em `backend.path` |
| `frontend`, `mobile` e cada chave de `surfaces:` | `.gofi/index/code/{nome-do-escopo}/` | a árvore daquele `path`, lida pelo extractor TS/JS |
| `sdk` | `.gofi/index/code/sdk/` | o SDK vendorizado em `.gofi/gofi-sdk-<lang>/` |
| outra linguagem sobre a árvore do backend (`gofi index code --lang java`) | `.gofi/index/code/{lang}/` | a árvore do backend lida pelo extractor daquela linguagem — mais um escopo do mesmo índice |

Cada pasta dessas carrega o **mesmo trio**: `gofi_graph.json`,
`gofi_graph_report.md` e `gofi_graph.html`. Quem diz onde cada uma está é o
índice, e é por isso que ele é o primeiro arquivo a abrir.

> **`--lang` não é o seletor de superfície.** Superfície declarada no
> `.gofi.yaml` é escopo do índice principal: `gofi show <símbolo>` e
> `gofi find --in code` **não têm flag de linguagem** e já procuram em todos os
> escopos, na ordem do índice (projeto primeiro, SDK depois). `--lang` é flag
> **de build** (`gofi index code --lang <lang>`, extractor instalado): lê a
> árvore do backend com o extractor daquela linguagem e arquiva o resultado
> como mais um escopo do mesmo índice — que a consulta alcança sem flag.

## Os artefatos e seu custo

Gerados por `gofi index code` — **nunca** editados à mão.

| Artefato | O que responde | Custo | Ler? |
|----------|----------------|-------|------|
| `gofi_graph_index.json` | Quais escopos existem, a pasta de cada um, linguagem, framework, **modo do scan**, tamanho | dezenas de linhas | **Sempre** — é o ponto de entrada |
| `gofi_graph_report.md` (do escopo relevante) | Resumo, pacotes, pontos centrais, comunidades, ciclos, conexões inesperadas | ~1 tela (limitado por construção) | **Quase sempre** — o panorama |
| `gofi show <símbolo>` | Origem, assinatura, doc, o que chama, quem chama, tipos que toca, **contexto** | ~30 linhas por chamada | **Sob demanda**, um símbolo por vez |
| `gofi find --in code "<termo> <termo>"` (≥2 palavras) | **Busca**: cada símbolo que casa com o termo, com `arquivo:linhas` — a vizinhança vem do `gofi show` | poucas linhas por resultado (`-n` corta; padrão 5) | Quando você **não sabe o nome exato** — é esta a substituta do `grep -r` |
| `gofi find --text "<literal>"` / `--regex "<re>"` (`-i` ignora caixa) | **Texto exato**: cada linha que casa, agrupada sob a unidade que a contém — `◆ Símbolo L12-19` no código, `§Seção L40-60` em documento —, em todo arquivo do projeto (inclusive o SDK vendorizado; nunca `.gofi/index`) | até 30 linhas (`-n` muda; `--in` estreita), fecha com *"N linhas em M arquivos"* | Quando o alvo é **texto, não declaração** — o último degrau do fallback (ver abaixo) |
| `gofi path <A> <B>` | O caminho de A até B, aresta por aresta | poucas linhas | Quando a pergunta é *"como A alcança B"* |
| `gofi_graph.json` | O grafo cru | **milhares de linhas** | ❌ **Nunca** |
| `gofi_graph.html` | Visualização para humano | — | ❌ Nunca (é para o usuário, via `gofi index open`) |

Nomes aceitam forma curta: `NewServer`, `api.NewServer` e o ID completo
resolvem igual. Quando o nome casa com mais de uma coisa, o `show` não
adivinha: lista os candidatos com a referência prefixada de cada um (`sym:`,
`doc:`, `ctx:`, `table:`) para você escolher.

## Leitura — a escada de 3 degraus (sempre nesta ordem)

1. **`index.json` → orientação.** `.gofi/index/code/gofi_graph_index.json`. Descubra
   **quais escopos existem, em que pasta** e a linguagem de cada um antes de
   qualquer pergunta — é o `dir` de cada escopo que diz onde está o `report.md`
   que você vai ler no passo 2. Ele também traz o **modo** do último scan de
   cada escopo (`fast`/`deep`), que determina o quanto você pode concluir das
   arestas (ver *O modo do scan* abaixo). Se não existir, o grafo ainda não foi
   construído — rode `gofi index code` (`gofi index status` diz, sem construir,
   se cada escopo está `fresh`, `stale`, `unknown` ou `missing`).
2. **`report.md` → panorama.** Leia o relatório **do escopo relevante**
   (`.gofi/index/code/{dir-do-escopo}/gofi_graph_report.md`). Ele já entrega os
   pacotes, os pontos centrais (o que muita coisa depende), os ciclos e as
   **conexões inesperadas** — que é exatamente onde uma mudança costuma quebrar
   algo distante. Não precisa de `grep` para isso.
3. **`show` → precisão.** Para cada símbolo que a tarefa realmente toca, uma
   chamada de `gofi show <símbolo>`. É ela que responde *quem chama* — a
   pergunta que antes exigia varrer o módulo inteiro com `grep`. Não sabe o nome
   exato? **Busque no próprio índice** com dois ou mais termos
   (`gofi find --in code "criar pedido"`), que devolve cada símbolo que casa,
   com `arquivo:linhas`; o `show` do escolhido traz a vizinhança. *Como A
   alcança B* é `gofi path A B`.

Só então abra os arquivos — **apenas** os que os passos 1–3 apontaram.

> **Regra de ouro — a busca literal começa depois de um `show`/`find` que não respondeu.**
> O gate **não** é "isto é símbolo?"; é "**eu já consultei o índice?**". A
> pergunta de classificação parece equivalente e não é: ela se responde de
> cabeça, antes de qualquer consulta, e é por ela que o reflexo de varrer o
> repositório volta — você decide sozinho que o alvo "não era símbolo" e o grafo
> nunca chega a ser consultado. A consulta é barata (~370 ms, ~30 linhas) e
> erra para menos, não para mais: quando ele não tem o alvo, responde vazio em
> uma chamada e aí sim a busca literal (`gofi find --text`) é o movimento certo.
>
> Na prática: **uma** consulta — `gofi show` pelo alvo, ou `gofi find --in code`
> com dois termos se você não sabe o nome — antes da primeira busca literal
> (`gofi find --text`) ou `grep`. Caiu na busca literal mesmo assim?
> **Diga que caiu e por quê.**

### A escada do fallback — a busca literal é o último degrau, e só se for o caso

`show`/`find` vazio **não autoriza a busca literal automaticamente**. Vazio quase sempre
significa *pergunta mal formulada*, não *alvo ausente*. Antes de varrer, suba
os degraus na ordem — e pare no primeiro que responder:

1. **Reformule a busca no próprio grafo.** Nome exato errado?
   `gofi find --in code "<termo> <termo>"` (≥2 palavras) casa por nome
   parcial. Alvo que não é nó (`const`/`var`, diretiva, import)? `gofi show` no
   **vizinho concreto que o referencia** — o DTO, o service, o tipo.
2. **A pergunta é de ausência?** ("ninguém mais chama", "esta camada não alcança
   aquela", "quem implementa esta interface"). Então o problema não é a busca
   literal ser necessária — é o modo. `gofi index code --deep`. Varrer a árvore
   **não** substitui isso: `gofi find --text` (como o `grep`) acha o texto, não
   resolve dispatch por interface — só o grafo (`show`, sob `--deep`) resolve.
3. **O grafo está stale?** Você escreveu código nesta sessão, ou
   `gofi index status` acusa `stale` — `gofi index code` (incremental: só as
   árvores que mudaram) e repita o passo 1.
4. **Só então `gofi find --text "<literal>"`/`--regex "<re>"`**, e apenas nos
   casos legítimos: não há extractor para aquela linguagem; ou o alvo é
   comprovadamente texto e não declaração (string literal, SQL, chave de config,
   tag de struct, import, rota montada em runtime, mensagem de erro). Cada
   linha vem sob o símbolo ou a §seção que a contém — o que o `grep` não diz —,
   e `--in` estreita a área. **Sempre declarado** — "caí na busca literal
   porque X". `grep`/`Grep` puro só quando o `gofi` não alcança (árvore fora
   do projeto, binário indisponível) — igualmente declarado.

## O que o grafo não indexa como nó — pergunte pelo vizinho concreto

O grafo é feito de **declarações**: tipos, funções, métodos, interfaces,
structs. Não é feito de todo texto que existe no arquivo. Ficam de fora, e o
`show` responde vazio para eles:

| Alvo | Por que não é nó | O movimento certo |
|------|------------------|-------------------|
| `const` / `var` (inclusive erros sentinela e instâncias de pacote) | o extractor indexa a declaração de tipo e função, não a de valor | `gofi show` no **símbolo que o referencia** — o tipo ou a função onde ele é usado — e leia o `arquivo:linha` |
| diretiva em comentário (`//gofi:context`) | comentário não é declaração; é lido como **atributo** de um nó, não como nó | pergunte pelo pacote/símbolo e leia o campo `contexto` da resposta |
| import, string literal, SQL, tag de struct, chave de config, rota montada em runtime | é texto dentro do corpo, não declaração | `gofi find --text "<literal>"` (ou `--regex`) — **declarando** que é fallback; cada linha já vem com o símbolo que a contém |

O erro a evitar aqui não é usar a busca literal nesses casos: é **pular para
ela direto**. Quase sempre existe um vizinho concreto que *é* nó e que devolve o
`arquivo:linha` que você queria, mais a vizinhança de brinde. Exemplos do mesmo
movimento: *onde mora o validator que este DTO usa?* → `gofi show <DTO>` (não
busca literal pela `var`); *quem consome esta constante de erro?* → `gofi show` no
service que a retorna; *este pacote é de que contexto?* → `gofi show <pacote>`
ou num símbolo dele.

## O modo do scan — valide antes de concluir

O grafo tem dois modos, e **eles não respondem a mesma pergunta**:

| Modo | Como resolve as chamadas | O que você pode afirmar |
|------|--------------------------|--------------------------|
| `fast` (padrão) | heurística sintática; chamada ambígua **não vira aresta** — é contada e reportada | "isto chama aquilo" (aresta presente é evidência) |
| `deep` | type-checker: cada chamada resolvida, implementações de interface visíveis | também "**nada mais** chama isto" (ausência de aresta vira evidência) |

**Onde ler o modo:** o campo `mode` de cada escopo no
`gofi_graph_index.json`, e o cabeçalho do `report.md` (*"Gerado por gofi index
(…, modo `fast`)"* + a linha do §Resumo com a contagem de ambíguas).

**Quem decide o modo:** `.gofi.yaml` → `graph: deep:`. `gofi init` escreve o
bloco com o valor que a ausência já significava — **`deep: false`, ou seja
`fast`**. Projeto antigo ainda pode não ter o bloco; ausência lê igual a `fast`.
Consequência prática, e é a armadilha:

- `gofi init` e `gofi index code` constroem **no modo configurado** — o build
  padrão é `fast`, a menos que o `.gofi.yaml` diga `graph: deep: true`. Nada
  reconstrói o grafo sozinho no commit; quem o atualiza é quem roda
  `gofi index code` (e `--fast` força o sintático numa execução, mesmo com
  `deep: true`).
- Logo, com o default (`deep: false`) **todo grafo que você encontra pronto é
  `fast`**, por mais recente que seja; e mesmo com `deep: true`, um build
  rodado com `--fast` deixa no disco um grafo `fast`. **Encontrar o grafo
  atualizado não é o mesmo que encontrá-lo exato.** O `mode` do índice é a
  resposta; o `.gofi.yaml` diz só o que o próximo build fará.
- Portanto, no default, **`deep` é sempre um pedido explícito** —
  `gofi index code --deep` rodado por quem está prestes a concluir algo que o
  grafo `fast` não sustenta. Quando isso é, na seção seguinte.
- Se o projeto quer exatidão por padrão nos builds, o lugar é o
  `.gofi.yaml` (`graph:` → `deep: true`). Sugira ao dev; a escolha é dele.

## Quando rodar `--deep` — e quando não

`deep` custa: exige que o projeto compile e paga o type-checker. Não é o modo de
trabalho, é o **modo de prova**. A regra que decide é uma só:

> **Você vai afirmar uma ausência?** Então precisa de `deep`. Vai apenas
> navegar? `fast` basta — e já está no disco.

**Rode `gofi index code --deep` antes de:**

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

O hook de pre-commit põe o grafo em dia **no commit** (`gofi index hooks`) — mas
o agent trabalha antes dele. O agent escreve
código, e um pipeline encadeado (`/gofi-full`: `eng → qa`) passa de fase sem
parar — então, sem ação explícita, o QA auditaria contra um mapa que **não
contém a implementação que acabou de ser feita**. O mesmo vale para você:
consultar código que acabou de mudar sem reconstruir devolve o mapa de antes.
`gofi index status` diz, sem construir, se o grafo ficou para trás.

Por isso: **ao fechar uma implementação, antes de atualizar memória e spec** —
e antes de consultar código que você acabou de mudar —, rode

```sh
gofi index code
```

O build é incremental por padrão: árvore que não mudou é pulada (`--full`
força tudo), então rodar é barato e rodar sempre é seguro. A próxima skill da cadeia lê um mapa atualizado.
Ele reconstrói **no modo configurado**. Se a entrega mexeu em artefato
compartilhado, ou a conclusão que você vai escrever é uma ausência, é aqui que
entra `--deep` — ver *Quando rodar `--deep`*.

## Limites (declare-os, não os esconda)

- **Extractor nativo só para Go, TypeScript e JavaScript.** Java, C#, Rust e
  demais exigem um extractor instalado (`gofi index install <lang>`). **Sem ele
  não há grafo** para aquela linguagem — caia no `gofi find --text`/`--regex`
  (que busca em todo arquivo do projeto, com ou sem grafo) e diga que caiu.
- **Só o extractor Go lê `//gofi:context`.** No grafo de TS/JS o campo
  `contexto` vem vazio; a ponte grafo→spec, ali, ainda é o nome da pasta.
- **`fast` é heurística sintática** e é o que `init` e `gofi index code`
  produzem enquanto o `.gofi.yaml` não disser o contrário — ver *O modo do
  scan*. Confira o `mode` antes de tirar conclusão de ausência.
- **`deep` não alcança TS/JS.** O extractor de superfície é sintático; front e
  mobile ficam em `fast` mesmo sob `--deep`. Ali, ausência nunca é prova.
- **Superfície não se consulta com `--lang`.** Front e mobile são escopos do
  índice principal; `gofi show` e `gofi find --in code` já procuram neles, sem
  flag. `--lang` é só flag de build (`gofi index code --lang <lang>`, extractor
  instalado).
- **Grafo é estrutura, não comportamento.** Ele não sabe de reflexão, injeção
  por string, handler registrado por tabela nem SQL. Nessas, a busca literal
  (`gofi find --text`/`--regex`) continua soberana.

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

- ❌ Ler `gofi_graph.json` (ou pedir para o usuário colar) — o `report.md`, o
  `gofi find --in code` e o `gofi show` existem exatamente para isso.
- ❌ Começar por busca literal (`gofi find --text`, `grep -r`) no módulo quando a pergunta é *"quem chama X"* e
  existe grafo da linguagem. Nem para **achar** o símbolo:
  `gofi find --in code "<termo> <termo>"` busca por nome parcial dentro do
  próprio índice.
- ❌ **Classificar o alvo de cabeça para pular o `show`** — decidir sozinho
  que "isto é `var`, o grafo não tem" e ir direto à busca literal. Mesmo quando a
  conclusão está certa, o caminho está errado: é assim que o reflexo de varrer
  o repositório volta, e a chamada que você economizou custava ~370 ms. Uma
  consulta ao índice primeiro, sempre.
- ❌ Cair na busca literal (`gofi find --text` ou `grep`) **sem dizer que caiu**. Fallback silencioso vira, na leitura
  de quem revisa, uma busca que parece ter sido feita no grafo.
- ❌ Concluir de um scan `fast` que **nada** quebra — ausência de aresta ali não
  é prova de ausência de uso. Sem checar o `mode`, você não sabe em que modo
  está o grafo que acabou de ler.
- ❌ Ler o `report.md` da raiz achando que ele cobre o front: cada escopo tem o
  seu, na pasta que o índice aponta.
- ❌ Fechar uma implementação sem `gofi index code` (a próxima fase
  herda um mapa desatualizado).
- ❌ Editar qualquer arquivo de `.gofi/index/code/` à mão — é artefato derivado, é
  sobrescrito no próximo build.
- ❌ Criar pacote de contexto sem `//gofi:context` (fica invisível à ponte
  grafo→spec).
