---
name: absolute-rules
description: Regras que nunca se quebram num backend Quarkus com gofi — injeção por construtor, validação na borda com Bean Validation, dinheiro em long de centavos, segredos fora do código, rastreio gofi:implements e gofi:tests
keywords: [regras, absolutas, java, quarkus, injeção, construtor, bean validation, dinheiro, centavos, segredo, gofi:implements, gofi:tests]
---

# Regras absolutas — Quarkus

1. **Injeção por construtor**, campos `final`; nada de `@Inject` em campo.
2. **Toda entrada é validada na borda**: DTO com Bean Validation e `@Valid`
   no resource.
3. **Dinheiro é `long` em centavos** (ou `BigDecimal` com escala fixa quando
   a regra pede). Nunca `double` para valor monetário.
4. **Segredo e configuração vêm do ambiente**, por `application.properties`
   e `@ConfigProperty`.
5. **Rastreio.** O método que realiza um requisito leva `// gofi:implements
   REQ-NN`; o `@Test` que prova um cenário, `// gofi:tests REQ-NN.n` logo acima.
6. **Teste pulado não existe.** `@Disabled` e `assumeTrue` sem motivo de
   ambiente reprovam o sensor de testes.
