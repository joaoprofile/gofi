# Leitura por endpoint — o que abrir e o que extrair

## Pré-execução por endpoint

Identificado o(s) handler(s), ler **na ordem**:

1. `handler/{aggregate}_handler.go` — rotas, path params, query params,
   auth, status codes
2. `handler/middleware.go` se existir — `authFromContext`, claims disponíveis
3. `application/{aggregate}_application.go` se existir — workflow
   orquestrador (chamado pelo handler quando há coordenação cross-domain ou saga)
4. `service/{aggregate}_service.go` — regras implícitas, valores
   preservados/resetados
5. `application/errors.go` **e** `service/errors.go` — todos os códigos de
   erro do contexto
6. `model/{aggregate}_dto.go` ou `model/{aggregate}.go` — structs
   request/response/entity com tags de validação
7. `model/presets.go`, `model/*_constants.go`, `model/entity.go` — enums,
   defaults, catálogos
8. `repository/{aggregate}_repository.go` quando o endpoint usa SELECT
   customizado (constante SQL — colunas `--` comentadas não chegam ao response)
9. **Composition root** (`services/{api-service}/wire.go`) — prefixo `/v1`,
   public vs private, ordem de middlewares
10. **`.migrations/*.sql` do contexto** — `NOT NULL`, `DEFAULT`, `UNIQUE`,
    `CHECK` revelam: nullability do response, valores iniciais, mutações que
    disparam 409, faixa de valores aceita

Se o endpoint retorna paginação (tipo de página do SDK, ex.: `Page[T]`):

11. Ler o **tipo de paginação do SDK** em `.gofi/gofi-sdk-{lang}/` — extrair
    os nomes JSON reais do envelope. **Nunca** assuma os nomes dos campos;
    confirme no código do SDK (e/ou na knowledge do passo 5 de `project-discovery.md`).

Não pule `errors.go` — é onde estão os códigos reais que o frontend tem que
mapear e o QA tem que cobrir.


## O que extrair de cada arquivo

### Do handler (`_handler.go`)
- Método HTTP e path completo (incluindo prefixo `/v1`)
- Path params e query params (como são lidos da request)
- Se a rota valida `authFromContext` (privada vs pública)
- Status HTTP de sucesso e de erro
- Quais campos do `auth` o backend usa (claims de tenancy/identidade) — **o
  frontend NÃO envia esses no body**; os nomes reais vêm do middleware/claims
  do projeto

### Do service / application (`_service.go`, `_application.go`)
- Regras de negócio que afetam o contrato:
  - Campos resetados em certas operações (ex.: um campo derivado zerado no PUT)
  - Campos preservados de registros anteriores
  - Defaults aplicados quando registro não existe
  - Validações de ID antes de qualquer operação
  - Limites hardcoded (ex.: máximo N itens por bulk)
- O que retorna: `(*T, error)`, `(T, error)`, `error` — determina se há
  corpo na resposta de sucesso

### Do errors.go (service e application)
- Código string exato (ex.: `"XXX_NOT_FOUND"`)
- Tipo → HTTP: not found → 404, validation → 400, operation → 500,
  forbidden → 403, conflict → 409 (mapeie pelos helpers reais do projeto)
- Mensagem human-readable

### Dos DTOs (`_dto.go`)
- Campos request com tags de validação (obrigatório, `min`, `max`, `oneof`,
  `uuid`, `email`, etc.)
- Campos response com tipos JSON
- Campos nullable (ponteiro no modelo) → `T | null` no TS
- **Campos comentados não existem**: campo comentado no struct ou `-- coluna`
  no SQL do repo = campo desativado, **não documentar**

### Das constantes (`presets.go`, `*_constants.go`)
- Enums e valores string exatos (válidos para path params e body)
- Defaults, limites, catálogos
- Maps de referência que o frontend pode exibir como legenda

### Das migrations (`.migrations/*.sql`)
- `NOT NULL` confirma campo obrigatório no response
- `DEFAULT` revela valor inicial quando registro recém-criado
- `UNIQUE` indica que mutation pode disparar 409
- `CHECK` revela faixa de valores aceita
- `FK ... ON DELETE` revela cascata invisível

