# Atualização de memória ao fechar a auditoria

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

### 2. A spec viva e a mudança — não se editam

A mudança aprovada e a spec viva são seladas (decisão 0003): editá-las desfaz
a aprovação, e a auditoria é independente justamente porque não mexe no que
audita.

- **Drift** entre o código e a mudança (contrato, dado, erro, Mapa de
  requisitos) é achado com severidade — nunca uma correção no documento.
- **Lacuna do contrato** (cenário que não testa o requisito, requisito que
  falta, contradição entre dois REQ) é achado também, marcado como do
  contrato: quem corrige é o `/gofi-spec`, numa mudança.
- **Versão:** a da spec viva sobe sozinha quando a mudança aceita é arquivada
  (`gofi change archive`). A auditoria não bumpa nada.
- **Nada de marca de auditoria** em documento algum — nem ✅, nem data, nem
  nome de agent: proveniência vive no git e o resultado, na memória.

### 3. `.claude/memory/contexts/{contexto}.md` — frontmatter

Atualizar `status` e `ressalvas` no frontmatter (sem tocar `project.md`):

```yaml
status: aprovado    # ou: reprovado (algum BLOCKER)
ressalvas: 0        # MAJOR + MINOR pendentes; 0 só no ✅ Aprovado
atualizado: {data}
```

`⚠️ Aprovado com ressalvas` grava `status: aprovado` com `ressalvas` maior que
zero — e **não** encerra o ciclo: o condutor (CLI, painel e `/gofi-full`) só
aceita `aprovado` com `ressalvas: 0` e reabre a implementação nos demais casos.
SUGGESTION não entra na contagem.

> O índice global (panorama de todos os contextos) é gerado por `/gofi-status`
> lendo esse frontmatter. Nenhuma tabela por-contexto no `project.md`.

Se a auditoria revelou um padrão novo digno de ser preservado, registre em
`.claude/sdk/<lang>/knowledge/<topico>.md` e propague para `gofi-eng` evitar
reincidência.
