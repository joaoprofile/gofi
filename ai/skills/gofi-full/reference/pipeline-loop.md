# Loop contínuo — gates, roteamento e guarda

## Procedimento — o loop contínuo

Mantenha um **estado de execução** (use `TodoWrite` para tornar visível): fase
atual, número de idas-e-voltas por fase, e o veredicto da última fase.

```
fase ← ponto de entrada
loop:
  1. INVOCAR o agente da `fase` (via Skill tool: /gofi-pd, /gofi-spec, /gofi-eng, /gofi-qa).
     - Deixe o agente fazer sua pré-execução, suas PERGUNTAS de discovery/decisão,
       e gravar o artefato + frontmatter. Não interfira no método dele.
  2. LER o veredicto da fase (artefato + frontmatter atualizado).
  3. GATE — a fase passou? (critérios por fase abaixo)
       - SIM  → avança para a próxima fase na sequência. Se a fase era gofi-qa
                e passou limpo → FIM (entrega aprovada).
       - NÃO  → CLASSIFIQUE a causa-raiz e VOLTE para a fase responsável
                (roteamento abaixo). Passe ao agente anterior o laudo/lacuna
                concreta a corrigir. Depois de corrigir, **re-avança** pela
                sequência (não pula fases: spec corrigida → eng → qa de novo).
  4. GUARDA DE LOOP — ver abaixo. Se exceder, ESCALE ao usuário.
```

### Gate por fase (quando a fase "passou")

| Fase | Passou quando | Reprova quando |
|------|---------------|----------------|
| `gofi-pd`  | PRD gerado, sem ambiguidade bloqueante de escopo; frontmatter `status: prd` | discovery incompleto a ponto de impedir a spec |
| `gofi-spec`| spec SDD completa e internamente consistente; `status: spec` | spec não fecha por **lacuna de negócio** no PRD |
| `gofi-eng` | implementação compila e testes passam (`build`+`test` verdes), **todo bug fix/melhoria vem com teste de regressão** e o **grafo foi reconstruído** (`gofi index code`); `status: implementado` | spec **ambígua/contraditória**, impossível implementar como especificado, **ou fix/melhoria sem teste de regressão** |
| `gofi-qa`  | veredicto **✅ Aprovado** com **0 blockers, 0 majors e sem ressalvas** | qualquer blocker/major, **⚠️ com ressalvas**, ou ❌ reprovado |

> **Índice de documentos entre fases.** Cada fase escreve documento que a
> seguinte precisa achar: o PRD do `gofi-pd`, a spec do `gofi-spec`. O
> índice de documentos se reconstrói sozinho — `gofi find`/`gofi show` o
> refazem quando algum documento mudou —, então a fase seguinte já enxerga o que
> acabou de ser escrito. O que não se refaz sozinho são os `INDEX.md`
> versionados: rode `gofi index docs` ao fim de cada fase que criou ou editou
> PRD/spec. Rode também `gofi index check`: faceta fora do léxico é barata de
> corrigir na hora e cara de descobrir três fases depois.
>
> **Grafo entre fases.** O hook de pre-commit só atualiza o grafo no commit, e
> as fases correm antes dele. Sem `gofi index code` (incremental,
> barato) ao fim do `gofi-eng`, o `gofi-qa` auditaria um mapa sem a
> implementação recém escrita. `gofi index status` diz se o grafo ficou para
> trás; se a fase anterior não rodou, rode você antes de invocar o QA.
>
> **O modo importa tanto quanto o frescor.** `gofi index code` reconstrói no
> modo do `.gofi.yaml` (`graph: deep:`), que **por padrão é `fast`** — e em `fast` a ausência de aresta não prova ausência
> de uso. Quando a entrega mexeu em artefato compartilhado (struct de `model/`,
> interface, enum, coluna), ou quando o laudo vai afirmar uma ausência, o gate do
> `gofi-eng` exige `gofi index code --deep`, senão a análise de impacto e o
> laudo do QA se apoiam numa negativa que o grafo não sustenta. Os gatilhos estão
> listados em *Quando rodar `--deep`*.
> Protocolo: `.claude/expertise/harness-protocols/graph-retrieval.md`.

> **Minors/suggestions do QA:** o usuário pediu "sem nenhuma ressalva". Trate
> minors como itens a corrigir no mesmo passe de `gofi-eng` antes de reauditar.
> Suggestions são opcionais — só viram ressalva-aceita se o **usuário decidir**
> explicitamente ignorá-las (Lei 2 do `/gofi-full`). Sem decisão do usuário, o alvo é laudo limpo.

### Roteamento de reprovação (causa-raiz → fase de retorno)

A reprovação **não** volta sempre uma casa: volta à fase **dona da causa**.

- **Bug de implementação / violação de camada / padrão do SDK / conformidade
  com spec** → volta para **`gofi-eng`** (a spec está certa, o código não a
  cumpre). Maioria dos casos.
- **Spec errada / drift spec↔código onde a spec é que está errada / RN faltando
  na spec / contrato mal especificado** → volta para **`gofi-spec`**; depois
  **`gofi-eng`** re-implementa e **`gofi-qa`** reaudita.
- **Lacuna de negócio / requisito ausente / intenção errada / regra de domínio
  que ninguém definiu** → volta para **`gofi-pd`**; depois desce spec → eng → qa.

Ao voltar, **entregue ao agente anterior a lista concreta** (itens do laudo,
linhas, RN faltante) — não mande "refaça", mande "corrija isto".

### Guarda de loop (anti-ciclo infinito)

- Conte idas-e-voltas **por fase**. Se a **mesma fase reprovar 2× pelo mesmo
  motivo**, **pare e escale ao usuário**: apresente o impasse, o que já foi
  tentado, e peça **decisão** (relaxar requisito? mudar abordagem? aceitar como
  ressalva?). Isso é decisão do usuário (Lei 2 do `/gofi-full`), não falha do fluxo.
- Se um agente **pedir input que você não tem** (discovery/decisão), **não
  invente** — deixe a pergunta chegar ao usuário e aguarde a resposta antes de
  prosseguir aquela fase.
- Teto duro: **3 ciclos completos** (pd→qa) sem aprovação limpa → escale.
