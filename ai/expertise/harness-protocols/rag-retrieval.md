# Protocolo de Retrieval (RAG) — ler e escrever corpora gastando poucos tokens

> **Cross-agent, portável.** Vale, sem mudar uma palavra, para qualquer projeto
> com o mesmo SDK. Governa como **todo** agent lê e escreve os corpora do
> projeto (`specs/`, `prd/`, `institutional/{project}/`, `memory/contexts/`).
> Objetivo único: **descobrir o mínimo, ler o mínimo, escrever o mínimo** — o
> corpus cresce sem que o custo de token por leitura cresça junto.


## Procurar documento é `gofi find`, não carregar índice

O corpus tem uma camada de consulta — e é a mesma do código: `gofi find` busca
documentos e símbolos de uma vez (`--in` estreita as áreas; `--in code` fica só
no código). **Pergunte; não carregue.**

```bash
gofi find "ordem de delete chave estrangeira"
gofi show table:core_company     # quem documenta + quem implementa
gofi show sdd-churn.md      # backlinks e saídas
gofi show ctx:pricing         # a árvore do contexto
```

Devolve candidatos já com **contexto, §seção e faixa de linhas** — leia direto
com `Read(offset, limit)`, não o arquivo inteiro.

**A escada, em ordem:**

1. `gofi find` — sempre o primeiro movimento.
2. Vazio? Reformule com o **termo técnico** (o corpus costuma nomear em inglês:
   `retry`, `logout`, `buybox`, `purge`). Se o par que faltou é entre a língua da
   pergunta e a do identificador, **registre-o** em
   `.claude/lexicon/sinonimos.md` — é assim que a busca melhora com uso em vez
   de apodrecer.
3. Só então os `INDEX.md`: o roteador de contextos e o shard. Use quando quer
   **navegar** um contexto, não quando quer **achar** um assunto.
4. `gofi find --text "<literal>"` (ou `--regex`, `-i`; `--in specs,prd` estreita)
   é o fallback declarado para texto exato, como no grafo de código — cada linha
   vem sob a §seção que a contém, com a faixa de linhas. `grep` só se o `gofi`
   não alcançar (pasta fora do projeto).

**O corpus é um grafo, não uma pasta.** A entidade é a dobradiça: a tabela do
schema é o único símbolo que existe nos documentos **e** no código, então *"quem
documenta isto?"* e *"quem implementa isto?"* são a mesma pergunta. Um pacote
marcado `//gofi:context {contexto}` fecha o mesmo elo pelo lado do código, e é o
único caminho para um contexto que não possui tabela própria.

Antes de editar um documento, `gofi show <doc>` mostra quem depende
dele — é a análise de impacto barata.

Os artefatos são **derivados**. O índice de documentos (`.gofi/index/docs/`,
fora do git) não pede nada: `gofi find`/`gofi show` o reconstroem sozinhos
quando falta ou quando algum documento mudou — a consulta seguinte a uma edição
já a enxerga. Os `INDEX.md` versionados, esses sim: `gofi index docs` após
criar, renomear ou mexer em frontmatter, e `gofi index check` antes de
commitar. `gofi index check` mede se uma mudança no
corpus melhorou ou piorou a busca — sem isso é mudança torcendo para dar certo.

## Princípio

Cada corpus é um **RAG**: um `INDEX.md` (manifesto sempre carregável, derivado
do frontmatter) + N documentos, cada um com **frontmatter + `keywords`**. Você
**nunca** varre a pasta e **nunca** lê um doc inteiro por reflexo. O INDEX diz
*qual* doc cobre o tema; o frontmatter confirma; o `gofi show` localiza a §seção; o
`Read` traz **só** aquela seção.

## Os corpora e seus índices

| Corpus | INDEX | Regenerado por |
|--------|-------|----------------|
| Specs (SDD) | `specs/INDEX.md` | `gofi index docs` |
| PRDs | `prd/INDEX.md` | `gofi index docs` |
| Institucional (negócio) | `.claude/institutional/{project.name}/INDEX.md` | manual (um fato = uma linha) |
| Estado por-contexto | *(sem índice commitado)* | `/gofi-status` lê o frontmatter dos `contexts/*.md` sob demanda |

## Leitura — o funil de 4 passos (sempre nesta ordem)

1. **INDEX → descoberta.** Carregue **só** o `INDEX.md` do corpus. Case o tema
   da tarefa contra a coluna **Keywords** (ou **Tópicos/Carregar quando**, no
   institucional). Selecione **apenas** os docs que casam — em dúvida entre 1–2
   próximos, pegue o de maior match e expanda só se faltar contexto.
2. **Frontmatter → confirmação.** Leia **só o frontmatter** do doc-alvo
   (`Read` das ~12 primeiras linhas, ou `sed -n '1,/^---$/p'`). Confirme
   `contexto`, `status`, `versao`, `keywords`. Se não casar, volte ao passo 1.
3. **`gofi show` → localização.** `gofi show <doc>` lista as §seções com a
   faixa de linhas de cada uma — é dali que sai o `offset`/`limit` da
   §relevante (`grep -n '^## '` não é mais necessário). Nunca presuma o offset.
4. **`Read` → só a seção.** `Read` (com `offset`/`limit`) **apenas** da(s)
   §seção(ões) que a tarefa exige. Não leia vizinhas "por garantia".

> **Regra de ouro:** ler o doc inteiro só se a tarefa genuinamente exigir o doc
> inteiro (raro). O default é seção. O INDEX inteiro cabe em poucos tokens; um
> doc inteiro não.

## Escrita — deixar o corpus indexável e barato

Ao **criar ou editar** um doc de qualquer corpus:

- **Frontmatter + `keywords` obrigatórios.** Base nos templates
  (`.claude/templates/`). `keywords` = **8–14 termos kebab-case de busca do
  domínio** — são eles que o INDEX expõe e o próximo leitor casa. Escolha
  termos que alguém buscaria, não sinônimos genéricos.
- **Todo doc nasce `versao: "1.0"` e fica em `1.0`** enquanto a solução descrita
  não estiver em produção. A versão conta **estados de produção**, não edições:
  refinar, reescrever, trocar decisão, corrigir erro, pôr nota de superseded —
  nada disso bumpa. `atualizado` muda sempre; `status` diz a fase. Política em
  `document-versioning.md`.
- **Zero proveniência.** **Sem** `**Autor:**`/`**Data:**`/`**Versão:**` no
  corpo, **sem** `## Rastreabilidade`, **sem** nome de agent/pessoa, **sem**
  journal datado. Quem/quando vive no **git**; a versão vive no **frontmatter**.
- **Histórico de 1 linha.** Uma linha por **versão** do doc (baseline
  consolidada), não um changelog multi-versão. Edição que não bumpa **não**
  acrescenta linha — doc em `1.0` tem uma linha só, por mais que tenha sido
  reescrito no caminho.
- **Um fato = um lugar.** Não duplique entre docs/chunks; cruze com link
  relativo.
- **Regenere o INDEX** ao criar, renomear ou mudar frontmatter:
  `gofi index docs`. Nunca edite a tabela do
  INDEX à mão — ela é derivada. (Institucional: registre a linha manualmente no
  seu `INDEX.md`.)

## Anti-padrões (não faça)

- ❌ `Read` da pasta inteira / `cat specs/**` para "achar" algo — use o INDEX.
- ❌ Ler o doc inteiro quando a tarefa toca uma seção.
- ❌ Editar a tabela do `INDEX.md` na mão (ela drena na próxima regeneração).
- ❌ Criar doc sem `keywords` (fica invisível ao retrieval).
- ❌ Colocar proveniência/rastro de agent no corpo do doc.
