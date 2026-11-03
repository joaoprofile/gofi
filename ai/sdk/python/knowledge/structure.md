---
name: structure
description: Layout físico de um backend FastAPI com gofi — um pacote por contexto em app/{contexto}/, um módulo por camada (router, service, repository, models, schemas), adapters/, migrations, testes ao lado do código, .venv do starter
keywords: [estrutura, layout, fastapi, pacote, contexto, router, service, repository, models, schemas, pydantic, adapter, migration, pytest, venv]
---

# Estrutura de Contexto — FastAPI

```
{backend}/
  pyproject.toml              — dependências, mypy (strict), ruff, pytest
  .venv/                      — o ambiente que os sensores usam (gofi init)
  migrations/                 — migrations em par (upgrade/downgrade)
  app/
    main.py                   — FastAPI(); inclui o router de cada contexto; único que lê o ambiente
    {contexto}/               ← um pacote por contexto (com __init__.py)
      router.py               — APIRouter: valida a entrada (schema), chama o service, devolve o schema de saída
      dependencies.py         — Depends do contexto (sessão, usuário, service montado)
      service.py              — regra de negócio; recebe o repository pelo construtor
      repository.py           — acesso a dados; só ele conhece o banco
      models.py               — entidades de persistência
      schemas.py              — pydantic: entrada e saída da API
      adapters/               — integração com o que está fora
      test_{módulo}.py        — o teste, ao lado do que testa
```

## Regras de posicionamento

| Arquivo | Onde fica | Onde NÃO fica |
|---------|-----------|---------------|
| Regra de negócio | `service.py` | no router, no repository |
| SQL / ORM | `repository.py` | no service, no router |
| Schema pydantic | `schemas.py` | dentro do router |
| Integração externa | `adapters/` | no service direto |
| Teste | `test_*.py` no pacote do contexto | numa pasta `tests/` separada |

O gofi lê as camadas por esse layout (o pack `python`): um módulo fora dele
cai na camada `domain`.
