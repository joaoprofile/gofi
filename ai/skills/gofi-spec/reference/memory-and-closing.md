# Versionamento, memória e fechamento da spec

## Versionamento — a versão conta estados de produção

Política completa em `.claude/expertise/harness-protocols/document-versioning.md` — **leia
antes de tocar o frontmatter**. O essencial:

- Toda spec **nasce em `1.0` e permanece em `1.0`** enquanto a solução descrita
  não estiver em produção. Refinar, reescrever, trocar decisão de arquitetura,
  acrescentar ADR, corrigir erro factual: nada disso bumpa, e nada disso entra
  no histórico. Código mergeado mas não deployado **ainda é 1.0**.
- A versão sobe **uma vez por estado de produção**: a solução já está em
  produção **e** a spec passa a descrever um comportamento diferente **que irá**
  para produção.
- **Teste:** *esta edição vai fazer alguém mudar código que já está rodando em
  produção?* Não → não bumpa.
- Reformulação profunda vira **spec nova** (sufixo `-v2`), que nasce em 1.0. A
  antiga não bumpa por ganhar nota de superseded, e continua sendo o **as-built**
  até a nova entrar em produção.
- Cross-spec: bumpa só o lado cujo comportamento **em produção** muda.
- **Nunca bumpe ao editar no meio de um fluxo.** Mesmo quando a mudança passa
  no teste, a versão sobe uma vez, quando o fluxo fecha com a auditoria aprovada
  — e quem sobe é o `gofi-qa` (lei comum 4).
- `atualizado` muda **sempre**; `status` diz a fase.

## Atualização de memória — **OBRIGATÓRIA ao final de toda spec gerada ou bumpada (com consumidor downstream)**

> **Como gravar:** `gofi memory write {contexto}` com o arquivo inteiro pela entrada
> padrão — não `Edit`/`Write`. O Claude Code trata `.claude/` como sensível e pede
> aprovação a cada edição ali; numa fase conduzida ninguém aprova, e a memória não
> fecha. O comando confere o frontmatter (`contexto`, `versao`, `status`, `keywords`)
> antes de gravar. Protocolo: `.claude/expertise/harness-protocols/memory.md`.

> **Regra:** sempre que uma spec for **criada** ou **alterada**, atualize a memória do contexto no mesmo turno — independentemente de ter havido bump de versão. Não tratar isso como passo opcional. Status da spec deve sempre estar presente e refletir a versão mais recente.
> Idem para bumps cross-spec: se a spec do contexto X foi bumpada por decisão tomada no contexto Y, **ambos** os `contexts/{X}.md` e `contexts/{Y}.md` precisam ser atualizados.
> **Exceção:** ajustes em spec cuja solução ainda não está em produção não exigem bump — ver "Versionamento" acima. A **memória do contexto** segue regra própria (`memory-protocol.md`) e continua sendo atualizada em toda fase.

### 1. `.claude/memory/contexts/{contexto}.md` — frontmatter (estado por-contexto)

> Estado de contexto **não** vai mais para `project.md`. Atualize o **frontmatter**
> do arquivo do próprio contexto (um arquivo por contexto = sem conflito de git).
> O índice global é gerado por `/gofi-status` lendo esse frontmatter.

Atualizar/criar o frontmatter:

```yaml
---
contexto: {contexto}
servicos: [{serviço}, ...]
status: spec
versao: "1.0"
versao_prd: "{X.Y}" | n/a
versao_spec: "{X.Y}"
prd: prd/{contexto}/prd-{contexto}.md | n/a
spec: specs/{contexto}/sdd-{contexto}.md
diretorio: services/domain/{contexto}/
atualizado: {data}
---
```

Em bumps cross-spec, atualizar o frontmatter de **cada** contexto afetado (`versao_spec` + `atualizado`).

**`project.md` só é tocado** se nascer um **serviço/binário novo** (linha na tabela "Serviços").

### 2. `.claude/memory/contexts/{contexto}.md` (handoff por contexto)

**Sempre criar** se não existir. **Sempre atualizar** se já existir e a spec mudou (mesmo que mínima — bump de versão, ADR ajustada, regra cross-context).

Conteúdo mínimo:

- **Cabeçalho:**
  - `Status:` valor mais recente (`spec criada` | `spec criada **v{X.Y}**` | `em implementação` | `implementado` | `em QA`)
  - `Próximo passo:` qual agente roda em seguida (ex.: `/gofi-eng para implementar a partir de specs/{contexto}/sdd-{contexto}.md`)
  - `PRD origem:` caminho do PRD
- **Manifesto do serviço** (nome, module, banco, porta, prefixo, redis, auth)
- **Resumo do contexto** (3–5 linhas — o que faz, principais decisões)
- **Decisões de arquitetura** (lista dos ADRs por título)
- **Pontos de atenção para `gofi-eng`** (regras não-óbvias, gotchas, cross-context calls esperadas)
- **Variáveis de ambiente adicionais** (lista ou "nenhuma")
- **`## Histórico de versões`** — uma linha por **versão** (não por evento):
  `| v{X.Y} | {data} | spec v{X.Y} criada/bump — {motivo curto} |`.

> **RAG (pós-baseline v1.0):** a memória é consolidada, não jornal. O
> manifesto/resumo/decisões viram **as-built no `## Estado atual`** da cabeça +
> uma linha no `## Histórico de versões`. **Não** apende `## gofi-...: {data}`.
> O detalhe cronológico vive no git/spec. Modelo completo em
> `.claude/expertise/harness-protocols/memory.md`.

### Checklist de fechamento (rode mentalmente antes de devolver "spec gerada")

- [ ] `specs/{contexto}/sdd-{contexto}.md` foi criado/atualizado
- [ ] `specs/{outros-contextos}/sdd-*.md` afetados foram bumpados (se houve cross-spec impact)
- [ ] `.claude/memory/contexts/{contexto}.md` frontmatter (`status: spec`, `versao_spec`, `atualizado`) + histórico refletem o estado atual
- [ ] `.claude/memory/project.md` só tocado se nasceu serviço/binário novo
- [ ] `.claude/memory/contexts/{outros-afetados}.md` foram atualizados (cross-spec)
- [ ] Output final cita os arquivos modificados

Layout completo do contexto em `.claude/expertise/harness-protocols/memory.md`.

## Protocolo de aprendizado contínuo

Ver `.claude/expertise/harness-protocols/learning.md`.

> **Regra absoluta — knowledge é domínio-neutro.** Arquivos sob
> `.claude/knowledge/` e `.claude/sdk/<lang>/` descrevem **padrão técnico**
> (como elicitar, como estruturar a spec). **Nunca** cite nomes de
> entidades do produto, roles concretos, module paths reais, endpoints
> do produto, ou refs a versões de spec específicas. Use placeholders
> (`{contexto}`, `<module>`, `RoleA`, `entity`). Conteúdo de domínio
> (RNs, entidades, ADRs do projeto) vive em `specs/{contexto}/` e
> `.claude/memory/`, **nunca** em knowledge. Teste antes de escrever:
> *"este texto serviria, sem alteração, a um projeto totalmente diferente
> que use o mesmo SDK?"* — se não serviria, é spec ou memória.

Em particular:
- Correções nas perguntas de elicitação → atualize esta skill
- Mudanças no formato da spec → atualize `templates/sdd-template.md`
- Lições de modelagem cross-language → atualize `knowledge/shared/` (genéricas, sem domínio)
- Lições language-specific → atualize `.claude/sdk/<lang>/knowledge/` (genéricas, sem domínio)
- Generalize qualquer trecho domínio-específico antes de salvar em knowledge
