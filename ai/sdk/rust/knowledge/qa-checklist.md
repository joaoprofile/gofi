---
name: qa-checklist
description: Checklist de auditoria de um backend Axum + sqlx — conformidade com a spec, camadas, validação, erros, persistência, segurança e testes, com severidade
keywords: [qa, checklist, auditoria, rust, axum, sqlx, conformidade, camadas, validação, erros, segurança, testes, severidade]
---

# Checklist de QA — Axum + sqlx

## 1. Conformidade com a Spec
- [ ] Cada requisito ADICIONADO/MODIFICADO tem a função com `gofi:implements REQ-NN`.
- [ ] Cada cenário tem o `#[test]` com `gofi:tests REQ-NN.n`, e o `assert` prova o ENTÃO.

## 2. Camadas
- [ ] Handler sem regra de negócio; service sem `axum`; SQL só no repository.

## 3. Validação
- [ ] Entrada desserializada em DTO com os tipos certos; regra de faixa checada antes do service agir.

## 4. Erros
- [ ] `enum` de erro com `thiserror`; `IntoResponse` num lugar só; sem `unwrap()` fora do main e dos testes.

## 5. Persistência
- [ ] Escrita em mais de uma tabela numa transação (`pool.begin()`); migration com down que desfaz o up.
- [ ] Query com bind (`$1`), nunca `format!` com entrada do usuário.

## 6. Segurança
- [ ] Rota que não é pública passa pelo extractor/middleware de autenticação.

## 7. Testes
- [ ] Teste em `{camada}_tests.rs`; repository falso pelo trait; sem `#[ignore]`.

## Severidade
`blocker`: quebra requisito, segurança ou dado. `major`: viola camada, validação
ou erro do checklist. `minor`: legibilidade, nome. `suggestion`: alternativa.
