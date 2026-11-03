---
name: structure
description: Layout físico de um backend NestJS com gofi — um módulo Nest por contexto em src/{contexto}/, arquivo por camada (controller, service, repository), entities/ e dto/, adapters/, migrations, specs ao lado do código
keywords: [estrutura, layout, nestjs, módulo, contexto, controller, service, repository, entity, dto, adapter, migration, spec, jest]
---

# Estrutura de Contexto — NestJS

```
{backend}/
  package.json, tsconfig.json, tsconfig.build.json, eslint.config.mjs
  src/
    main.ts                     — NestFactory + ValidationPipe global; único que lê PORT
    app.module.ts               — importa o módulo de cada contexto
    migrations/                 — migrations em par (up/down), uma por mudança de schema
    {contexto}/                 ← um módulo Nest por contexto
      {contexto}.module.ts      — providers e controllers do contexto; exporta só o que outro contexto usa
      {contexto}.controller.ts  — rotas: valida a entrada (DTO), chama o service, traduz a resposta
      {contexto}.service.ts     — regra de negócio; recebe o repository por injeção
      {contexto}.repository.ts  — acesso a dados; só ele conhece o banco
      entities/{entidade}.entity.ts
      dto/{operação}.dto.ts     — entrada e saída da API, com class-validator
      adapters/{sdk}.adapter.ts — integração com o que está fora (fila, IAM, HTTP externo)
      {arquivo}.spec.ts         — o teste, ao lado do que testa
```

## Regras de posicionamento

| Arquivo | Onde fica | Onde NÃO fica |
|---------|-----------|---------------|
| Regra de negócio | `{contexto}.service.ts` | no controller, no repository |
| SQL / ORM | `{contexto}.repository.ts` | no service, no controller |
| DTO com `class-validator` | `dto/` | dentro do controller |
| Integração externa | `adapters/` | no service direto |
| Teste | `*.spec.ts` ao lado do arquivo | numa pasta `test/` separada (exceto e2e) |

O gofi lê as camadas por esse layout (o pack `nodejs`): um arquivo fora dele
cai na camada `domain`, e as regras de arquitetura do grafo não o enxergam.
