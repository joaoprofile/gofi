---
name: layers
description: Responsabilidade de cada camada de um contexto Axum (handler, service, repository, adapter), o fluxo de request e de erro com thiserror e IntoResponse, e as dependências proibidas
keywords: [camadas, handler, service, repository, adapter, thiserror, IntoResponse, trait, injeção, fluxo, dependência]
---

# Camadas de um Contexto — Axum + sqlx

| Camada | Responsabilidade | Não pode |
|--------|------------------|----------|
| **Handler** | Extrair e validar a entrada, chamar o service, devolver o DTO | regra de negócio; chamar o repository; SQL |
| **Service** | Regra de negócio; opera pelo trait do repository; devolve `Result<_, {Contexto}Error>` | conhecer `axum`; SQL |
| **Repository** | Ler e gravar com sqlx; devolve `Option<Entidade>` | regra de negócio; conhecer o DTO |
| **Adapter** | Ponte com SDK ou serviço externo | regra de negócio do domínio |

O grafo recusa handler → repository, repository → service/handler, service →
handler, model/dto → qualquer camada.

## Erros entre camadas

- **Service** devolve um `enum {Contexto}Error` com `thiserror`.
- **Handler** traduz por `impl IntoResponse for {Contexto}Error`: não
  encontrado 404, conflito 409, entrada inválida 400 — num lugar só.
- Nada de `unwrap()`/`expect()` fora do `main.rs` e dos testes.

## Testabilidade

- O service recebe o repository como trait (`Arc<dyn {Contexto}Repository>`
  ou genérico); no teste, uma struct falsa que implementa o trait.
