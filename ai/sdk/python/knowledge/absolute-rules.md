---
name: absolute-rules
description: Regras que nunca se quebram num backend FastAPI com gofi — tipagem estrita (mypy strict), validação na borda com pydantic, dinheiro em inteiro, segredos fora do código, rastreio gofi:implements e gofi:tests
keywords: [regras, absolutas, python, mypy, tipagem, pydantic, validação, dinheiro, centavos, segredo, gofi:implements, gofi:tests]
---

# Regras absolutas — FastAPI

1. **Tipado de ponta a ponta.** Toda função anotada; o `mypy --strict` do
   starter é o build — o que não tipa não passa.
2. **Toda entrada é validada na borda** por um schema pydantic; o router nunca
   lê `request.json()` à mão.
3. **Dinheiro é `int` em centavos.** Nunca `float` para valor monetário; quando
   a entrada traz decimal, `Decimal` convertido na borda.
4. **Segredo e configuração vêm do ambiente**, lidos num lugar só.
5. **Rastreio.** A função que realiza um requisito leva `# gofi:implements
   REQ-NN`; o teste que prova um cenário, `# gofi:tests REQ-NN.n` logo acima.
6. **Teste pulado não existe.** `pytest.mark.skip`, `skipif` sem motivo de
   plataforma e `xfail` reprovam o sensor de testes.
