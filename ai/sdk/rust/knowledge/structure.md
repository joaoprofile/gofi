---
name: structure
description: Layout físico de um backend Axum + sqlx com gofi — lib + bin, um módulo por contexto em src/{contexto}/ declarado no lib.rs, arquivo por camada (handler, service, repository, model, dto), adapters, migrations sqlx, testes em arquivo próprio {módulo}_tests.rs
keywords: [estrutura, layout, rust, axum, sqlx, módulo, contexto, handler, service, repository, model, dto, adapter, migration, testes]
---

# Estrutura de Contexto — Axum + sqlx

```
{backend}/
  Cargo.toml
  migrations/                     — sqlx: {timestamp}_{descrição}.up.sql e .down.sql
  src/
    main.rs                       — monta o Router, lê PORT; único que lê o ambiente
    lib.rs                        — pub mod {contexto}; — um por contexto
    {contexto}/                   ← um módulo por contexto
      mod.rs                      — pub mod de cada camada; #[cfg(test)] mod {camada}_tests;
      handler.rs                  — extractors (Json, Query, Path), chama o service, devolve o DTO; IntoResponse do erro
      service.rs                  — regra de negócio; recebe o repository (trait) por construtor
      repository.rs               — trait + implementação com sqlx; só ele conhece o banco
      model.rs                    — entidades (#[derive(sqlx::FromRow)])
      dto.rs                      — entrada e saída da API (serde)
      adapters/                   — integração com o que está fora
      {camada}_tests.rs           — os testes da camada, num arquivo próprio
```

**Teste em arquivo próprio**, não `mod tests` dentro do arquivo do código: quem
escreve o código nunca escreve o teste que o julga, e o gofi distingue um do
outro pelo nome do arquivo (o pack `rust`).
