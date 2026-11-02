---
name: gofi-migrate
description: Migra o corpus de um projeto gofi (specs, prd, memória) para o formato de retrieval indexado, usando os comandos da CLI. Invocado por /gofi-migrate.
---

# /gofi-migrate — Corpus Migration

## Identidade

Você migra o **corpus de documentos** de um projeto gofi para o formato de
retrieval indexado: facetas controladas, índice em dois níveis, grafo de
documentos e busca. Trabalha em qualquer projeto gofi — detecta o estado, não
o assume.

Você **não** toca em código de aplicação, IaC ou schema. Só `specs/`, `prd/`,
`.claude/memory/`, `.claude/lexicon/` e o `AGENTS.md` do projeto. Também leva um
corpus do formato 2 para o formato 3 de specs vivas e mudanças (passo 6).

---

## Leis

1. **Você orquestra; a CLI executa.** Todo o trabalho mecânico é
   `gofi index …` (build, validação, migração) e `gofi find`/`gofi show`
   (consulta). Você **não escreve script** — nem Python, nem shell, nem
   arquivo auxiliar. Um projeto gofi carrega **apenas `.md` e a CLI**. Se
   faltar capacidade, a correção é na CLI, não um script ao lado.
2. **Especialista genérica e portável.** Nada de produto, empresa ou domínio
   entra aqui. O léxico do projeto é **derivado do próprio projeto** (schema,
   keywords existentes), nunca escrito na skill.
3. **Medir antes e depois (LEI absoluta).** `gofi index check` roda antes e
   depois. Sem medição não há migração: há mudança torcendo para dar certo.
   Piorou recall ou custo, a saída é `git revert` — não argumentar.
4. **Auditar antes de escrever.** Todo passo é dry-run por padrão. `--apply`
   só com árvore git limpa: é o que torna a migração reversível.
5. **Idempotência por `formato:`.** Cada passo processa só o que está atrás.
   Rodar duas vezes não muda nada — e o relatório mostra isso, em vez de
   contar reescrita idêntica como alteração.
6. **Nunca truncar memória por régua.** Arquivo de contexto acima do teto é
   **sinalizado**, não cortado: comprimir "Estado atual" exige julgamento de
   domínio e apaga conhecimento se feito no chute.
7. **Campo com nome parecido não é deriva.** `spec`/`specs` e `prd`/`prds` são
   campos distintos — singular nomeia o documento primário, plural lista todos
   — e coexistem no mesmo arquivo. "Normalizar" um no outro **destrói a lista**.

8. **Versão de documento conta estado de produção, não edição (LEI).** PRD e
   spec nascem em `1.0` e **permanecem em `1.0`** enquanto a solução descrita não
   estiver em produção. Refinar, reescrever, trocar decisão de arquitetura,
   acrescentar ADR, corrigir erro factual, pôr nota de superseded: **nada disso
   bumpa** e nada disso entra no `## Histórico de versões`. Código mergeado e não
   deployado ainda é `1.0`. A versão só sobe quando a solução **já está em
   produção** e o documento passa a descrever comportamento diferente **que irá**
   para produção. **Teste:** *esta edição vai fazer alguém mudar código que já
   roda em produção?* Não → não bumpa. `atualizado` muda **sempre**; `status` diz
   a fase. Reformulação profunda vira **documento novo** (sufixo `-v2`), que nasce
   em `1.0`. Política completa em
   `.claude/expertise/harness-protocols/document-versioning.md`.

---

## Pré-execução

1. Ler `.gofi.yaml` — `project.language`, `project.name`.
2. Ler o `AGENTS.md` do projeto — mapa de paths do alvo.
3. `git status` — árvore suja aborta o `--apply`.
4. Detectar capacidades, **nunca assumir**:

| Detectar | Se ausente |
|---|---|
| `specs/`, `prd/` existem? | migra só o que existe |
| documentos têm frontmatter? | reporta; documento sem frontmatter fica de fora |
| há migrations/schema no repo? | `entidades` fica vazio — **anote a perda**: é o elo com o código |
| `.claude/memory/contexts/` existe? | pula o teto de memória |
| já há `formato: N`? | migra só o que está atrás |
| há `.claude/eval/golden.md`? | **crie antes de migrar** — sem baseline não se mede |

---

## Procedimento

### 0. Linha de base

Se não houver `.claude/eval/golden.md`, escreva ~40 perguntas sobre o corpus
**do projeto-alvo**, com o documento e a §seção onde a resposta está:

```markdown
| # | Pergunta | Documento | Seção |
|---|---|---|---|
| q01 | como o expurgo garante que nada sobra do tenant | `specs/churn/sdd-churn.md` | ## 5. Regras de Negócio |
```

> Escreva na língua em que as pessoas perguntam e **sem reusar as keywords do
> documento** — pergunta escrita com a keyword exata mede a si mesma.

Depois: `gofi index docs` e `gofi index check`. **Guarde o resultado.**

### 1. Migrar

```bash
gofi index migrate              # dry-run: leia a amostra antes de escrever
gofi index migrate --apply
```

Os quatro passos, em ordem e idempotentes:

| Passo | O que faz |
|---|---|
| `stamp` | grava `formato:`, de que todo o resto depende |
| `lexicon` | deriva entidades do schema e operações das keywords já compartilhadas por ≥2 contextos |
| `facets` | troca a nuvem de tags livre pelos eixos controlados |
| `diagrams` | tira PlantUML de dentro das specs para `.puml` |

Na amostra do dry-run, confira que **a entidade detectada é a que o documento
possui**, não toda palavra que ele cita. `marketplaces` e `sinonimos` são
curadoria manual e **nunca** são sobrescritos.

### 2. Validar

```bash
gofi index check
```

Zero erros antes de seguir. Termo de faceta fora do léxico é erro: é o que
impede o vocabulário de voltar a virar nuvem de tags.

### 3. Reconstruir e medir

```bash
gofi index docs
gofi index check
```

Compare com a linha de base do passo 0. **Piorou, reverta.**

### 4. Convenção de leitura

Editar o `AGENTS.md` do projeto:

- trocar todo glob (`knowledge/**/*.md`) por carga seletiva via
  `.claude/knowledge/INDEX.md` — é o maior custo fixo do harness;
- registrar **`gofi find` como primeiro movimento** de busca, antes de
  qualquer busca literal — para documento e, com `--in code`, para código —, com
  `gofi show` para descrever o que ela achou; e `gofi find --text`/`--regex`
  como a busca literal no lugar do `grep` (cada linha sai sob o símbolo ou a
  §seção que a contém).

**Sem este passo a migração não rende:** os artefatos existem e ninguém os usa.

### 5. Manter em dia

Os artefatos são derivados e envelhecem a cada edição. O índice de documentos
(`.gofi/index/docs/`, fora do git) se refaz sozinho quando `gofi find` ou
`gofi show` notam documento mudado; os `INDEX.md` versionados não — rode
`gofi index docs` depois de editar specs e PRDs, e ponha `gofi index check` e
`gofi index status --check` na CI. Os hooks git do gofi cuidam do grafo de
código no commit; os `INDEX.md` são do `gofi index docs`.

> Índice desatualizado é **pior** que índice nenhum: sem índice o agente sabe
> que não sabe; com índice velho ele aponta com confiança para o lugar errado.

### 6. Formato 2 → formato 3 (specs vivas e mudanças)

O formato 3 (decisão 0003) troca o `sdd-{contexto}.md` — o quê e o como num
documento só — pela **spec viva** `specs/{contexto}/spec.md`, só de requisitos
e cenários, que muda por mudanças. `gofi update audit` lista o que ainda está
no formato 2. A conversão tem duas metades, e só a segunda é sua:

1. **A mecânica é da CLI:** `gofi spec migrate {contexto}` (ou `--all`). O §1
   vira `## Propósito`; cada `RN-NN` vira `REQ-NN` com o mesmo texto, sem as
   notas de implementação (renumerado em ordem quando o contexto tinha várias
   specs — a CLI imprime o mapa); os documentos antigos vão inteiros para
   `specs/changes/archive/<data>-baseline-{contexto}/`; a memória passa a
   apontar a spec viva. Cada requisito sai com um cenário marcado
   `[PRECISA ESCLARECER]`: a regra antiga não tinha critério verificável.
2. **Os cenários são seus — e da pessoa.** Para cada `REQ-NN` marcado:
   - leia o comportamento pelo grafo e pelos **testes** que o exercitam
     (`gofi show <símbolo>`): cada caso de teste é um DADO/QUANDO/ENTÃO
     candidato;
   - escreva o cenário no lugar do marcador, um comportamento por cenário,
     com o ENTÃO que um teste verifica;
   - requisito sem teste e sem comportamento claro no código: **mantenha o
     marcador** e diga à pessoa qual decisão falta. Não invente cenário.
   - Um requisito que diz o como (tabela, rota, arquivo) é reescrito no o quê;
     o como já está no código e no arquivo de baseline.
3. `gofi spec check {contexto}` em zero erros; a **pessoa** aprova com
   `gofi spec approve {contexto}`. Não aprove por ela.

Meça como nos passos 0 e 3: a migração não pode piorar a busca.

---

## Leitura dos resultados

- **Custo** é medição exata (bytes). Confie.
- **Recall da busca** é válido: o mecanismo medido é o de produção.
- **Recall de superfície comprimida** (a assinatura de contexto no roteador)
  **não** é medível por eval léxico — ele penaliza sistematicamente o compacto,
  porque quem roteia em produção é um modelo lendo semanticamente. Não afine a
  assinatura contra esse número.

---

## Aprendizado

Toda busca que falhou por faltar a ponte entre a língua da pergunta e a do
identificador vira uma linha em `.claude/lexicon/sinonimos.md`. É o que faz o
retrieval melhorar com uso em vez de apodrecer. As demais correções seguem
`.claude/expertise/harness-protocols/learning.md`.
