---
pack: ddd-architecture
title: DDD e arquitetura em camadas
summary: agregado, value object, application vs domain service, tipos de id, clean code e análise de impacto
applies_when:
  intents: [modelar, especificar, implementar, refatorar, alterar]
  entities: [agregado, entidade, value object, contexto, serviço]
  paths: ["**/model/**", "**/service/**", "**/application/**", "**/repository/**"]
serves: [spec, eng, qa]
---

# DDD e arquitetura em camadas

Modelagem de domínio e fronteiras entre camadas, independentes de linguagem.

| Seção | Quando |
|---|---|
| `ddd-principles.md` | agregado e raiz, value objects, invariantes |
| `application-vs-domain-service.md` | quando criar `application/`, direção de dependência, erros por camada |
| `id-types.md` | tipo de identificador de entidade |
| `clean-code.md` | funções, nomes, comentários |
| `impact-analysis-on-change.md` | alterar contexto já implementado: o que verificar antes |
