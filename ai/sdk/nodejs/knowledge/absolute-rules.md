---
name: absolute-rules
description: Regras que nunca se quebram num backend NestJS com gofi — tipagem estrita, validação na borda, dinheiro em inteiro, sem any, segredos fora do código, rastreio gofi:implements e gofi:tests
keywords: [regras, absolutas, typescript, strict, any, validação, dinheiro, centavos, segredo, env, gofi:implements, gofi:tests]
---

# Regras absolutas — NestJS

1. **TypeScript estrito.** Sem `any` explícito, sem `// @ts-ignore`; tipo que
   falta é declarado.
2. **Toda entrada é validada na borda.** DTO com `class-validator`; o
   `ValidationPipe` global (`whitelist: true`) descarta campo desconhecido.
3. **Dinheiro é inteiro em centavos** (`number` inteiro, ou `bigint` quando
   passa de 2^53). Nunca ponto flutuante para valor monetário.
4. **Segredo e configuração vêm do ambiente**, lidos num lugar só; nenhum
   valor sensível no código nem no teste.
5. **Rastreio.** O método que realiza um requisito leva `// gofi:implements
   REQ-NN`; o `it` que prova um cenário, `// gofi:tests REQ-NN.n` logo acima.
   Sem eles o rastreio do gofi não fecha e a mudança não é aceita.
6. **Teste pulado não existe.** `it.skip`, `xit`, `describe.skip` reprovam o
   sensor de testes.
