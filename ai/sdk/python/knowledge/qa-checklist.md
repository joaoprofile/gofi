---
name: qa-checklist
description: Checklist de auditoria de um backend FastAPI — conformidade com a spec, camadas, validação, erros, persistência, segurança e testes, com severidade
keywords: [qa, checklist, auditoria, fastapi, conformidade, camadas, validação, erros, segurança, testes, severidade]
---

# Checklist de QA — FastAPI

## 1. Conformidade com a Spec
- [ ] Cada requisito ADICIONADO/MODIFICADO tem a função com `gofi:implements REQ-NN`.
- [ ] Cada cenário tem o teste com `gofi:tests REQ-NN.n`, e o `assert` prova o ENTÃO.

## 2. Camadas
- [ ] Router sem regra de negócio; service sem `fastapi`; SQL só no repository.

## 3. Validação
- [ ] Todo corpo, query e path tipado por schema pydantic ou anotação com restrição (`Query(ge=0)`).

## 4. Erros
- [ ] Service levanta erro de domínio; a tradução para HTTP mora num lugar só.
- [ ] Nenhum `except Exception: pass`.

## 5. Persistência
- [ ] Escrita em mais de uma tabela numa transação; migration com downgrade que desfaz o upgrade.

## 6. Segurança
- [ ] Rota que não é pública tem a dependência de autenticação; nenhuma query montada por f-string com entrada do usuário.

## 7. Testes
- [ ] Service testado com o repository falso; router com `TestClient`.
- [ ] Sem `skip`; asserção que prova o cenário, não só que "não deu erro".

## Severidade
`blocker`: quebra requisito, segurança ou dado. `major`: viola camada, validação
ou erro do checklist. `minor`: legibilidade, nome. `suggestion`: alternativa.
