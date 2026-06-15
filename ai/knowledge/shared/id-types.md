---
title: Tipos de ID — quando usar UUID vs sequencial
scope: cross-agent, cross-language
applies_to: [gofi-spec, gofi-eng, gofi-qa, gofi-ui]
last_updated: 2026-05-09
---

# Tipos de ID — quando usar UUID vs sequencial

Regra do toolchain. Vale para qualquer linguagem-alvo (`project.language`).
`gofi-spec` deve seguir na elicitação **sem perguntar**; `gofi-eng` aplica
na implementação; `gofi-qa` aponta como divergência caso a regra seja
violada; `gofi-ui` modela como `string` no TypeScript/equivalente.

---

## Regra

> **Entidades de identidade — `tenant` e `user` — sempre têm `id` UUID.**

Especificamente:

- `tenant.id` → coluna `UUID PRIMARY KEY` **sem `DEFAULT`** — quem gera o
  valor é a aplicação (ex.: `uuid.NewString()` no `service` antes do `Save`).
  Modelado em Go como `string` (sem dependência de tipo customizado).
- `user.id` → idem.
- **Toda FK** que aponte para `tenant.id` ou `user.id` (qualquer coluna do tipo
  `*.tenant_id`, `*.created_by_user_id`, `*.author_user_id`, etc.) **também** é UUID.
- Em **qualquer linguagem**, IDs UUID são modelados pelo tipo nativo do
  ecossistema (Go: `string`; Java: `UUID`; Rust: `Uuid`; TypeScript: `string`).

> **Geração na aplicação, não no banco.** A coluna **é** do tipo `UUID`, mas
> **não** carrega `DEFAULT gen_random_uuid()` (Postgres) nem equivalente. Por
> consequência, **não use `CREATE EXTENSION IF NOT EXISTS "pgcrypto"`** na
> migration — não há dependência. O `service` gera o UUID antes de chamar
> `repo.Save(...)`; o `INSERT` envia o `id` explicitamente em `VALUES (...)`,
> sem `RETURNING id`. Vale para `tenant`, `user` e qualquer outra entidade
> que a spec marcar como UUID por anti-enumeração.

> **Versão do UUID: sempre v7** (RFC 9562). Os primeiros 48 bits são
> timestamp Unix em milissegundos, o que torna o id **time-ordered** —
> resolve o pior problema de UUIDv4 como PK em B-tree: inserts aleatórios
> espalhados pelo índice causam fragmentação e perdem localidade de cache.
> v7 mantém anti-enumeração (62 bits aleatórios) e ainda permite `ORDER BY id`
> aproximar `ORDER BY created_at` de graça. Em Go, gerar com
> **`uuid.NewV7()`** do `github.com/google/uuid` (≥ v1.6.0). **Nunca** usar
> `uuid.NewString()` / `uuid.New()` (ambos retornam v4) para PK nova. UUIDs
> v4 herdados de schema legado continuam válidos para leitura — a regra é
> sobre **geração nova**.

Todas as **outras entidades** (qualquer entidade de negócio que **não**
apareça em URL pública, path param autenticado, JWT claim ou subdomínio)
seguem o **default sequencial** (`BIGINT GENERATED ALWAYS AS IDENTITY` em
PostgreSQL) — exceto se a spec/PRD indicar requisito explícito de
anti-enumeração para aquela entidade.

## Por quê

- **Anti-enumeração:** `tenant` e `user` aparecem em path params autenticados
  (`/api/v1/tenants/{id}`, `/api/v1/users/{id}`), em URLs públicas (rotas
  `/public/tenants/{slug}/...`), e em claims de JWT. ID sequencial permite
  descobrir quantos tenants/usuários existem e iterar trivialmente.
- **Não vaza volume:** `tenant.id=42` revela "este SaaS tem ~42 clientes";
  `user.id=3` revela "tenant tem só 3 usuários". UUIDs aleatórios não.
- **Identidades distribuídas:** UUID é estável para distribuição cross-region,
  edge caching e tracking sem coordenação central.

Entidades de negócio internas (transacionais, operacionais, contábeis,
catálogo) **não** vazam informação sensível por contagem — o atacante já
precisa estar autenticado no tenant e o filtro `WHERE tenant_id = $1` as
protege. Para essas, sequencial dá índice menor, joins mais baratos e
chave humana-legível em logs/debug.

## Como aplicar (linguagem-agnóstico)

### SQL (qualquer dialeto que suporte UUID)

```sql
-- PostgreSQL (default do toolchain) — sem extensão pgcrypto, sem DEFAULT.
-- A aplicação gera o UUID antes do INSERT.
CREATE TABLE tenant (
    id UUID PRIMARY KEY,
    -- ... resto
);

CREATE TABLE "user" (
    id        UUID PRIMARY KEY,
    tenant_id UUID NOT NULL REFERENCES tenant(id),
    -- ... resto
);

-- FKs em qualquer outro contexto que apontem para tenant/user
CREATE TABLE audit_log (
    id              BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    tenant_id       UUID NOT NULL REFERENCES tenant(id),
    author_user_id  UUID REFERENCES "user"(id) ON DELETE SET NULL,
    -- ... resto
);
```

> **Em qualquer dialeto, o app é o gerador.** `id UUID PRIMARY KEY` sem
> `DEFAULT`/`NEWID()`/`SYS_GUID()`. MySQL 8+ usa `BINARY(16)` ou `CHAR(36)`,
> SQL Server `UNIQUEIDENTIFIER`, Oracle `RAW(16)` — todos sem geração no
> banco. Mantenha o tipo na linguagem-alvo como `string`/`UUID`.

### Spec (`gofi-spec`)

Seções afetadas em toda spec que envolva `tenant` ou `user`:

- **§3.1 Entidade** — coluna `id`/`tenant_id`/`*_user_id` declarada como `UUID`;
  tipo Go (ou linguagem-alvo) como `string`/UUID nativo.
- **§3.4 Tabelas SQL** — `id UUID PRIMARY KEY` **sem `DEFAULT`**. Não declarar
  `CREATE EXTENSION IF NOT EXISTS "pgcrypto"` — geração é responsabilidade da
  aplicação. Apontar explicitamente em §3 ou ADR que o `service` gera o UUID
  **v7** (ex.: Go `uuid.NewV7()`) antes do `Save`.
- **§0.1 Contratos** — assinaturas de Repository/Service usam `string` (Go)
  para `tenantID`, `userID`, `actorID`, `targetID`. `Save` recebe a entity
  com `id` já populado pelo `service`; **não** usar `INSERT ... RETURNING id`
  (não há nada a devolver — o app já tem o id).
- **§4 Operações** — payloads JSON e exemplos mostram UUID
  (`"tenantId": "8b9c1d4e-..."`, não `"tenantId": 1`). Se possível, usar
  exemplos com timestamp prefix de UUIDv7 (`"01890e6b-..."`) para reforçar
  visualmente a versão.
- **§6 Validações** — path params validados com `validate:"uuid"` (qualquer
  versão), **não** `validate:"uuid4"`. Motivo: lock em v4 quebra ao receber
  v7 do front (que recebeu de uma `Save` anterior). A versão é decisão do
  produtor (sempre v7); o consumidor valida apenas o formato.

### Implementação (`gofi-eng`)

- Path params: validar formato UUID antes de chamar service (Go: `uuid.Parse`
  do `github.com/google/uuid`). Devolver `400 Invalid ID` quando malformado —
  evita gastar query no banco.
- **Geração do `id` no `service`, antes do `repo.Save(...)`, sempre UUIDv7** —
  Go: `id, err := uuid.NewV7()` (`github.com/google/uuid` ≥ v1.6.0); em erro,
  `service` retorna `Err{Ctx}Persist.Wrap(err)` (falha de `NewV7` é falha de
  geração de aleatórios — degrada o request). **Nunca** `uuid.NewString()` /
  `uuid.New()` para PK nova (esses retornam v4 — perdem ordenação temporal).
  Repository recebe `id` já populado e faz `INSERT ... VALUES ($1, ...)`
  incluindo a coluna `id` na lista; **sem `RETURNING id`** (o `service` já
  tem o valor). Quando o app gera o id, o `Save` pode voltar a `error` puro
  (sem `*Entity` de retorno). Excede regra geral de "Insert simples" — ver
  `.claude/sdk/<lang>/knowledge/repository-insert-simple.md`.
- DTO/Entity: validators `validate:"uuid"` (qualquer versão) — **não** lockar
  em `uuid4` nem `uuid7`. Lock em versão quebra quando o produtor evolui (v4
  → v7) e dispensa entrada legada.
- Migration: **não** colocar `CREATE EXTENSION IF NOT EXISTS "pgcrypto"`.
  Coluna sai como `id UUID PRIMARY KEY` sem default. Se a migration tinha
  esse `CREATE EXTENSION` herdado de spec antiga, removê-lo é parte da
  correção.
- JWT claims (`tenant_id`, `user_id`): em `gofi/iam` já são `string`. Validar
  formato UUID em `*FromClaims` por defesa em profundidade.
- SQL: `lib/pq` aceita `string` UUID em ambas as direções (Scan e Args).
- Não use `int64` em lugar nenhum para esses IDs — quebra ao receber UUIDs.

### UI (`gofi-ui`)

- Tipos TypeScript: `tenantId: string`, `userId: string` — nunca `number`.
- Inputs/forms que recebem UUID validam com regex ou `zod.string().uuid()`
  antes do submit.
- Path params em routers (`/users/:id`) tratados como `string`.

### Auditoria (`gofi-qa`)

Ponto de checagem obrigatório:

- [ ] `tenant.id` e `user.id` são UUID no schema (`UUID PRIMARY KEY` sem `DEFAULT`)?
- [ ] Toda coluna `tenant_id`/`*_user_id` é UUID?
- [ ] **Nenhuma migration declara `CREATE EXTENSION IF NOT EXISTS "pgcrypto"` para gerar UUID** — geração é responsabilidade da aplicação. Schema com `DEFAULT gen_random_uuid()`/`NEWID()`/`SYS_GUID()` é divergência (MAJOR).
- [ ] **Service gera o `id` como UUIDv7** (Go: `uuid.NewV7()`) antes de chamar `repo.Save(...)`. Uso de `uuid.NewString()` / `uuid.New()` (que retornam v4) para PK nova é divergência (MAJOR — perde ordenação temporal e fragmenta índice). Repo NÃO usa `INSERT ... RETURNING id`.
- [ ] DTO/Entity validators usam `validate:"uuid"` (qualquer versão) — **não** `validate:"uuid4"` nem `validate:"uuid7"` (lock em versão quebra evolução do produtor).
- [ ] Assinaturas Go/linguagem-alvo usam `string`/UUID nativo (não `int64`)?
- [ ] Exemplos de payload nas specs mostram UUIDs (não inteiros)?
- [ ] Path params validam formato UUID antes do service?

## Quando NÃO seguir

A regra é **default**, não dogma. Saia dela apenas com justificativa explícita
em ADR da spec, e somente se:

- A entidade é interna (não aparece em path/URL/JWT) e o desempenho de
  índice/join é crítico (escala 10⁹+).
- Há requisito legal/regulatório que exige ID humano-legível e sequencial
  (raro).

Em ambos os casos, registre a divergência no Histórico da spec com motivo.

## Histórico

- 2026-05-09 (tarde) — versão obrigatória passa a ser **UUIDv7** (RFC 9562)
  para PKs novas (driver: feedback do usuário sobre boas práticas de
  indexação/ordenação). Go: `uuid.NewV7()` do `github.com/google/uuid`
  v1.6.0+. Validators de DTO trocam `uuid4` por `uuid` (qualquer versão)
  para não lockar versão na borda. Snippet de geração nas skills
  `gofi-spec`/`gofi-eng`/`gofi-qa` atualizado.
- 2026-05-09 — geração do UUID passa a ser responsabilidade da aplicação
  (driver: feedback do usuário). Coluna SQL fica `UUID PRIMARY KEY` sem
  `DEFAULT`; migration não declara `CREATE EXTENSION pgcrypto`; service
  gera o id (Go: `uuid.NewString()`) antes do `Save`; repo NÃO usa
  `RETURNING id`. Aplica-se a `tenant`, `user` e a qualquer outra entidade
  marcada UUID por anti-enumeração.
- 2026-05-04 — arquivo tornado domínio-neutro: removidos exemplos de
  entidades de negócio específicas e referências a versões/ADRs de specs
  de projetos consumidores. Knowledge cross-agent descreve **padrão
  técnico**, não estado de um produto.
- 2026-05-01 — regra incorporada ao toolchain (driver: feedback do
  usuário sobre anti-enumeração de identidades em path params, JWT e URLs
  públicas).
