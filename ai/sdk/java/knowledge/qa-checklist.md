---
name: qa-checklist
description: Checklist de auditoria de um backend Quarkus — conformidade com a spec, camadas, validação, erros, persistência, segurança e testes, com severidade
keywords: [qa, checklist, auditoria, quarkus, conformidade, camadas, validação, erros, segurança, testes, severidade]
---

# Checklist de QA — Quarkus

## 1. Conformidade com a Spec
- [ ] Cada requisito ADICIONADO/MODIFICADO tem o método com `gofi:implements REQ-NN`.
- [ ] Cada cenário tem o `@Test` com `gofi:tests REQ-NN.n`, e a asserção prova o ENTÃO.

## 2. Camadas
- [ ] Resource sem regra de negócio; service sem `Response`; SQL só no repository.

## 3. Validação
- [ ] Todo corpo com `@Valid` e DTO anotado; parâmetro de query com restrição (`@Min`, `@NotNull`).

## 4. Erros
- [ ] Service lança exceção de domínio; um `ExceptionMapper` traduz; nenhum `catch` que engole.

## 5. Persistência
- [ ] Escrita em mais de uma tabela em `@Transactional`; migration Flyway nova, nunca editada depois de aplicada.

## 6. Segurança
- [ ] Rota que não é pública tem `@RolesAllowed`/`@Authenticated`; nenhuma query por concatenação.

## 7. Testes
- [ ] Regra de negócio testada sem `@QuarkusTest`; sem `@Disabled`; asserção que prova o cenário.

## Severidade
`blocker`: quebra requisito, segurança ou dado. `major`: viola camada, validação
ou erro do checklist. `minor`: legibilidade, nome. `suggestion`: alternativa.
