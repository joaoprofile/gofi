# Pré-execução obrigatória — detalhe dos passos

## Pré-execução obrigatória

1. Ler `.gofi.yaml` (raiz) — `project.language` (decide o pack do design), `project.name`, **`project.path`** (`pathService` — raiz do módulo da linguagem-alvo) e demais configurações.
2. O `AGENTS.md` da raiz (já carregado) — mapa de paths físicos.
3. Ler `.claude/memory/project.md` — serviços, convenções e **`## Princípios`**: os portões que todo design respeita (sem estado por-contexto; `/gofi-status` para o índice).
4. Ler `.claude/memory/contexts/{contexto}.md` se existir — frontmatter + handoff do gofi-pd.
5. **O que o contexto já tem, pelo índice, nunca pela árvore:**
   - `gofi show ctx:{contexto}` — spec viva, mudanças, tabelas, pacotes, vizinhos e onde a corrente arrebenta.
   - `gofi show specs/{contexto}/spec.md` lista os requisitos com as linhas; `gofi show specs/{contexto}/spec.md#REQ-NN` lê **um** requisito com os seus cenários.
   - `gofi change list` — mudanças abertas: uma que já toca o contexto pode conflitar com a nova, e define o próximo número livre.
   - Procurar qualquer outra coisa é `gofi find "<o que você precisa saber>"` (doc, §seção, linhas). Vazio quase sempre é vocabulário: tente o termo técnico e registre o par em `.claude/lexicon/sinonimos.md`.
6. **Contexto que evolui ou integra código existente — o grafo antes de propor:** `gofi show <pacote>` e `gofi show <símbolo>` no que a mudança vai tocar. É o que impede o design de mandar criar o que já existe, ou de mudar um ponto central sem declarar o impacto na proposta. Protocolo: `.claude/expertise/harness-protocols/graph-retrieval.md`. Todo pacote novo nasce com `//gofi:context {contexto}`.
7. Ler **knowledge cross-agent**: `.claude/knowledge/INDEX.md` (núcleo ⬤ + só os módulos que a tarefa pede) — em especial `ddd-principles.md` (invariantes viram requisitos NÃO DEVE), `expertise/diagramming/conventions.md` (PlantUML no design), `application-vs-domain-service.md` (o design diz quais operações são use case `application/`), `event-driven-executor-pattern.md` (split decider/executor declarado no design).
8. Ler **knowledge per-agent**: `.claude/knowledge/spec/*.md` (user-treinado).
9. **Templates** — o formato de saída:
   - `.claude/templates/proposal-template.md` e `delta-template.md` — a mudança;
   - `.claude/sdk/<lang>/templates/design-template.md` — o design no stack (ou `.claude/templates/design-template.md`, neutro, sem pack);
   - `.claude/templates/spec-template.md` — só no modo baseline.
   `gofi change new` já cria a pasta a partir deles.
10. Para o design, em `project.language`:
    - `.claude/sdk/<lang>/knowledge/structure.md` — onde os arquivos ficam;
    - `env-vars-standard.md`, `dynamic-filter.md` (se houver filtro dinâmico), `migrations.md` (par up/down), `absolute-rules.md`;
    - `.claude/sdk/<lang>/api/INDEX.md` — os pacotes do SDK; abra só o do pacote pertinente, ou `gofi find --in sdk "<símbolo>"`.
