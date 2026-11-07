# Pré-execução — detalhe dos passos de leitura

## Ler a spec (passo 5)

Ler a **mudança** — **fonte da verdade para conformidade** (decisão 0003): `gofi change list` para o id; os deltas em `deltas/` da pasta da mudança (o turno diz qual; `gofi change list` mostra) dão os cenários a verificar, um a um; o `design.md` dá dados, contratos, erros e o Mapa de requisitos; `project.md` §Princípios dá os portões. Requisito vivo que um delta cita: `gofi show specs/{ctx}/spec.md#REQ-NN`. Formato 2 (`sdd-*.md`): a spec é lida como antes. **Procurar documento é `gofi find`:** `gofi find "<o que você precisa saber>"` — devolve o documento, a §seção e a **faixa de linhas**; leia com `Read(offset, limit)`, nunca o arquivo inteiro. Vazio quase sempre é vocabulário, não documento faltando: tente o termo técnico (o par que faltou, diga à pessoa no fim; não escreva no léxico no meio da tarefa). Os `INDEX.md` servem para **navegar** um contexto, não para **achar** um assunto. Antes de apontar divergência entre documentos, `gofi show <doc>` mostra quem depende do que você vai questionar. Protocolo: `.claude/expertise/harness-protocols/rag-retrieval.md`

## Knowledge cross-agent (passo 6)

Ler **knowledge cross-agent**: `.claude/knowledge/INDEX.md` (núcleo ⬤ + só os módulos que a tarefa pede) (inclui `expertise/diagramming/conventions.md` — auditar se diagramas da spec/laudo são PlantUML; Mermaid/ASCII/imagem é divergência; `application-vs-domain-service.md` — auditar separação de camadas: application não chama repository direto, service não importa bridge/factory/application, erros na camada correta, tests da application mockam service e não repository). Quando o contexto usa filtro dinâmico, ler também `.claude/sdk/<lang>/knowledge/lookup-endpoints.md` e `dynamic-filter.md` — `sqln.FilterMapping` como allowlist (chave = nome de API; campo/operador/sort fora do mapping rejeitado pelo SDK, sem validação duplicada no handler; `SearchType: "embedded"` + `Content` vs `SearchType: "v1/<path>"`); rota dedicada `GET /{ctx}/status` foi descontinuada e em código novo é divergência

## Linguagem-alvo (passo 8)

Para `project.language`:
- Ler **checklist completo**: `.claude/sdk/<lang>/knowledge/qa-checklist.md`
- Ler **regras absolutas**: `.claude/sdk/<lang>/knowledge/absolute-rules.md`
- Ler os módulos de `sdk/<lang>/knowledge/` que o `.claude/knowledge/INDEX.md` indicar para padrões consolidados (cache, value-objects, repository-primitive-return, etc.)
- Ler a API dos pacotes do SDK que o contexto utiliza: `.claude/sdk/<lang>/api/INDEX.md` → só os arquivos desses pacotes (ou `gofi find --in sdk "<símbolo>"`)
- Ler, de `.claude/sdk/<lang>/boilerplates/`, só os das camadas que a mudança tocou — referência de código correto
