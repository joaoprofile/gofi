# Template A — Manual de Integração Frontend

## Templates

> Nos templates abaixo, `XXX_*`, `{contexto}`, `{recurso}`, `foo`, `name`,
> `status`, `parentId` etc. são **placeholders** — substitua pelos nomes
> reais lidos do código. Não copie os placeholders para a doc final.

### Template A — Manual de Integração Frontend (default)

**Formato: manual passo-a-passo, objetivo, sem teoria.** O leitor é dev
front que vai implementar agora — quer saber "como faço". Cada seção
responde uma pergunta concreta. Nada de discussão de design ou explicação
de por que o backend é assim.

Inclua **todas** as seções aplicáveis; omita só quando genuinamente não
se aplica.

```markdown
# {Título} — Manual de Integração Frontend

Documento baseado no código real: {arquivos lidos}.

Base URL: `{BASE_URL}/v1`  •  Auth: Bearer JWT  •  Content-Type: `application/json`

---

## Índice
[auto, com âncoras]

## 1. O que essa API faz
[3–5 linhas. O que ela controla, o que ela devolve, quem chama.]

## 2. Como autenticar
- Header: `Authorization: Bearer <token>`
- Claims que o backend lê (NÃO envie no body): [listar os reais — ex. id do tenant, id do usuário]
- Token expira → backend devolve `401`; renove via [endpoint de refresh].

## 3. Recursos e endpoints

Tabela-mapa pra dev achar rápido:

| # | Método | Path | O que faz | Auth |
|---|--------|------|-----------|------|
| 3.1 | GET | `/v1/...` | listar X | privado |
| 3.2 | POST | `/v1/...` | criar X | privado |
[uma linha por endpoint, todas as rotas registradas]

Detalhes nos blocos §3.1, §3.2, … abaixo.

[Subseção por endpoint — ver layout adiante.]

## 4. Filtro dinâmico (quando o endpoint suportar)
[Como montar o body de filtro, valores aceitos, operadores válidos,
combinações lógicas, exemplos prontos. Ver §Filtro dinâmico abaixo.]

## 5. Paginação (quando o endpoint suportar)
- Query params: `page` (default 0, **base zero**), `limit` (default conforme
  o SDK), `sort` (campo), `direction` (`ASC`/`DESC`). Confirme nomes e
  defaults reais no handler/SDK.
- Response envelope canônico (tipo de página do SDK) — **confirme os nomes
  reais dos campos** lendo o tipo de paginação em `.gofi/gofi-sdk-{lang}/`:
  ```json
  {
    "content": [ /* items */ ],
    "totalElements": 142,
    "totalPages": 10,
    "number": 0,
    "size": 15,
    "numberOfElements": 15
  }
  ```
- Nunca assuma nomes de campo do envelope sem confirmar no SDK.

## 6. Catálogos / Enums / Presets
[Valores fixos com tabela comparativa. UI usa pra montar selects.]

## 7. Validação por campo
[Tabela objetiva — uma linha por campo, sem prosa.]

| Campo | Tipo | Obrigatório | Min | Max | Formato / Aceita | Erro se inválido |
|-------|------|-------------|-----|-----|------------------|------------------|
| `name` | string | sim | 1 | 80 | qualquer | `XXX_VALIDATION` |
| `email` | string | sim | — | — | email | `XXX_VALIDATION` |
| `status` | string | sim | — | — | `ACTIVE` \| `INACTIVE` | `XXX_VALIDATION` |

## 8. Códigos de erro
| Code | HTTP | Quando ocorre | O que o front faz |
|------|------|---------------|-------------------|
| `XXX_NOT_FOUND` | 404 | … | mostrar "não encontrado" |
| `XXX_VALIDATION` | 400 | … | destacar campo no form |
| `XXX_CONFLICT` | 409 | … | "já existe — escolha outro" |
| `XXX_FORBIDDEN` | 403 | … | esconder ação ou redirecionar |

[**Listar todos** os erros do `errors.go` — service e application.]

## 9. Tipos TypeScript prontos pra colar
```ts
// requests
export interface CreateXxxRequest { ... }
export interface UpdateXxxRequest { ... }

// response
export interface Xxx { ... }
export type XxxStatus = 'ACTIVE' | 'INACTIVE';

// filtro dinâmico (quando aplicável)
export interface DynamicFilter { field?: string; condition?: string; value?: unknown; logicalOperator?: 'AND' | 'OR'; }
export interface FilterRequest { filters: DynamicFilter[]; page?: number; limit?: number; sort?: string; direction?: 'ASC' | 'DESC'; }
```

## 10. Passo a passo de implementação
[Sequência numerada que o dev segue na ordem. Cada passo é uma ação concreta.]

1. Adicionar tipos do §9 em `src/api/{contexto}.ts`.
2. Criar função `list{Xxx}(filters)` que faz `POST /v1/.../query` (§3.X).
3. Criar função `get{Xxx}ById(id)` que faz `GET /v1/.../{id}` (§3.Y).
4. Criar função `create{Xxx}(body)` que faz `POST /v1/...` (§3.Z).
5. Mapear erros da §8 → toasts/inline UI.
6. Se houver filtro dinâmico: usar `POST /v1/.../schemas` (§3.W) ao montar a
   tela e construir os selects a partir do objeto devolvido (chave = campo,
   opções em `schema[campo].content`).
7. Para paginação: começar `page=0`, controles infinitos ou paginados a
   partir de `totalPages`.

## 11. Armadilhas
[Lista curta, uma linha cada. Só coisas que vão pegar o dev de surpresa.]

- Parser de data estrito no backend (ex.: RFC3339 sem milissegundos) →
  enviar data **sem** `.000Z`: `d.toISOString().replace(/\.\d{3}Z$/, 'Z')`.
- Campo `parentId` é zerado silenciosamente quando `PUT` altera outro campo.
- `page` começa em `0`, não `1`.
- Path param sempre validado como ID — `400 XXX_VALIDATION` se malformado.
- Token sem a claim de tenancy → `403`, não `401`.
```

#### Subseção de endpoint (uma por rota)

Cada bloco responde: o que pedir, o que volta, o que pode dar errado.

```markdown
### 3.N METHOD /v1/path/{param}

**O que faz:** [uma linha — verbo concreto, sem teoria]

**Quando usar:** [uma linha — em que momento da tela o front chama isso]

**Path params**
| Param | Tipo | Obrigatório | Formato | Descrição |
|-------|------|-------------|---------|-----------|
| `id` | string | sim | UUID v4/v7 | id do recurso |

**Query params**
| Param | Tipo | Default | Descrição |
|-------|------|---------|-----------|
| `page` | uint16 | 0 | base zero |
| `limit` | uint16 | 15 | máx 100 |

**Headers**
- `Authorization: Bearer <token>` — obrigatório

**Request — exemplo pronto pra colar**

```http
POST /v1/{contexto}/{id}/config HTTP/1.1
Host: {api-host}
Authorization: Bearer eyJhbGciOi...
Content-Type: application/json

{
  "name": "Example Name",
  "parentId": "0190f6a3-7e2c-7c3a-9a55-3f1b6e8d9ab2",
  "active": true
}
```

**Validações deste endpoint** (resumo do §7 filtrado)
| Campo | Regra |
|-------|-------|
| `name` | string, 1–80 |
| `parentId` | UUID, opcional |

**Resposta {STATUS} — sucesso**
```json
{
  "id": "0190f6a3-9c1d-7e8a-bf3a-2c8d4e5f1a0b",
  "name": "Example Name",
  "parentId": "0190f6a3-7e2c-7c3a-9a55-3f1b6e8d9ab2",
  "active": true,
  "createdAt": "2026-05-30T14:23:11Z",
  "updatedAt": "2026-05-30T14:23:11Z"
}
```

**Resposta de erro**
| HTTP | Code | Quando | Body exemplo |
|------|------|--------|--------------|
| 400 | `XXX_VALIDATION` | name vazio | `{"code":"XXX_VALIDATION","message":"..."}` |
| 404 | `XXX_NOT_FOUND` | id inexistente | `{"code":"XXX_NOT_FOUND","message":"..."}` |
| 409 | `XXX_CONFLICT` | nome duplicado | `{"code":"XXX_CONFLICT","message":"..."}` |

> Notas específicas: [comportamentos não-óbvios deste endpoint em uma linha cada]
```

#### Filtro dinâmico (§4) — layout obrigatório quando o endpoint usa filtro dinâmico do SDK

Filtro dinâmico é onde o front mais erra. **Trate como cidadão de primeira
classe**: schema dos campos, operadores aceitos, exemplos prontos. O shape
exato (nomes de campo do request, operadores, formato do `/schemas`) vem do
SDK/knowledge — confirme antes de afirmar.

```markdown
## 4. Filtro dinâmico

### 4.1 Como montar uma chamada com filtro

1. Chame `POST /v1/{contexto}s/schemas` (§3.X) **uma vez** ao montar a tela —
   devolve quais campos podem ser filtrados/ordenados, com que operadores e
   que valores aceitam.
2. Construa o body com `params` (paginação/ordenação) e a lista `filters`
   (regra abaixo), usando **só** os nomes de campo que o schema devolveu.
3. Faça `POST /v1/{contexto}s/query` (§3.Y) com o body montado.

### 4.2 Shape do request

```json
{
  "params": { "page": 0, "limit": 15, "sortField": "created", "sortDirection": "DESC" },
  "filters": [
    { "field": "status", "condition": "IN", "value": ["ACTIVE", "PENDING"] },
    { "logicalOperator": "AND" },
    { "field": "name", "condition": "LIKE", "value": "abc" }
  ]
}
```

**Regras invioláveis:**
- `field` e `sortField` são o **nome de API** (chave do schema), nunca a
  coluna do banco.
- Paginação só em `params` (`page` base zero, `limit`, `sortField`,
  `sortDirection` `ASC`/`DESC`) — nunca na raiz.
- Cada item de `filters` ou tem `{field, condition, value}` **ou** tem só
  `{logicalOperator}`.
- Operadores lógicos (`AND`/`OR`) **separam** filtros — nunca abrem ou
  fecham a lista, nunca aparecem consecutivos. Sem conector entre dois
  filtros = `AND`.
- Filtro 1, AND, Filtro 2, OR, Filtro 3 → ✅
- AND, Filtro 1, Filtro 2 → ❌ (lidera com operador)
- Filtro 1, AND, AND, Filtro 2 → ❌ (operadores consecutivos)
- Lista vazia → devolve tudo (paginado).

### 4.3 Operadores

Os operadores aceitos por campo vêm em `schema[campo].ops` — é a fonte da
verdade. `condition` é o operador literal (alias como `"eq"`/`"contains"` é
rejeitado):

| Operador | Significado | Formato de `value` |
|----------|-------------|--------------------|
| `=`, `!=` | igual / diferente | escalar |
| `<`, `<=`, `>`, `>=` | comparação | número ou data ISO-8601 |
| `BETWEEN` | intervalo | `"inicio\|fim"` (datas RFC3339) |
| `IN`, `NOT IN` | lista | array de strings/IDs |
| `LIKE`, `NOT LIKE` | contém (case-insensitive) — mande só o termo | string |
| `IS NULL`, `IS NOT NULL` | nulo / não nulo | omitir `value` |

### 4.4 Campos disponíveis para filtrar (do `schema` deste endpoint)

| Field | Label | FilterType | Ordenável | Operadores (`ops`) | Valores aceitos |
|-------|-------|------------|-----------|--------------------|-----------------|
| `name` | NAME | text | sim | `LIKE`, `=`, … | qualquer string |
| `status` | STATUS | search-multiple | — | `IN`, `NOT IN`, `=`, … | `ACTIVE`, `INACTIVE`, `PENDING` (§6) |
| `parentId` | PARENT | search-multiple | — | `IN`, `NOT IN`, … | IDs obtidos via `GET /v1/{recurso-pai}` |
| `created` | CREATED_AT | — | sim | `<`, `>`, `BETWEEN`, … | ISO-8601 |

[Uma linha por chave do schema (o `FilterMapping` do handler). Para
`searchType: "embedded"`, listar valores inline na coluna "Valores aceitos".
Para `searchType: "<path>"`, apontar o endpoint que devolve os valores.]

### 4.5 Mocks prontos pra copiar

**Listar tudo (sem filtro), página 0**
```json
{ "params": { "page": 0, "limit": 15 }, "filters": [] }
```

**Filtro single — status = ACTIVE**
```json
{
  "params": { "page": 0, "limit": 15 },
  "filters": [
    { "field": "status", "condition": "=", "value": "ACTIVE" }
  ]
}
```

**Filtro multi — status IN (ACTIVE, PENDING)**
```json
{
  "params": { "page": 0, "limit": 15 },
  "filters": [
    { "field": "status", "condition": "IN", "value": ["ACTIVE", "PENDING"] }
  ]
}
```

**Filtro composto — status ACTIVE AND name contém "abc"**
```json
{
  "params": { "page": 0, "limit": 15 },
  "filters": [
    { "field": "status", "condition": "=", "value": "ACTIVE" },
    { "logicalOperator": "AND" },
    { "field": "name", "condition": "LIKE", "value": "abc" }
  ]
}
```

**Filtro OR — status ACTIVE OR name contém "xyz"**
```json
{
  "filters": [
    { "field": "status", "condition": "=", "value": "ACTIVE" },
    { "logicalOperator": "OR" },
    { "field": "name", "condition": "LIKE", "value": "xyz" }
  ]
}
```

**Filtro BETWEEN — criado entre duas datas**
```json
{
  "filters": [
    { "field": "created", "condition": "BETWEEN",
      "value": "2026-01-01T00:00:00Z|2026-05-30T23:59:59Z" }
  ]
}
```

**Filtro IS NULL — sem parent definido**
```json
{
  "filters": [
    { "field": "parentId", "condition": "IS NULL" }
  ]
}
```

### 4.6 Resposta do `/schemas` (uma vez por tela)

Objeto indexado pelo nome de API; campos vazios são omitidos:

```json
{
  "status": { "ops": ["=", "!=", "IN", "NOT IN", "IS NULL", "IS NOT NULL"],
              "label": "STATUS", "filterType": "search-multiple",
              "searchType": "embedded",
              "content": { "ACTIVE": "ACTIVE", "INACTIVE": "INACTIVE" } },
  "name":   { "ops": ["=", "!=", "LIKE", "NOT LIKE", "..."], "sortable": true,
              "label": "NAME", "filterType": "text" },
  "created": { "ops": ["<", "<=", ">", ">=", "BETWEEN", "..."], "sortable": true,
               "label": "CREATED_AT" }
}
```

**Regra do `content`:**
- `searchType: "embedded"` → use `schema[campo].content` direto pra popular o select.
- `searchType: "v1/<path>"` → faça `GET /v1/<path>` pra carregar os valores.

### 4.7 Erros típicos do filtro
| Code | Quando |
|------|--------|
| `XXX_VALIDATION` | `field` não é chave do schema |
| `XXX_VALIDATION` | `condition` fora de `schema[campo].ops` |
| `XXX_VALIDATION` | `sortField` não é campo com `sortable: true` |
| `XXX_VALIDATION` | `BETWEEN` fora do formato `inicio\|fim` |
| `XXX_VALIDATION` | dois `logicalOperator` consecutivos |
| `XXX_VALIDATION` | filter list começando/terminando com `logicalOperator` |
```

