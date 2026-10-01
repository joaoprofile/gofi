---
name: migrations
description: Migrations com sqln/migrate — DATABASE_MIGRATION, pasta .migrations, embed, drivers suportados
sdk: v0.8.2
keywords: [sqln, migrate, migrations, DATABASE_MIGRATION, golang-migrate, embed, schema]
---

# Migrations — `sqln/migrate`

API: `.claude/sdk/go/api/sqln-migrate.md` e `connection.WithMigrations` em
`sqln-connection.md`. Exemplo executável com `.migrations/` + seed:
`examples/sqln/search` e `examples/sqln/filter-api`.

## Caminho padrão — o componente aplica no boot

- `DATABASE_MIGRATION=true` + `database.New()` → antes de abrir o pool, aplica
  (`up`) tudo que falta em **`.migrations`**, caminho **relativo ao diretório
  de trabalho do processo**. Falha de migration = `Build` falha (o serviço não
  sobe com schema pela metade).
- Motor: golang-migrate; os arquivos seguem a convenção dele:
  `000001_create_{tabela}.up.sql` + `000001_create_{tabela}.down.sql`,
  numeração sequencial, um par por mudança.
- A migration roda num pool dedicado que é fechado depois (`RunAndClose`) —
  não prende conexão do pool da aplicação.
- Localização da pasta no repositório: `structure.md`. Em container, o
  `WORKDIR` precisa enxergar `.migrations` (ou use embed, abaixo).

## Drivers

O driver de migration é registrado pelo mesmo import em branco do driver SQL.
Em v0.8.2 **só `sqln/driver/postgres` registra driver de migration**; com
`mysql`/`sqlserver`/`oracle`, `DATABASE_MIGRATION=true` falha com
`migration driver not registered` — aplique o schema por fora ou registre um
`migrate.Driver` próprio (`migrate.RegisterDriver`).

## Migrations embutidas no binário

`migrate.Config{FS: <embed.FS>, Path: "<dir>"}`: se `Path` existe no `FS`,
lê do embed; senão cai para o disco (com warning). Para usar embed é preciso
abrir a conexão à mão (o componente só lê do disco):

```go
//go:embed .migrations
var migrations embed.FS // no pacote que fica ao lado da pasta

conn, err := connection.NewConnection(cfg,
    connection.WithMigrations(migrate.Config{FS: migrations, Path: ".migrations"}))
// ... connection.SetGlobal(conn) + database.FromDB(conn.DB()) — ver database-connection.md
```

`migrate.Run(db, driver, cfg)` existe para quem já tem um `*sql.DB`, mas
golang-migrate prende uma conexão desse pool até o fim do processo — prefira
`RunAndClose` num `*sql.DB` aberto só para isso.

## Regras de conteúdo

- Migration aplicada é imutável: correção = nova migration.
- DDL de produção em tabela grande: `CREATE INDEX CONCURRENTLY` (fora de
  transação — arquivo só com esse comando). Estratégia de índices:
  `postgres-index-strategy.md`.
- Constraints com nome explícito (`CONSTRAINT uq_{tabela}_{colunas} UNIQUE
  (...)`) — o repository traduz conflito por `Constraint`
  (`persistence-rules.md`).
- IDs gerados na aplicação não têm `DEFAULT` nem extensão de geração no
  schema (`.claude/expertise/ddd-architecture/id-types.md`).
