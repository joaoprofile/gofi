# Fechamento do gofi-eng — memória, spec e output

## Atualização de memória ao concluir

> **Como gravar:** `gofi memory write {contexto}` com o arquivo inteiro pela entrada
> padrão — não `Edit`/`Write`. O Claude Code trata `.claude/` como sensível e pede
> aprovação a cada edição ali; numa fase conduzida ninguém aprova, e a memória não
> fecha. O comando confere o frontmatter (`contexto`, `versao`, `status`, `keywords`)
> antes de gravar. Protocolo: `.claude/expertise/harness-protocols/memory.md`.

**Primeiro, atualize o grafo.** As fases seguintes (`gofi-qa`, `gofi-doc`) e a
próxima tarefa consultam o mapa antes de qualquer commit — e só no commit o
hook de pre-commit o atualiza. Sem isto o QA auditaria um
mapa que não contém a implementação recém-escrita:

```sh
gofi index code
```

O build é incremental — árvore sem fonte alterado é pulada (`--full` força) —,
então rodar sempre é barato; `gofi index status` diz se o grafo ficou para trás.
O build padrão é `fast` a menos que o `.gofi.yaml` declare `graph.deep: true`,
então o grafo que você encontra pronto costuma ser sintático. Se a
entrega mexeu em artefato compartilhado, ou a análise de impacto vai afirmar que
**nada mais** usa algo, rode `gofi index code --deep` aqui — os gatilhos estão
listados em *Quando rodar `--deep`* do protocolo
(`.claude/expertise/harness-protocols/graph-retrieval.md`).

Depois, aplicar **todas** as três:

### 1. `.claude/memory/contexts/{contexto}.md`

Pós-baseline v1.0, a memória **não** acumula entradas datadas. Ao concluir a implementação:
1. **Refresh do `## Estado atual`** da cabeça — reescreva o que mudou (arquivos criados + decisões não-óbvias entram como as-built vigente, não como changelog).
2. **Adicione uma linha** ao `## Histórico de versões` e bumpe `versao` no frontmatter (1.0→1.1 melhoria, →2.0 estrutural): `| v1.1 | {data} | {resumo curto} |`.
3. O detalhe/porquê vive no commit e na spec — não na memória.

Protocolo completo em `.claude/expertise/harness-protocols/memory.md`.

### 2. `.claude/memory/contexts/{contexto}.md` — frontmatter

Atualizar o frontmatter para refletir a implementação concluída (sem tocar `project.md`):

```yaml
status: implementado      # em_implementacao enquanto não concluiu
versao_spec: "{X.Y}"      # ponteiro p/ a versão do doc; spec só bumpa quando muda
                          # comportamento JÁ em produção (document-versioning.md)
atualizado: {data}
```

> O índice global é gerado por `/gofi-status` — não existe mais "mover entre tabelas"
> no `project.md`. **`project.md` só é tocado** se nasceu um **serviço/binário novo**
> (linha na tabela "Serviços").

### 3. A mudança e a spec viva — não se editam

A mudança aprovada e a spec viva são seladas (decisão 0003): editá-las desfaz
a aprovação. A implementação não as corrige para casar com o código.

- **Divergiu de contrato, dado ou Mapa de requisitos?** É decisão: pare e
  pergunte (conduzido pelo gofi, no arquivo de `.gofi/elicit/`). O `/gofi-spec`
  registra a resposta na mudança e ela volta a pedir `gofi change approve`.
- **Arquivo a mais, helper, teste** — é o como, e é seu: o design não fixa a
  estrutura de arquivos.
- **`tasks.md`** é derivado: marque `[x]` nas tarefas concluídas.
- A spec viva só muda quando a mudança é aceita e arquivada
  (`gofi change archive`), e a versão sobe aí.

## Output esperado

```
### Arquivos criados
- {pathContext}model/entity.go
- {pathContext}model/dto.go
- {pathContext}service/errors.go
- {pathContext}service/{contexto}_service.go
- {pathContext}service/{contexto}_service_test.go
- {pathContext}repository/{contexto}_repository.go
- {pathContext}adapter/iam_adapter.go    (se aplicável)
- {pathContext}handler/middleware.go     (se aplicável)
- {pathContext}handler/{contexto}_handler.go
- {pathContext}handler/{contexto}_handler_test.go
- {pathContext}handler/auth_handler.go   (se aplicável)
- {pathService}/.migrations/{N}_{contexto}.up.sql    ← par obrigatório
- {pathService}/.migrations/{N}_{contexto}.down.sql  ← par obrigatório (DROP em ordem inversa, com IF EXISTS)

### Decisões
- [ADR inline quando relevante]

### Próximos passos
- Executar migration
- Configurar variáveis de ambiente (se houver fora do padrão gofi)
- Executar gofi-qa
```
