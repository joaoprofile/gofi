---
pack: platform-delivery
title: Plataforma e entrega
summary: princípios de IaC e delivery, e o arquivo .env do projeto
applies_when:
  intents: [provisionar, implantar, configurar ambiente, criar pipeline]
  entities: [ambiente, pipeline, infraestrutura, variável de ambiente]
  paths: ["**/*.tf", "**/.env*", "**/Dockerfile", ".github/workflows/**", "**/azure-pipelines*.yml"]
serves: [eng, ops]
---

# Plataforma e entrega

IaC, pipelines e ambiente, independentes de cloud e de ferramenta.

| Seção | Quando |
|---|---|
| `iac-principles.md` | princípios e regras de IaC e delivery |
| `env-file-management.md` | manter o `.env` da raiz ao implementar um contexto que precisa de variáveis |
