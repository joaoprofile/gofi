# Protocolo de Retrieval (RAG) — ler e escrever corpora gastando poucos tokens

> **Cross-agent, portável.** Vale, sem mudar uma palavra, para qualquer projeto
> com o mesmo SDK. Governa como **todo** agent lê e escreve os corpora do
> projeto (`specs/`, `prd/`, `institutional/{project}/`, `memory/contexts/`).
> Objetivo único: **descobrir o mínimo, ler o mínimo, escrever o mínimo** — o
> corpus cresce sem que o custo de token por leitura cresça junto.

## Princípio

Cada corpus é um **RAG**: um `INDEX.md` (manifesto sempre carregável, derivado
do frontmatter) + N documentos, cada um com **frontmatter + `keywords`**. Você
**nunca** varre a pasta e **nunca** lê um doc inteiro por reflexo. O INDEX diz
*qual* doc cobre o tema; o frontmatter confirma; o `grep` localiza a §seção; o
`Read` traz **só** aquela seção.

## Os corpora e seus índices

| Corpus | INDEX | Regenerado por |
|--------|-------|----------------|
| Specs (SDD) | `specs/INDEX.md` | `bash .claude/scripts/gen-index.sh specs` |
| PRDs | `prd/INDEX.md` | `bash .claude/scripts/gen-index.sh prd` |
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
3. **`grep` → localização.** `grep -n '^## ' <doc>` para listar as seções e
   achar o número da §relevante. Nunca presuma o offset.
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
- **Todo doc tem `versao: "1.0"` + `status` + `keywords`.**
- **Zero proveniência.** **Sem** `**Autor:**`/`**Data:**`/`**Versão:**` no
  corpo, **sem** `## Rastreabilidade`, **sem** nome de agent/pessoa, **sem**
  journal datado. Quem/quando vive no **git**; a versão vive no **frontmatter**.
- **Histórico de 1 linha.** Uma linha por versão do doc (baseline consolidada),
  não um changelog multi-versão.
- **Um fato = um lugar.** Não duplique entre docs/chunks; cruze com link
  relativo.
- **Regenere o INDEX** ao criar, renomear ou mudar frontmatter:
  `bash .claude/scripts/gen-index.sh {specs|prd}`. Nunca edite a tabela do
  INDEX à mão — ela é derivada. (Institucional: registre a linha manualmente no
  seu `INDEX.md`.)

## Anti-padrões (não faça)

- ❌ `Read` da pasta inteira / `cat specs/**` para "achar" algo — use o INDEX.
- ❌ Ler o doc inteiro quando a tarefa toca uma seção.
- ❌ Editar a tabela do `INDEX.md` na mão (ela drena na próxima regeneração).
- ❌ Criar doc sem `keywords` (fica invisível ao retrieval).
- ❌ Colocar proveniência/rastro de agent no corpo do doc.
