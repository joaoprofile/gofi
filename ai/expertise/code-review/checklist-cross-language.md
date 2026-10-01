# Escopo de auditoria — itens cross-language

A lista exata de itens a verificar para a linguagem-alvo está em
**`.claude/sdk/<lang>/knowledge/qa-checklist.md`**. Aplique todos os
itens relevantes ao contexto.

Em todo contexto, você verifica também:

### Conformidade com a spec (cross-language)
- [ ] Todos os campos da entidade estão implementados
- [ ] Todas as operações listadas existem e funcionam
- [ ] Todas as RN-* estão implementadas
- [ ] HTTP status codes correspondem ao mapeado na spec
- [ ] Filtros de listagem se comportam como especificado
- [ ] Ciclo de vida de status segue o documentado

### Separação de camadas (cross-language, ver `sdk/<lang>/knowledge/layers.md`)
- [ ] Handler não acessa repository diretamente
- [ ] Service não conhece tipos de transporte (HTTP)
- [ ] Repository não conhece DTOs
- [ ] Handler não contém lógica de negócio

### Testabilidade (cross-language)
- [ ] Service recebe interface de repository
- [ ] Handler recebe interface de service
- [ ] Service test cobre: sucesso, validação inválida, not-found, erro de repo
- [ ] Handler test cobre: sucesso, decode error, service error
- [ ] Mocks são handcraft (sem frameworks externos)
- [ ] Mock implementa todos os métodos da interface (incluindo cleanup)
- [ ] **Todo bug fix / melhoria de comportamento na entrega tem teste de regressão** ancorado no cenário corrigido (reproduz o defeito → assert do comportamento correto), na camada onde o defeito mora. Cruzar `## Histórico de Alterações` da spec + memória do contexto + diff contra os `_test.go` tocados. Fix/melhoria sem teste que o trave é **MAJOR** — o objetivo do teste é impedir regressão futura, não só cobrir a linha nova

### Segurança geral
- [ ] Sem SQL concatenado (parâmetros posicionais sempre) — `gofi find --regex --in code` pelo padrão de concatenação/`Sprintf` em SQL mostra a linha e o `◆ Símbolo` que a contém
- [ ] Sem dados sensíveis em log (senha, token, CPF completo)
- [ ] Sem erros internos vazando em respostas HTTP
- [ ] IDs validados antes de uso
- [ ] **`tenant.id` e `user.id` são UUID** no schema (`UUID PRIMARY KEY` **sem `DEFAULT`** — geração é responsabilidade da aplicação); **toda FK** que aponte para eles (`*.tenant_id`, `*.created_by_user_id`, `*.author_user_id`, etc.) também é UUID. **Migration NÃO declara `CREATE EXTENSION IF NOT EXISTS "pgcrypto"` para gerar UUID** — apontar como **MAJOR** se schema tiver `DEFAULT gen_random_uuid()`/`NEWID()`/`SYS_GUID()`. **Service gera o `id` como UUIDv7** (Go: `uuid.NewV7()` do `github.com/google/uuid` ≥ v1.6.0) antes de chamar `repo.Save(...)`; uso de `uuid.NewString()` / `uuid.New()` (que retornam v4) para PK nova é **MAJOR** — perde ordenação temporal e fragmenta índice B-tree. Validators de DTO usam `validate:"uuid"` (qualquer versão) — `validate:"uuid4"`/`validate:"uuid7"` é **MINOR** (lock em versão quebra evolução do produtor). Repo NÃO usa `INSERT ... RETURNING id` (apontar como MAJOR se usar — `Save` deve devolver apenas `error`). Em Go, modelados como `string` em entidade/DTO/contratos. Path params validam formato UUID antes do service. Regra completa em `.claude/expertise/ddd-architecture/id-types.md` — apontar como divergência se a implementação usa `BIGINT`/`int64` para esses IDs sem ADR explícita justificando exceção

### Aderência ao knowledge user-treinado
- [ ] Padrões registrados em `.claude/knowledge/qa/*.md` foram respeitados pelo gofi-eng

## Lei do teste de regressão (MAJOR)

**Bug fix ou melhoria sem teste de regressão → MAJOR (LEI absoluta).**
Toda correção de bug e toda melhoria de comportamento na entrega **tem que**
vir acompanhada de um teste que **ancora o cenário corrigido** (input que
reproduzia o defeito → assert do comportamento correto), no lugar certo da
pirâmide (service/handler/repository/adapter). Ausência é **MAJOR** — o QA
audita isso explicitamente (ver §"Testabilidade (cross-language)"). Descobrir o que é fix/
melhoria: `## Histórico de Alterações` da spec + `## Estado atual`/`## Histórico
de versões` da memória do contexto + o diff. Fix confirmado sem teste que o
trave é reprovação.
