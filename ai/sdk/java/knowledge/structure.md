---
name: structure
description: Layout físico de um backend Quarkus com gofi — Maven, um pacote por contexto abaixo do módulo (backend.module), classe por camada (Resource, Service, Repository), entity/ e dto/, adapter/, migrations Flyway, testes espelhando o pacote em src/test/java
keywords: [estrutura, layout, quarkus, maven, pacote, contexto, resource, service, repository, entity, dto, adapter, flyway, junit]
---

# Estrutura de Contexto — Quarkus

```
{backend}/
  pom.xml
  src/main/java/{módulo}/              ← backend.module do .gofi.yaml, como caminho
    {contexto}/                        ← um pacote por contexto
      {Contexto}Resource.java          — JAX-RS: valida a entrada (DTO + Bean Validation), chama o service, devolve o DTO de saída
      {Contexto}Service.java           — @ApplicationScoped; regra de negócio; recebe o repository por injeção
      {Contexto}Repository.java        — acesso a dados; só ele conhece o banco
      {Situação}ExceptionMapper.java   — traduz o erro de domínio em resposta HTTP
      entity/{Entidade}.java
      dto/{Operação}Request.java, {Entidade}Response.java (records)
      adapter/{Sdk}Client.java         — integração com o que está fora
  src/main/resources/
    application.properties             — o único lugar que lê o ambiente
    db/migration/V{n}__{descrição}.sql — Flyway
  src/test/java/{módulo}/{contexto}/
    {Classe}Test.java                  — o teste, no mesmo pacote da classe
```

Nenhuma classe mora direto no pacote do módulo: o gofi lê o contexto pelo
pacote logo abaixo dele (o pack `java`), e uma classe ali fica sem contexto.
