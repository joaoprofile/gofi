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

## Os artefatos e seu custo

Todos vivem em `.gofi/graph/` (Go na raiz; outra linguagem em
`.gofi/graph/{lang}/`). Gerados por `gofi graph build` — **nunca** editados à mão.

| Artefato | O que responde | Custo | Ler? |
|----------|----------------|-------|------|
| `gofi_graph_index.json` | Quais escopos existem (backend, front, mobile, SDK), linguagem, framework, modo do scan, tamanho | dezenas de linhas | **Sempre** — é o ponto de entrada |
| `gofi_graph_report.md` | Resumo, pacotes, pontos centrais, comunidades, ciclos, conexões inesperadas | ~1 tela (limitado por construção) | **Quase sempre** — o panorama |
| `gofi graph explain <símbolo>` | Origem, assinatura, doc, o que chama, quem chama, tipos que toca, **contexto** | ~30 linhas por chamada | **Sob demanda**, um símbolo por vez |
| `gofi graph explain <A> --to <B>` | O caminho de A até B, aresta por aresta | poucas linhas | Quando a pergunta é *"como A alcança B"* |
| `gofi_graph.json` | O grafo cru | **milhares de linhas** | ❌ **Nunca** |
| `gofi_graph.html` | Visualização para humano | — | ❌ Nunca (é para o usuário, via `gofi graph open`) |

## Leitura — a escada de 3 degraus (sempre nesta ordem)

1. **`index.json` → orientação.** Descubra os escopos e a linguagem de cada um
   antes de qualquer pergunta. Ele também diz o **modo** do último scan
   (`fast`/`deep`) — o que determina se você pode confiar nas arestas de
   chamada (ver *Limites* abaixo). Se não existir, o último build foi feito com
   caminho explícito (`gofi graph build ./algum/path`), que não escreve índice —
   siga pelo `report.md`.
2. **`report.md` → panorama.** Leia o relatório do escopo relevante. Ele já
   entrega os pacotes, os pontos centrais (o que muita coisa depende), os ciclos
   e as **conexões inesperadas** — que é exatamente onde uma mudança costuma
   quebrar algo distante. Não precisa de `grep` para isso.
3. **`explain` → precisão.** Para cada símbolo que a tarefa realmente toca,
   uma chamada de `gofi graph explain`. É ela que responde *quem chama* — a
   pergunta que antes exigia varrer o módulo inteiro com `grep`.

Só então abra os arquivos — **apenas** os que os passos 1–3 apontaram.

> **Regra de ouro:** `grep -r` pelo módulo inteiro é o **fallback**, não o
> primeiro movimento. Use quando não há grafo para a linguagem, quando o alvo
> não é um símbolo (string, chave de config, SQL) ou quando o grafo está em
> modo `fast` e a resposta precisa ser exata.

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

## Limites (declare-os, não os esconda)

- **Extractor nativo só para Go, TypeScript e JavaScript.** Java, C#, Rust e
  demais exigem um extractor instalado (`gofi graph install <lang>`). **Sem ele
  não há grafo** para aquela linguagem — caia no `grep` e diga que caiu.
- **Só o extractor Go lê `//gofi:context`.** No grafo de TS/JS o campo
  `contexto` vem vazio; a ponte grafo→spec, ali, ainda é o nome da pasta.
- **`fast` é heurística sintática.** No modo padrão, chamadas ambíguas **não
  viram aresta** — são contadas e reportadas. Para decidir *"isso quebra ou
  não"* com o grafo, exija `deep`: `gofi graph build --deep` resolve cada
  chamada pelo type-checker e revela implementações de interface (custa exigir
  que o projeto compile).
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
  existe grafo da linguagem.
- ❌ Confiar em aresta de chamada de um scan `fast` para afirmar que **nada**
  quebra — ausência de aresta ali não é prova de ausência de uso.
- ❌ Fechar uma implementação sem `gofi graph build --update` (a próxima fase
  herda um mapa desatualizado).
- ❌ Editar qualquer arquivo de `.gofi/graph/` à mão — é artefato derivado, é
  sobrescrito no próximo build.
- ❌ Criar pacote de contexto sem `//gofi:context` (fica invisível à ponte
  grafo→spec).
