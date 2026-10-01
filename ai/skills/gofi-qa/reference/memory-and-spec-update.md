# Atualização de memória e spec

> **Como gravar:** `gofi memory write {contexto}` com o arquivo inteiro pela entrada
> padrão — não `Edit`/`Write`. O Claude Code trata `.claude/` como sensível e pede
> aprovação a cada edição ali; numa fase conduzida ninguém aprova, e a memória não
> fecha. O comando confere o frontmatter (`contexto`, `versao`, `status`, `keywords`)
> antes de gravar. Protocolo: `.claude/expertise/harness-protocols/memory.md`.

Após a auditoria:

### 1. `.claude/memory/contexts/{contexto}.md`

Pós-baseline v1.0, a auditoria **não** apende entrada datada. Ao concluir:
1. **Refresh do `## Estado atual`** — reflita o veredicto e as pendências abertas/fechadas (o score vira estado, não changelog).
2. **Adicione uma linha** ao `## Histórico de versões` + atualize `atualizado` no frontmatter: `| v{X.Y} | {data} | QA {aprovado/reprovado} — {resumo} |`.

Protocolo em `.claude/expertise/harness-protocols/memory.md`.

### 2. `specs/{contexto}/sdd-{contexto}.md`

**Frontmatter: não toque.** O `status` da spec é da **aprovação da pessoa**
(gravado na parada de revisão); o veredito da auditoria vai para a memória do
contexto (§3 abaixo), não para a spec.

**Versão: você fecha o fluxo.** Ninguém bumpou no meio dele. Ao **aprovar**, se o
fluxo passou no teste — mudou comportamento que já roda em produção —, suba a
`versao` da spec (e a do PRD, se ele mudou) **uma vez** e escreva a linha do
Histórico. Se a solução ainda não foi a produção, ou se nada muda no
comportamento, nada sobe. Reprovou: nada sobe. Política em
`.claude/expertise/harness-protocols/document-versioning.md` §Quando o bump acontece.

**Nada de marca de auditoria na spec** — nem ✅, nem data, nem nome de agent:
proveniência vive no git e o resultado, na memória.

**Histórico de versões:** uma linha por versão — só a do bump acima, quando
houver. A auditoria em si não cria linha. Se a auditoria levou a
uma mudança de comportamento em código **já em produção**, aí sim há bump — e
a linha descreve a mudança, sem citar agent nem pessoa (proveniência vive no
git; ver `.claude/expertise/harness-protocols/rag-retrieval.md`).

**Contratos §0.1:** se a auditoria revelou drift entre spec e implementação,
corrigir a spec (a spec é a verdade pós-QA).

**Estrutura §8:** se arquivos foram adicionados durante a implementação
(ex: `_test.go`), atualizar.

### 3. `.claude/memory/contexts/{contexto}.md` — frontmatter

Atualizar o `status` no frontmatter (sem tocar `project.md`):

```yaml
status: aprovado    # ou: reprovado
atualizado: {data}
```

> O índice global (panorama de todos os contextos) é gerado por `/gofi-status`
> lendo esse frontmatter. Nenhuma tabela por-contexto no `project.md`.

Se a auditoria revelou um padrão novo digno de ser preservado, registre em
`.claude/sdk/<lang>/knowledge/<topico>.md` e propague para `gofi-eng` evitar
reincidência.
