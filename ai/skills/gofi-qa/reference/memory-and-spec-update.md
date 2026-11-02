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

### 3. O veredicto — `gofi verdict write {contexto}`

Depois da memória (§1), grave o veredicto. É ele que fecha `status` e
`ressalvas` no frontmatter da memória — não os escreva à mão.

```sh
gofi verdict write {contexto} <<'JSON'
{"result": "reprovado", "findings": [
  {"severity": "major", "rule": "{contexto}/REQ-04.1", "symbol": "{pacote}.{Tipo}.{Método}",
   "file": "{arquivo}", "line": 88, "owner": "eng",
   "message": "cancelamento após envio não é recusado",
   "evidence": "specs/changes/{id}/deltas/{contexto}.md#REQ-04"}
]}
JSON
```

| Veredicto do laudo | `result` | Achados |
|---|---|---|
| ✅ Aprovado | `aprovado` | nenhum blocker, major ou minor (suggestion pode) |
| ⚠️ Aprovado com ressalvas | `aprovado` | majors e/ou minors, nenhum blocker |
| ❌ Reprovado | `reprovado` | ao menos um blocker |

- **`owner`**: `eng` quando o código não cumpre a mudança; `spec` quando o
  contrato está errado ou incompleto (cenário que não testa o requisito,
  requisito que falta, design contraditório); `pd` quando falta decisão de
  negócio. É por ele que o loop volta à fase certa.
- **`rule`** é a âncora do achado: o cenário (`{ctx}/REQ-NN.n`), o princípio do
  projeto ou a regra do checklist. Achado sem âncora é SUGGESTION.
- **`symbol`** é o nó do grafo (`gofi show`): com ele, o mesmo defeito é
  reconhecido na próxima auditoria mesmo que as linhas mudem.
- O comando recusa o que não fecha (blocker num aprovado, reprovado sem nada
  que reprove) — corrija o JSON, não o laudo.

O frontmatter que ele deixa na memória:

```yaml
status: aprovado    # ou: reprovado (algum BLOCKER)
ressalvas: 0        # MAJOR + MINOR pendentes; 0 só no ✅ Aprovado
atualizado: {data}
```

`⚠️ Aprovado com ressalvas` deixa `status: aprovado` com `ressalvas` maior que
zero — e **não** encerra o ciclo: o loop (`gofi run`, como quer que tenha sido iniciado) só
aceita `aprovado` com `ressalvas: 0` e reabre a implementação nos demais casos.

> O índice global (panorama de todos os contextos) é gerado por `/gofi-status`
> lendo esse frontmatter. Nenhuma tabela por-contexto no `project.md`.

Se a auditoria revelou um padrão novo digno de ser preservado, registre em
`.claude/sdk/<lang>/knowledge/<topico>.md` e propague para `gofi-eng` evitar
reincidência.
