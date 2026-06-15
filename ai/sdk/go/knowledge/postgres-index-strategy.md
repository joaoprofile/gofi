# Estratégia de Índices — PostgreSQL

Aplicado quando o contexto materializa entidades em PostgreSQL via
`gofi/sqln`. Cobre: classificação de perfil de acesso por tabela, padrões
de índice por tipo de filtro e tuning de `fillfactor`/autovacuum.

A spec **declara** o perfil de cada tabela em §3 (modelo de dados) ou §4
(arquitetura). O `gofi-eng` usa pra escolher índices + storage params. O
`gofi-qa` audita.

---

## 1. Perfil de acesso por tabela

Toda tabela cabe em um destes quatro perfis:

| Perfil | Padrão de write | Padrão de read | Implicações de índice |
|---|---|---|---|
| **Cold** | poucos INSERT/UPDATE | muito SELECT (listagem rica, filtros combinados) | Tolera mais índices. Filtros frequentes → composite/partial. Substring → GIN trgm. |
| **Hot UPDATE** | UPDATE constante em colunas específicas | SELECT por PK ou JOIN | Minimizar índices em colunas voláteis (cada um quebra HOT update). `fillfactor=70-80` + autovacuum tunado. |
| **Hot DELETE+INSERT** | DELETE em lote por chave + INSERT em lote (rebuild) | SELECT por chave de agrupamento | Manter só índices da chave de DELETE/agrupamento e dos workers. Sem `fillfactor` (não há UPDATE). Autovacuum agressivo (turnover gera bloat). |
| **Append-only** | INSERT contínuo, sem UPDATE/DELETE | SELECT por PK ou time-range | Particionar por tempo (RANGE). Índices declarados no parent (propagam). Cuidado com índices de baixo benefício — write amp em volume alto. |

---

## 2. Multi-tenant: leading column é o tenant

Em SaaS multi-tenant, **toda query filtra por tenant**. Índice
single-column em coluna não-tenant atravessa todos os tenants no scan e
desperdiça I/O.

```sql
-- Anti-padrão (atravessa tenants)
CREATE INDEX idx_x_sku ON x(sku);

-- Correto (escopa por tenant)
CREATE INDEX idx_x_tenant_sku ON x(tenant_id, sku);
```

Variante: **partial index** quando um valor de coluna domina (ex: 90% dos
registros têm `status='ACTIVE'`):

```sql
CREATE INDEX idx_x_tenant_active
    ON x(tenant_id) WHERE status = 'ACTIVE';
```

Partial é menor (cabe em menos páginas) e não escreve entradas pros
demais valores. Combina com índices do filtro adicional via `BitmapAnd`.

---

## 3. Por tipo de filtro

| Tipo na spec / `/schemas` | Operadores típicos | Estratégia |
|---|---|---|
| `text` exato (código, sku) | `=`, `IN` | Btree composite `(tenant, col)` |
| `text` prefixo | `LIKE 'x%'` | Btree composite (mesma forma) |
| `text` substring | `LIKE '%x%'`, `ILIKE`, regex | **GIN com `gin_trgm_ops`** (extensão `pg_trgm`). Single column é OK — planner combina com índice do tenant via `BitmapAnd`. |
| `number` exato | `=`, `IN` | Btree composite `(tenant, col)` |
| `number` range | `BETWEEN`, `<`, `>` | Btree composite (range na 2ª coluna) |
| `boolean` | `=` | Geralmente NÃO indexar (baixa cardinalidade). Exceção: partial pelo valor minoritário. |
| `enum`/`status` poucos valores | `=`, `IN` | Composite `(tenant, status)` ou partial pelo valor dominante |
| Sort field | `ORDER BY` | Composite com a coluna de sort no fim, com `DESC` se aplicável |

Substring (`LIKE '%x%'`) **não usa** btree — sempre GIN trigram. Trigrama
em strings com menos de 3 caracteres não funciona; o planner cai pra seq
scan. Aceitável quando o filtro é livre.

---

## 4. Workers cross-cutting

Quando há jobs/agents que **filtram por uma coluna específica em todas as
tabelas** (ex: purge de churn, archive de retenção, replicação seletiva),
essa coluna precisa de índice em **toda tabela** que ela aparece, mesmo
nas hot UPDATE.

Padrão: `CREATE INDEX idx_<table>_<col> ON <table>(<col>)` — single column,
porque o filtro do worker é `WHERE <col> = $1`.

Se a coluna não existe naturalmente (ex: junction table sem coluna de
tenant), adicionar **denormalizada** + índice mantém o worker uniforme.
Documentar em ADR a regra de propagação (qual entidade pai define o valor
na escrita do agent que materializa).

Esse padrão é decisão de projeto — quem decide é a spec. Se o projeto
tem um worker desses, vive em `.claude/memory/project.md` ou em um
memory `project_*.md` específico.

---

## 5. fillfactor

`fillfactor` reserva % de espaço livre por página pra HOT updates (UPDATE
sem reindexação quando colunas indexadas não mudam).

| Perfil | fillfactor sugerido | Quando |
|---|---|---|
| Cold | 100 (default) | Sem UPDATE significativo |
| Hot UPDATE com colunas indexadas estáveis | 70-80 | UPDATE não toca colunas indexadas → HOT update funciona |
| Hot UPDATE em coluna indexada | 100 (default) | HOT update já é impossível — reservar espaço só desperdiça |
| Hot DELETE+INSERT | 100 (default) | Sem UPDATE |
| Append-only | 100 (default) | Sem UPDATE |

Aplicar com `ALTER TABLE <tabela> SET (fillfactor = N)` na migration.

---

## 6. Tuning de autovacuum

Tabelas com turnover alto (hot UPDATE / hot DELETE+INSERT) acumulam dead
tuples mais rápido que o default do Postgres limpa. Default dispara
autovacuum quando 20% da tabela é dead — pra turnover alto isso vira
bloat persistente. Padrão sugerido pra tabelas hot:

```sql
ALTER TABLE <tabela> SET (
    autovacuum_vacuum_threshold = 8000,
    autovacuum_vacuum_scale_factor = 0.01,
    autovacuum_analyze_threshold = 4000,
    autovacuum_analyze_scale_factor = 0.005,
    autovacuum_vacuum_cost_limit = 1000,
    autovacuum_vacuum_cost_delay = 10
);
```

`scale_factor=0.01` dispara com 1% de dead tuples — 20× mais agressivo
que o default.

---

## 7. Particionamento (append-only de alto volume)

Tabela append-only que cresce indefinidamente (history, eventos, audit
log) deve ser particionada por tempo desde v1 — migrar particionamento
em produção é caro.

```sql
CREATE TABLE x (...) PARTITION BY RANGE (created_at);
CREATE TABLE x_YYYY_MM PARTITION OF x
    FOR VALUES FROM ('YYYY-MM-01') TO ('YYYY-(MM+1)-01');
```

Índices declarados no parent propagam pras partições atuais e futuras.
Operação contínua de criar partições futuras é tarefa operacional (cron /
`pg_partman`) — fora do escopo da migration inicial.

---

## 8. Checklist antes de aprovar um índice

- [ ] Tabela tem perfil declarado (cold / hot UPDATE / hot DELETE+INSERT / append-only)?
- [ ] Leading column corresponde ao filtro mais comum (tenant na maioria dos casos)?
- [ ] O índice cobre uma query real declarada na spec — não hipotética?
- [ ] Custo de write aceitável pro perfil? (cada índice = O(log n) por linha escrita)
- [ ] Substring sobre `text`? GIN trgm em vez de btree single-column?
- [ ] Boolean/baixa cardinalidade? Partial em vez de índice cheio?
- [ ] Coluna de worker cross-cutting (purge, archive, replicação) presente em toda tabela onde a coluna existe?
- [ ] Hot UPDATE: número de índices em colunas voláteis foi minimizado?

---

## 9. Anti-padrões

- Btree single-column em substring (`LIKE '%x%'` não usa).
- Single-column sem tenant em multi-tenant (atravessa tenants).
- Índice em boolean/2-valores sem partial.
- Índice criado "por garantia" sem query declarada.
- Hot UPDATE com 6+ índices em colunas que mudam.
- Append-only sem partição quando volume cresce indefinidamente.
- `fillfactor` reduzido em cold table (desperdício).
- `fillfactor=70` em hot UPDATE onde toda coluna indexada também muda (HOT update já está bloqueado, espaço livre não ajuda).
- Drop+recreate de índice em produção sem `CONCURRENTLY` (lock).

---

## 10. Trigger de revisão

Revisar índices sempre que:

- Spec adiciona operação de leitura nova → o índice atual cobre o caminho?
- Tabela muda de perfil (cold passa a receber UPDATE frequente, append-only ganha UPDATE retroativo) → revisar índices voláteis + `fillfactor` + autovacuum.
- Worker cross-cutting é adicionado/removido do projeto → propagar índice em todas as tabelas afetadas (ou removê-lo).
- EXPLAIN em produção mostra seq scan onde planner deveria usar índice → investigar seletividade, recriar com forma diferente, ou ajustar query.
