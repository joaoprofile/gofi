---
name: layers
description: Responsabilidade de cada camada de um contexto FastAPI (router, service, repository, adapter), o fluxo de request e de erro, e as dependências proibidas
keywords: [camadas, router, service, repository, adapter, HTTPException, Depends, fluxo, injeção, dependência]
---

# Camadas de um Contexto — FastAPI

| Camada | Responsabilidade | Não pode |
|--------|------------------|----------|
| **Router** | Receber a request, validar pelo schema, chamar o service, devolver o schema de saída | regra de negócio; chamar o repository; SQL |
| **Service** | Regra de negócio; opera pelo repository; levanta o erro de domínio | importar `fastapi`; SQL |
| **Repository** | Ler e gravar; devolve a entidade ou `None` | regra de negócio; conhecer o schema da API |
| **Adapter** | Ponte com SDK ou serviço externo | regra de negócio do domínio |

O grafo recusa router → repository, repository → service/router, service →
router, models/schemas → qualquer camada, e o service ou o repository
importando `fastapi`.

## Erros entre camadas

- **Repository** devolve `None` para não encontrado.
- **Service** levanta erro de domínio (`class {Algo}Error(Exception)`), nunca
  `HTTPException`.
- **Router** (ou um exception handler do contexto) traduz: não encontrado 404,
  conflito 409, validação 422/400 — a tradução mora num lugar só.

## Testabilidade

- O service recebe o repository pelo construtor; no teste, uma classe falsa
  com os métodos usados — sem biblioteca de mock além de `unittest.mock` quando
  preciso.
- Router testado com `TestClient` e o service trocado por `app.dependency_overrides`.
