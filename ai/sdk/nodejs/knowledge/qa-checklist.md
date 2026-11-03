---
name: qa-checklist
description: Checklist de auditoria de um backend NestJS — conformidade com a spec, camadas, validação, erros, persistência, segurança e testes, com severidade
keywords: [qa, checklist, auditoria, nestjs, conformidade, camadas, validação, erros, segurança, testes, severidade]
---

# Checklist de QA — NestJS

## 1. Conformidade com a Spec
- [ ] Cada requisito ADICIONADO/MODIFICADO tem o método com `gofi:implements REQ-NN`.
- [ ] Cada cenário tem o `it` com `gofi:tests REQ-NN.n`, e a asserção prova o ENTÃO.

## 2. Camadas
- [ ] Controller sem regra de negócio; service sem `Request`/`Response`; SQL só no repository.
- [ ] Um contexto usa outro pelo que o módulo dele exporta, nunca importando o repository alheio.

## 3. Validação
- [ ] Todo `@Body()`, `@Query()` e `@Param()` tipado por DTO com `class-validator`.
- [ ] Número vindo de query string convertido e validado (`@Type(() => Number)`, `@IsInt()`).

## 4. Erros
- [ ] Service lança erro de domínio; a tradução para HTTP mora num lugar só.
- [ ] Nenhum `catch` que engole o erro.

## 5. Persistência
- [ ] Escrita que mexe em mais de uma tabela numa transação.
- [ ] Migration em par up/down; a down desfaz a up.

## 6. Segurança
- [ ] Rota que não é pública tem guard; autorização checada no service quando depende do dado.
- [ ] Nenhuma query montada por concatenação de texto do usuário.

## 7. Testes
- [ ] Service testado com o repository substituído; controller com `Test.createTestingModule`.
- [ ] Sem `it.skip`; sem asserção vazia (`expect(x).toBeDefined()` sozinho não prova cenário).

## Severidade
`blocker`: quebra requisito, segurança ou dado. `major`: viola camada, validação
ou erro do checklist. `minor`: legibilidade, nome. `suggestion`: alternativa.
