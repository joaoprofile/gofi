---
name: layers
description: Responsabilidade de cada camada de um contexto Quarkus (resource, service, repository, adapter), o fluxo de request e de erro com ExceptionMapper, e as dependências proibidas
keywords: [camadas, resource, service, repository, adapter, ExceptionMapper, CDI, injeção, fluxo, dependência]
---

# Camadas de um Contexto — Quarkus

| Camada | Responsabilidade | Não pode |
|--------|------------------|----------|
| **Resource** | Receber a request, validar (`@Valid` no DTO), chamar o service, devolver o DTO | regra de negócio; chamar o repository; SQL |
| **Service** | Regra de negócio; opera pelo repository; lança a exceção de domínio | `jakarta.ws.rs.core.Response`; SQL |
| **Repository** | Ler e gravar; devolve a entidade ou `Optional.empty()` | regra de negócio; conhecer o DTO |
| **Adapter** | Ponte com SDK ou serviço externo | regra de negócio do domínio |

O grafo recusa resource → repository, repository → service/resource, service
→ resource, entity/dto → qualquer camada.

## Erros entre camadas

- **Service** lança exceção de domínio (`class {Algo}Exception extends RuntimeException`).
- **ExceptionMapper** do contexto traduz: não encontrado 404, conflito 409;
  validação o Quarkus responde 400 sozinho. A tradução mora num lugar só.

## Testabilidade

- O service recebe o repository pelo construtor (injeção por construtor); no
  teste de unidade, uma implementação falsa — sem `@QuarkusTest` para regra de
  negócio.
- Resource testado com `@QuarkusTest` e REST Assured só quando o contrato HTTP
  é o que o cenário pede.
