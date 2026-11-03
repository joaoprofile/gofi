---
name: layers
description: Responsabilidade de cada camada de um contexto NestJS (controller, service, repository, adapter), o fluxo de request e de erro, e as dependências proibidas que o grafo recusa
keywords: [camadas, controller, service, repository, adapter, HttpException, fluxo, injeção, dependência, mocks]
---

# Camadas de um Contexto — NestJS

| Camada | Responsabilidade | Não pode |
|--------|------------------|----------|
| **Controller** | Receber a request, validar pelo DTO, chamar o service, devolver o DTO de saída | regra de negócio; chamar o repository; conhecer o banco |
| **Service** | Regra de negócio; opera pelo repository; lança o erro de domínio | conhecer `Request`/`Response` do Express; SQL |
| **Repository** | Ler e gravar; devolve entidade ou `null` | regra de negócio; conhecer o DTO da API |
| **Adapter** | Ponte com SDK ou serviço externo | regra de negócio do domínio |

O grafo recusa, sem gastar token: controller → repository, repository →
service/controller, service → controller, entidade/DTO → qualquer camada, e o
service ou o repository importando `express`.

## Fluxo

```
Controller.metodo(@Body() dto)      ← ValidationPipe já validou o DTO
  → service.operacao(dto)
      → repository.find/save(...)   ← entidade ou null
      ← lança {Contexto}NotFoundError / ...ConflictError
  ← DTO de saída
```

## Erros entre camadas

- **Repository** devolve `null` para não encontrado; erro de infraestrutura sobe.
- **Service** lança erro de domínio (`class {Algo}Error extends Error`), nunca
  `HttpException`.
- **Controller** (ou um `ExceptionFilter` do contexto) traduz: não encontrado
  404, conflito 409, validação 400 — a tradução mora num lugar só.

## Testabilidade

- O service recebe o repository pelo construtor (injeção do Nest); no teste,
  um objeto com os métodos usados — sem framework de mock além do `jest.fn()`.
- Controller testado com `Test.createTestingModule` e o service substituído.
